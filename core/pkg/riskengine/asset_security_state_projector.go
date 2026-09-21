package riskengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
)

// UpsertAssetSecurityState is a fail-closed compatibility wrapper for legacy UID-only callers.
// It proceeds only when the Pod UID resolves to exactly one active cluster owner.
func (e *Engine) UpsertAssetSecurityState(ctx context.Context, podUID string) error {
	if e == nil || e.db == nil || podUID == "" {
		return nil
	}
	var clusterIDs []string
	if err := e.db.WithContext(ctx).Model(&models.Pod{}).
		Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", id.ClusterID, podUID).
		Distinct().Order("cluster_id").Pluck("cluster_id", &clusterIDs).Error; err != nil {
		return fmt.Errorf("resolve pod security-state ownership: %w", err)
	}
	if len(clusterIDs) != 1 {
		return fmt.Errorf("cluster-qualified pod identity required for security state: uid=%s owners=%d", podUID, len(clusterIDs))
	}
	id, err := resourceidentity.New(clusterIDs[0], podUID)
	if err != nil {
		return err
	}
	return e.UpsertAssetSecurityStateForIdentity(ctx, id)
}

// UpsertAssetSecurityStateForIdentity projects one canonical {cluster_id,pod_uid} asset.
func (e *Engine) UpsertAssetSecurityStateForIdentity(ctx context.Context, id resourceidentity.Identity) error {
	if e == nil || e.db == nil {
		return nil
	}
	if err := id.Validate(); err != nil {
		return err
	}
	podUID := id.ResourceUID

	// Recompute only when missing or stale; storage failures are not cache misses.
	var prev models.AssetSecurityState
	tx := e.db.WithContext(ctx).
		Where("cluster_id = ? AND pod_uid = ?", id.ClusterID, podUID).
		First(&prev)
	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read previous security state: %w", tx.Error)
	}
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
		return fmt.Errorf("read pod: %w", err)
	}

	clusterID := id.ClusterID
	ns := pod.Namespace

	// Runtime signals within lookback
	since := time.Now().Add(-runtimeRiskLookback())

	type row struct {
		SignalType string
		C          int64
	}
	var rows []row
	if err := e.db.WithContext(ctx).
		Model(&models.RuntimeSignal{}).
		Select("signal_type, COALESCE(SUM(count),0) as c").
		Where("cluster_id = ? AND pod_uid = ? AND (created_at >= ? OR last_seen_at >= ?)", id.ClusterID, podUID, since, since).
		Group("signal_type").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("read runtime signals: %w", err)
	}

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
		MaxTime models.NullTime `gorm:"column:max_time"`
	}
	var maxSeen maxRow
	if err := e.db.WithContext(ctx).
		Model(&models.RuntimeSignal{}).
		Select("MAX(COALESCE(last_seen_at, created_at)) as max_time").
		Where("cluster_id = ? AND pod_uid = ? AND (created_at >= ? OR last_seen_at >= ?)", id.ClusterID, podUID, since, since).
		Scan(&maxSeen).Error; err != nil {
		return err
	}
	var lastActivity *time.Time
	if maxSeen.MaxTime.Time != nil && !maxSeen.MaxTime.Time.IsZero() {
		t := maxSeen.MaxTime.Time.UTC()
		lastActivity = &t
	}

	// Effective capabilities: minimal interpretation from legacy states.
	// (P0 compatibility: only used later as backbone candidate.)
	type capRow struct {
		CapabilityID string
	}
	var caps []capRow
	if err := e.db.WithContext(ctx).
		Table("pod_capabilities").
		Select("capability_id").
		Where("cluster_id = ? AND pod_uid = ? AND state IN (?)", id.ClusterID, podUID, []string{"confirmed", "exploited", "chained"}).
		Scan(&caps).Error; err != nil {
		return fmt.Errorf("read pod capabilities: %w", err)
	}

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
		var err error
		serviceAccountBound, err = clusterAdminBindingForPod(ctx, e.db, clusterID, ns, pod.ServiceAccount)
		if err != nil {
			return err
		}
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
