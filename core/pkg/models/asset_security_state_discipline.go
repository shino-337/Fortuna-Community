package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Schema groups for asset_security_state (ADR-003). SQL columns are assigned to groups here for documentation and CI validation hooks.
const (
	GroupIdentityContext           = "identity_context"
	GroupExposureContext           = "exposure_context"
	GroupSoftwareRiskContext       = "software_risk_context"
	GroupRuntimeSecurityContext    = "runtime_security_context"
	GroupEffectiveCapabilityContext = "effective_capability_context"
)

// AssetSecurityStateColumnGroup maps each exported schema field name (JSON tag base) to its ADR-003 group.
// New rows must be added when new persisted columns are introduced.
var AssetSecurityStateColumnGroup = map[string]string{
	"namespace":                         GroupIdentityContext,
	"clusterId":                         GroupIdentityContext,
	"serviceAccountBoundToClusterAdmin": GroupIdentityContext,
	"hostNetwork":                       GroupExposureContext,
	"hostPID":                           GroupExposureContext,
	"hostIPC":                           GroupExposureContext,
	"signalTotal24h":                    GroupRuntimeSecurityContext,
	"hasSuspiciousExec":                 GroupRuntimeSecurityContext,
	"hasNetworkQueueAnomaly":            GroupRuntimeSecurityContext,
	"hasEscapeRelated":                  GroupRuntimeSecurityContext,
	"lastRuntimeActivityAt":             GroupRuntimeSecurityContext,
	"runtimeSignalsByType":              GroupRuntimeSecurityContext,
	"effectiveCapabilities":             GroupEffectiveCapabilityContext,
}

// ValidateAssetSecurityStateJSON validates JSONB columns match expected top-level shapes (object / array).
func ValidateAssetSecurityStateJSON(state *AssetSecurityState) error {
	if state == nil {
		return nil
	}
	s := strings.TrimSpace(state.RuntimeSignalsByType)
	if s != "" && s != "{}" {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			return fmt.Errorf("runtimeSignalsByType: %w", err)
		}
	}
	s = strings.TrimSpace(state.EffectiveCapabilities)
	if s != "" && s != "[]" {
		var arr []interface{}
		if err := json.Unmarshal([]byte(s), &arr); err != nil {
			return fmt.Errorf("effectiveCapabilities: %w", err)
		}
	}
	return nil
}
