package rep

import (
	"encoding/json"
	"math"
	"strings"
)

// mergeIncidentConfidenceOnSuppress adjusts confidence when we update an existing incident
// inside cooldown (G-REP-01): reinforce when new evidence grows; slight decay on noisy repeats.
func mergeIncidentConfidenceOnSuppress(prevConf, newConf float64, prevEvidenceCount, newEvidenceCount int) float64 {
	prevConf = clamp01(prevConf)
	newConf = clamp01(newConf)
	if newEvidenceCount > prevEvidenceCount {
		// Stronger signal: bump toward certainty, capped.
		return math.Min(1.0, math.Max(prevConf, newConf)+0.03)
	}
	if newEvidenceCount == prevEvidenceCount && prevEvidenceCount > 0 {
		// Same evidence footprint seen again: small decay (repeat firing).
		return math.Max(0.05, prevConf*0.99)
	}
	return math.Max(prevConf, newConf)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func evidenceRefCountJSON(evidenceRefsJSON string) int {
	s := strings.TrimSpace(evidenceRefsJSON)
	if s == "" || s == "[]" {
		return 0
	}
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return 0
	}
	return len(arr)
}
