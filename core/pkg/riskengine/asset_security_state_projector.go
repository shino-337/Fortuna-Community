package riskengine

import (
	"context"
	"encoding/json"
	"time"

	"github.com/fortuna/core/pkg/models"
)

// UpsertAssetSecurityState minimal projector (P0.5).
// It computes a unified runtime snapshot used as backbone for Risk Engine + compatibility projection.
func (e *Engine) UpsertAssetSecurityState(ctx context.Context, podUID string) error {
	if e == nil || e.db == nil || podUID == "" {
		return nil
	}

	// Recompute only when missing or stale (best-effort cache).
	var prev models.AssetSecurityState
	tx := e.db.WithContext(ctx).
		Where("pod_uid = ?", podUID).
		First(&prev)
	if tx.Error == nil {
		if time.Since(prev.UpdatedAt) < 5*time.Minute {
			return nil
		}
	}

	// Identity + privilege context from pods
	var pod models.Pod
	if err := e.db.WithContext(ctx).
		Where("uid = ? AND deleted_at IS NULL", podUID).
		First(&pod).Error; err != nil {
		return nil
	}

	clusterID := pod.ClusterID
	ns := pod.Namespace

	// Runtime signals within lookback
	since := time.Now().Add(-runtimeRiskLookback())

	type row struct {
		SignalType string
		C          int64
	}
	var rows []row
	_ = e.db.WithContext(ctx).
		Model(&models.RuntimeSignal{}).
		Select("signal_type, COALESCE(SUM(count),0) as c").
		Where("pod_uid = ? AND (created_at >= ? OR last_seen_at >= ?)", podUID, since, since).
		Group("signal_type").
		Scan(&rows).Error

	var (
		signalTotal   int64
		hasSuspicious bool
		hasNetAnom    bool
		hasEscape     bool
		runtimeByType map[string]int64
	)
	runtimeByType = map[string]int64{}

	for _, r := range rows {
		signalTotal += r.C
		runtimeByType[r.SignalType] = r.C
		switch r.SignalType {
		case "SUSPICIOUS_EXEC_FROM_SNAPSHOT":
			hasSuspicious = true
		case "NETWORK_QUEUE_ANOMALY":
			hasNetAnom = true
		case "PROC_ROOT_PIVOT", "FS_ESCAPE_ATTEMPT", "NAMESPACE_ESCAPE", "CAPABILITY_MISUSE":
			hasEscape = true
		}
	}

	// Best-effort: last runtime activity timestamp for freshness/decay logic.
	type maxRow struct {
		MaxTime *time.Time `gorm:"column:max_time"`
	}
	var maxSeen maxRow
	_ = e.db.WithContext(ctx).
		Model(&models.RuntimeSignal{}).
		Select("MAX(COALESCE(last_seen_at, created_at)) as max_time").
		Where("pod_uid = ? AND (created_at >= ? OR last_seen_at >= ?)", podUID, since, since).
		Scan(&maxSeen).Error
	var lastActivity *time.Time
	if maxSeen.MaxTime != nil && !maxSeen.MaxTime.IsZero() {
		t := maxSeen.MaxTime.UTC()
		lastActivity = &t
	}

	// Effective capabilities: minimal interpretation from legacy states.
	// (P0 compatibility: only used later as backbone candidate.)
	type capRow struct {
		CapabilityID string
	}
	var caps []capRow
	_ = e.db.WithContext(ctx).
		Table("pod_capabilities").
		Select("capability_id").
		Where("pod_uid = ? AND state IN (?)", podUID, []string{"confirmed", "exploited", "chained"}).
		Scan(&caps).Error

	effective := make([]string, 0, len(caps))
	for _, c := range caps {
		if c.CapabilityID != "" {
			effective = append(effective, c.CapabilityID)
		}
	}

	rtJSON, _ := json.Marshal(runtimeByType)
	effJSON, _ := json.Marshal(effective)

	// cluster-admin binding context (reuses existing helper)
	serviceAccountBound := false
	if clusterID != "" && ns != "" && pod.ServiceAccount != "" {
		serviceAccountBound = clusterAdminBindingForPod(ctx, e.db, clusterID, ns, pod.ServiceAccount)
	}

	now := time.Now()
	newState := models.AssetSecurityState{
		AssetType:                         "pod",
		PodUID:                            podUID,
		Namespace:                         pod.Namespace,
		ClusterID:                         clusterID,
		HostNetwork:                       pod.HostNetwork,
		HostPID:                           pod.HostPID,
		HostIPC:                           pod.HostIPC,
		ServiceAccountBoundToClusterAdmin: serviceAccountBound,

		SignalTotal24h:         signalTotal,
		HasSuspiciousExec:      hasSuspicious,
		HasNetworkQueueAnomaly: hasNetAnom,
		HasEscapeRelated:       hasEscape,
		LastRuntimeActivityAt:  lastActivity,

		RuntimeSignalsByType:  string(rtJSON),
		EffectiveCapabilities: string(effJSON),

		UpdatedAt: now,
		CreatedAt: now,
	}

	if err := models.ValidateAssetSecurityStateJSON(&newState); err != nil {
		return err
	}

	// Upsert by pod_uid (unique)
	if prev.ID != 0 {
		newState.ID = prev.ID
		return e.db.WithContext(ctx).Model(&models.AssetSecurityState{}).
			Where("pod_uid = ?", podUID).
			Updates(map[string]interface{}{
				"asset_type":                             "pod",
				"namespace":                              newState.Namespace,
				"cluster_id":                             newState.ClusterID,
				"host_network":                           newState.HostNetwork,
				"host_pid":                               newState.HostPID,
				"host_ipc":                               newState.HostIPC,
				"service_account_bound_to_cluster_admin": newState.ServiceAccountBoundToClusterAdmin,
				"signal_total_24h":                       newState.SignalTotal24h,
				"has_suspicious_exec":                    newState.HasSuspiciousExec,
				"has_network_queue_anomaly":              newState.HasNetworkQueueAnomaly,
				"has_escape_related":                     newState.HasEscapeRelated,
				"last_runtime_activity_at":               newState.LastRuntimeActivityAt,
				"runtime_signals_by_type":                newState.RuntimeSignalsByType,
				"effective_capabilities":                 newState.EffectiveCapabilities,
				"updated_at":                             newState.UpdatedAt,
			}).Error
	}

	return e.db.WithContext(ctx).Create(&newState).Error
}
