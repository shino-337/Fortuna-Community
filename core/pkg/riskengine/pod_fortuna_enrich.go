package riskengine

import (
	"context"
	"encoding/json"
	"fmt"
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
func (e *Engine) enrichPodFortunaContext(ctx context.Context, enriched map[string]interface{}) error {
	if e == nil || e.db == nil || enriched == nil {
		return nil
	}
	uid, _ := enriched["uid"].(string)
	if strings.TrimSpace(uid) == "" {
		return nil
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

	if err := e.UpsertAssetSecurityState(ctx, uid); err != nil {
		return fmt.Errorf("project pod security state: %w", err)
	}

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

		// Risk Engine v2 input: expose structured security state to CEL.
		// Kept separate from fortuna.* to allow v2 rules to evolve without breaking legacy rules.
		var runtimeSignalsByType map[string]int64
		if strings.TrimSpace(state.RuntimeSignalsByType) != "" {
			if err := json.Unmarshal([]byte(state.RuntimeSignalsByType), &runtimeSignalsByType); err != nil {
				return fmt.Errorf("decode runtime signals: %w", err)
			}
		}
		if runtimeSignalsByType == nil {
			runtimeSignalsByType = map[string]int64{}
		}

		var effectiveCaps []string
		if strings.TrimSpace(state.EffectiveCapabilities) != "" {
			if err := json.Unmarshal([]byte(state.EffectiveCapabilities), &effectiveCaps); err != nil {
				return fmt.Errorf("decode effective capabilities: %w", err)
			}
		}
		if effectiveCaps == nil {
			effectiveCaps = []string{}
		}

		enriched["fortuna"] = fortuna
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
		return nil
	} else {
		return fmt.Errorf("read pod security state: %w", err)
	}
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
	scoped := false
	for _, tag := range rule.Tags {
		if strings.HasPrefix(tag, "resource-kind:") {
			scoped = true
			if strings.TrimPrefix(tag, "resource-kind:") == rt {
				return true
			}
		}
	}
	if scoped {
		return false
	}
	switch rt {
	case "ServiceAccount", "Role", "ClusterRole", "RoleBinding", "ClusterRoleBinding":
		return rule.Category == CategoryRBAC
	case "Pod":
		return ruleAppliesToPod(rule)
	default:
		return false
	}
}
