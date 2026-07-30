package graph

import (
	"context"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ComputeAttackPathMitreBoost scores runtime_events with decay × technique_weight × tactic_modifier,
// then applies burst damping sum/(log(1+n)). Returns boost capped by MitreBoostCap().
// pathRelevantMitre: when non-empty, runtime MITRE ids not on persisted attack paths are down-weighted (noise guard).
// correlationPrecision = events whose MITRE id is known in overlay index / max(1, total events).
func ComputeAttackPathMitreBoost(db *gorm.DB, podUID string, window time.Duration, pathRelevantMitre map[string]struct{}) (boost float64, correlationPrecision float64, meta map[string]interface{}) {
	meta = map[string]interface{}{}
	if db == nil || podUID == "" || !db.Migrator().HasTable("runtime_events") {
		return 0, 1.0, meta
	}
	var targetNS string
	_ = db.WithContext(context.Background()).Raw(
		`SELECT COALESCE(namespace,'') FROM pods WHERE uid = ? AND deleted_at IS NULL LIMIT 1`, podUID,
	).Scan(&targetNS)

	since := time.Now().Add(-window)
	var rows []runtimeEventLite
	q := `
SELECT TRIM(UPPER(re.mitre_technique)) AS mitre_technique,
       re.created_at AS created_at,
       COALESCE(re.node_name, '') AS node_name,
       re.pod_uid AS pod_uid,
       COALESCE(p.namespace, '') AS pod_namespace
FROM runtime_events AS re
INNER JOIN pods AS p ON p.uid = re.pod_uid AND p.deleted_at IS NULL
WHERE re.pod_uid = ?
  AND TRIM(re.mitre_technique) <> ''
  AND COALESCE(re.confidence, 0) > 0
  AND re.created_at > ?
ORDER BY re.created_at DESC`
	if err := db.WithContext(context.Background()).Raw(q, podUID, since).Scan(&rows).Error; err != nil {
		return 0, 1.0, meta
	}
	n := len(rows)
	if n == 0 {
		meta["runtime_mitre_event_count"] = 0
		meta["runtime_mitre_techniques_7d"] = []string{}
		return 0, 1.0, meta
	}
	hl := DefaultHalfLifeHours() * float64(time.Hour)
	if hl <= 0 {
		hl = 72 * float64(time.Hour)
	}
	now := time.Now()
	sumWeighted := 0.0
	matchedIndex := 0
	irrelevantN := 0
	for _, r := range rows {
		mid := normalizeMitreIDForOverlay(r.Mitre)
		if mid == "" {
			continue
		}
		pathRelevanceMul := 1.0
		if len(pathRelevantMitre) > 0 {
			if _, ok := pathRelevantMitre[mid]; !ok {
				pathRelevanceMul = 0.5
				irrelevantN++
			}
		}
		age := now.Sub(r.CreatedAt)
		if age < 0 {
			age = 0
		}
		decay := math.Exp(-float64(age) / hl)
		eff := EffectiveMitreWeight(mid)
		scopeMul := MitreRuntimeScopeTierMultiplier(r, mid, targetNS)
		_, tactic, _ := MitreRiskRecord(mid)
		if strings.EqualFold(strings.TrimSpace(tactic), "Discovery") {
			scopeMul = math.Min(scopeMul, 1.05)
		}
		tacticRealityMul := attackPathMitreTacticRealityMultiplier(tactic)
		if overlaySemanticFromYAMLFileEnabled() {
			if _, ok := mitreRiskIndex[mid]; ok {
				matchedIndex++
			}
		}
		sumWeighted += decay * eff * scopeMul * tacticRealityMul * pathRelevanceMul
	}
	burst := sumWeighted / math.Log(1.0+float64(n))
	scale := MitreBoostScale()
	cap := MitreBoostCap()
	boost = math.Min(cap, burst*scale)
	correlationPrecision = float64(matchedIndex) / float64(math.Max(1, float64(n)))

	meta["runtime_mitre_event_count"] = n
	meta["runtime_mitre_matched_overlay"] = matchedIndex
	meta["mitre_decay_half_life_hours"] = hl / float64(time.Hour)
	meta["mitre_weighted_sum_pre_burst"] = math.Round(sumWeighted*1000) / 1000
	meta["mitre_burst_denominator"] = math.Round(math.Log(1.0+float64(n))*1000) / 1000
	meta["mitre_correlation_precision"] = math.Round(correlationPrecision*1000) / 1000
	meta["mitre_scope_target_namespace"] = targetNS
	distinct := make([]string, 0)
	seen := map[string]bool{}
	for _, r := range rows {
		mid := normalizeMitreIDForOverlay(r.Mitre)
		if mid != "" && !seen[mid] {
			seen[mid] = true
			distinct = append(distinct, mid)
		}
	}
	meta["runtime_mitre_techniques_7d"] = distinct
	meta["mitre_path_relevance_set_size"] = len(pathRelevantMitre)
	meta["mitre_boost_path_irrelevant_events"] = irrelevantN
	return boost, correlationPrecision, meta
}

// attackPathMitreTacticRealityMultiplier biases runtime MITRE boost toward post-access tactics
// (execution, escalation, impact) vs scan-heavy discovery noise.
func attackPathMitreTacticRealityMultiplier(tactic string) float64 {
	t := strings.ToLower(strings.TrimSpace(tactic))
	switch t {
	case "execution", "privilege escalation", "impact":
		return 1.4
	case "lateral movement":
		return 1.2
	case "discovery":
		return 0.8
	default:
		return 1.0
	}
}
