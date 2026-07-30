package risk

import (
	"math"
	"os"
	"strings"
)

const envRiskContextTier = "FORTUNA_RISK_CONTEXT_TIER"

// RiskContextInfo captures how environment / exposure adjust the aggregation context multiplier.
type RiskContextInfo struct {
	Tier                 string  `json:"tier"`
	BaseMultiplier       float64 `json:"base_multiplier"`
	InternetFacing       bool    `json:"internet_facing"`
	SystemNamespace      bool    `json:"system_namespace"`
	EffectiveMultiplier  float64 `json:"effective_multiplier"`
}

// ResolveRiskContextMultiplier picks a tiered base multiplier then applies internet-facing and system-namespace rules.
// Tiers (FORTUNA_RISK_CONTEXT_TIER): dev=0.9, staging=1.0, prod=1.15. Unset defaults to staging (1.0) for backward-neutral clusters.
// Internet-facing: hostNetwork or exposure dimension score >= internetFacingExposureMin forces at least 1.2.
// System namespaces (kube-system, etc.): effective *= 0.9 on top of the above.
func ResolveRiskContextMultiplier(namespace string, systemNS, hostNetwork bool, exposureDimScore float64) RiskContextInfo {
	tier := strings.ToLower(strings.TrimSpace(os.Getenv(envRiskContextTier)))
	if tier == "" {
		tier = "staging"
	}
	var base float64
	switch tier {
	case "dev":
		base = 0.9
	case "staging":
		base = 1.0
	case "prod":
		base = 1.15
	default:
		base = 1.0
	}
	internet := hostNetwork || exposureDimScore >= internetFacingExposureMin
	eff := base
	if internet {
		eff = math.Max(eff, 1.2)
	}
	if systemNS {
		eff *= 0.9
	}
	if eff <= 0 {
		eff = 1.0
	}
	return RiskContextInfo{
		Tier:                tier,
		BaseMultiplier:      base,
		InternetFacing:      internet,
		SystemNamespace:     systemNS,
		EffectiveMultiplier: eff,
	}
}

// internetFacingExposureMin is the raw exposure dimension score (0–dimMaxExposure) above which we treat the workload as internet-facing for context tiering.
const internetFacingExposureMin = 8.0
