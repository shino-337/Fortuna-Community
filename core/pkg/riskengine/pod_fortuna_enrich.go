package riskengine

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
)

const envRuntimeRiskLookbackHours = "FORTUNA_RUNTIME_RISK_LOOKBACK_HOURS"

func runtimeRiskLookback() time.Duration {
	h := int64(24)
	if v := strings.TrimSpace(os.Getenv(envRuntimeRiskLookbackHours)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 && n <= 168 {
			h = n
		}
	}
	return time.Duration(h) * time.Hour
}

// enrichPodFortunaContext merges Fortuna DB/runtime aggregates into enriched object data for CEL (object.fortuna.*).
func (e *Engine) enrichPodFortunaContext(ctx context.Context, enriched map[string]interface{}) {
	if e == nil || e.db == nil || enriched == nil {
		return
	}
	uid, _ := enriched["uid"].(string)
	if strings.TrimSpace(uid) == "" {
		return
	}

	fortuna := map[string]interface{}{
		"signal_total_24h":                       int64(0),
		"has_suspicious_exec":                    false,
		"has_network_queue_anomaly":              false,
		"has_escape_related":                     false,
		"host_network":                           false,
		"host_pid":                               false,
		"host_ipc":                               false,
		"service_account_bound_to_cluster_admin": false,
	}

	// P0.5: derive from unified asset_security_state (backbone for future risk engine v2).
	// Keep compatibility by projecting into object.fortuna.* booleans.
	// Rollout-safety: if asset_security_state table isn't available yet (unit tests / partial migrations),
	// fallback to legacy behavior.
	_ = e.UpsertAssetSecurityState(ctx, uid)

	var state models.AssetSecurityState
	if err := e.db.WithContext(ctx).
		Where("pod_uid = ?", uid).
		First(&state).Error; err == nil {
		fortuna["signal_total_24h"] = state.SignalTotal24h
		fortuna["has_suspicious_exec"] = state.HasSuspiciousExec
		fortuna["has_network_queue_anomaly"] = state.HasNetworkQueueAnomaly
		fortuna["has_escape_related"] = state.HasEscapeRelated
		fortuna["host_network"] = state.HostNetwork
		fortuna["host_pid"] = state.HostPID
		fortuna["host_ipc"] = state.HostIPC
		fortuna["service_account_bound_to_cluster_admin"] = state.ServiceAccountBoundToClusterAdmin
		enriched["fortuna"] = fortuna

		// Risk Engine v2 input: expose structured security state to CEL.
		// Kept separate from fortuna.* to allow v2 rules to evolve without breaking legacy rules.
		var runtimeSignalsByType map[string]int64
		if strings.TrimSpace(state.RuntimeSignalsByType) != "" {
			_ = json.Unmarshal([]byte(state.RuntimeSignalsByType), &runtimeSignalsByType)
		}
		if runtimeSignalsByType == nil {
			runtimeSignalsByType = map[string]int64{}
		}

		var effectiveCaps []string
		if strings.TrimSpace(state.EffectiveCapabilities) != "" {
			_ = json.Unmarshal([]byte(state.EffectiveCapabilities), &effectiveCaps)
		}
		if effectiveCaps == nil {
			effectiveCaps = []string{}
		}

		enriched["securityState"] = map[string]interface{}{
			// runtime booleans (aligned with legacy object.fortuna.* naming)
			"signal_total_24h":                       state.SignalTotal24h,
			"has_suspicious_exec":                    state.HasSuspiciousExec,
			"has_network_queue_anomaly":              state.HasNetworkQueueAnomaly,
			"has_escape_related":                     state.HasEscapeRelated,
			"host_network":                           state.HostNetwork,
			"host_pid":                               state.HostPID,
			"host_ipc":                               state.HostIPC,
			"service_account_bound_to_cluster_admin": state.ServiceAccountBoundToClusterAdmin,
			// layer-4 structures for v2 rules
			"runtime_signals_by_type":  runtimeSignalsByType,
			"effective_capabilities":   effectiveCaps,
			"last_runtime_activity_at": state.LastRuntimeActivityAt,
		}
		return
	}

	// Legacy fallback (matches existing unit tests & current object.fortuna contract).
	var pod models.Pod
	if err := e.db.WithContext(ctx).Where("uid = ? AND deleted_at IS NULL", uid).First(&pod).Error; err == nil {
		fortuna["host_network"] = pod.HostNetwork
		fortuna["host_pid"] = pod.HostPID
		fortuna["host_ipc"] = pod.HostIPC
		fortuna["service_account_bound_to_cluster_admin"] = clusterAdminBindingForPod(ctx, e.db, pod.ClusterID, pod.Namespace, pod.ServiceAccount)
	}

	since := time.Now().Add(-runtimeRiskLookback())
	type row struct {
		SignalType string
		C          int64
	}
	var stats []row
	_ = e.db.WithContext(ctx).Model(&models.RuntimeSignal{}).
		Select("signal_type, count(*) as c").
		Where("pod_uid = ? AND created_at >= ?", uid, since).
		Group("signal_type").
		Scan(&stats).Error

	var total int64
	for _, s := range stats {
		total += s.C
		switch s.SignalType {
		case "SUSPICIOUS_EXEC_FROM_SNAPSHOT":
			fortuna["has_suspicious_exec"] = true
		case "NETWORK_QUEUE_ANOMALY":
			fortuna["has_network_queue_anomaly"] = true
		case "PROC_ROOT_PIVOT", "FS_ESCAPE_ATTEMPT", "NAMESPACE_ESCAPE", "CAPABILITY_MISUSE":
			fortuna["has_escape_related"] = true
		}
	}
	fortuna["signal_total_24h"] = total
	enriched["fortuna"] = fortuna
}

// ruleAppliesToPod selects YAML/policy rules safe to evaluate against Pod-shaped objects.
// RBAC rules that assume Role/Binding structure are skipped unless tagged "pod" (e.g. cluster-admin-pod correlation).
func ruleAppliesToPod(rule Rule) bool {
	switch rule.Category {
	case CategoryPodSecurity, CategoryRuntime, CategoryNetworkPolicy, CategoryCompliance, CategorySecrets:
		return true
	case CategoryRBAC:
		for _, t := range rule.Tags {
			if strings.EqualFold(strings.TrimSpace(t), "pod") {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func ruleMatchesResourceType(resourceType string, rule Rule) bool {
	rt := strings.TrimSpace(resourceType)
	switch rt {
	case "ServiceAccount", "Role", "ClusterRole", "RoleBinding", "ClusterRoleBinding":
		return rule.Category == CategoryRBAC
	case "Pod":
		return ruleAppliesToPod(rule)
	default:
		return false
	}
}
