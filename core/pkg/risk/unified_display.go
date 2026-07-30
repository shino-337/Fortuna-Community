package risk

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// CanonicalBreakdownItem is one row of the unified explainability contract (GET /risk/insights breakdown[]).
type CanonicalBreakdownItem struct {
	FactorID     string   `json:"factor_id"`
	Category     string   `json:"category"`
	Scope        string   `json:"scope"`
	Source       string   `json:"source"`
	Contribution float64  `json:"contribution"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// DeriveFinalLevelFromScore maps authoritative 0–100 score to ADR risk level bands.
func DeriveFinalLevelFromScore(score float64) string {
	switch {
	case score >= 70:
		return "critical"
	case score >= 40:
		return "high"
	case score >= 20:
		return "medium"
	default:
		return "low"
	}
}

// ParseBreakdownFromFactorsJSON extracts aggregation.risk_factors[] from risk_scores.factors JSON.
func ParseBreakdownFromFactorsJSON(raw string) []CanonicalBreakdownItem {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	agg, ok := payload["aggregation"].(map[string]interface{})
	if !ok {
		return nil
	}
	factors, ok := agg["risk_factors"].([]interface{})
	if !ok || len(factors) == 0 {
		return nil
	}
	out := make([]CanonicalBreakdownItem, 0, len(factors))
	for _, item := range factors {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		row := CanonicalBreakdownItem{
			FactorID: strings.TrimSpace(jsonToString(m["factor_id"])),
			Category: strings.TrimSpace(jsonToString(m["category"])),
			Scope:    strings.TrimSpace(jsonToString(m["scope"])),
			Source:   strings.TrimSpace(jsonToString(m["source"])),
		}
		if c, ok := jsonToFloat64(m["contribution"]); ok {
			row.Contribution = math.Round(c*100) / 100
		}
		if refs, ok := m["evidence_refs"].([]interface{}); ok {
			for _, ref := range refs {
				s := strings.TrimSpace(jsonToString(ref))
				if s != "" {
					row.EvidenceRefs = append(row.EvidenceRefs, s)
				}
			}
		}
		if row.FactorID == "" && row.Category == "" && row.Contribution == 0 {
			continue
		}
		out = append(out, row)
	}
	return out
}

// UnifiedDimensionsV3 is the 7 V3 sub-scores exposed to the API (camelCase, matches TS UnifiedRiskScore.dimensions).
// Values are read from risk_scores.factors JSON at factors.dimensions (snake_case keys in storage).
type UnifiedDimensionsV3 struct {
	Vulnerability      float64 `json:"vulnerability"`
	CapabilityExposure float64 `json:"capabilityExposure"`
	AttackPath         float64 `json:"attackPath"`
	RbacPolicy         float64 `json:"rbacPolicy"`
	RuntimeThreat      float64 `json:"runtimeThreat"`
	Exposure           float64 `json:"exposure"`
	BlastRadius        float64 `json:"blastRadius"`
}

// ParseDimensionScoresV3FromFactorsJSON extracts factors.dimensions from serialized risk score factors.
func ParseDimensionScoresV3FromFactorsJSON(raw string) *UnifiedDimensionsV3 {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	inner, ok := payload["dimensions"].(map[string]interface{})
	if !ok || inner == nil {
		return nil
	}
	// storage keys: vulnerability, capability_exposure, attack_path, rbac_policy, runtime_threat, exposure, blast_radius
	get := func(snake, plain string) float64 {
		if v, ok := jsonToFloat64(inner[snake]); ok {
			return v
		}
		if plain != "" {
			if v, ok := jsonToFloat64(inner[plain]); ok {
				return v
			}
		}
		return 0
	}
	return &UnifiedDimensionsV3{
		Vulnerability:      get("vulnerability", "vulnerability"),
		CapabilityExposure: get("capability_exposure", "capabilityExposure"),
		AttackPath:         get("attack_path", "attackPath"),
		RbacPolicy:         get("rbac_policy", "rbacPolicy"),
		RuntimeThreat:      get("runtime_threat", "runtimeThreat"),
		Exposure:           get("exposure", "exposure"),
		BlastRadius:        get("blast_radius", "blastRadius"),
	}
}

func jsonToString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	default:
		return ""
	}
}

func jsonToFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}
