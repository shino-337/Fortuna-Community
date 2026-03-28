package riskengine

// G-RE-01 scaffold: factorized score dimensions + provenance (no worker cut-over yet).
// See docs/adr/* and FORTUNA_RUNTIME_IMPLEMENTATION_BACKLOG.md G-RE-01.

// ScoreDimension identifies a contributor bucket for risk scoring v3.
type ScoreDimension string

const (
	DimExposure           ScoreDimension = "exposure"
	DimPrivilege          ScoreDimension = "privilege"
	DimSoftwareRisk       ScoreDimension = "software_risk"
	DimRuntimeThreat      ScoreDimension = "runtime_threat"
	DimCapabilityRisk     ScoreDimension = "capability_risk"
	DimConfidence         ScoreDimension = "confidence"
	DimFreshness          ScoreDimension = "freshness"
)

// DimensionProvenance records why a dimension received a contribution (for explainability / golden tests).
type DimensionProvenance struct {
	Dimension ScoreDimension `json:"dimension"`
	Reason    string         `json:"reason"`    // stable machine-readable code
	Weight    float64        `json:"weight"`    // applied weight before normalization (0 ok)
	Detail    string         `json:"detail,omitempty"`
}

// ScoreV3Snapshot is a portable struct for future persistence on risk_scores / worker output.
type ScoreV3Snapshot struct {
	ScorerVersion string                 `json:"scorerVersion"`
	Total         float64                `json:"total"`
	ByDimension   map[ScoreDimension]float64 `json:"byDimension"`
	Provenance    []DimensionProvenance  `json:"provenance"`
}

// MergeDimensionScores sums non-negative dimension values into Total (MVP helper).
func MergeDimensionScores(by map[ScoreDimension]float64) float64 {
	var t float64
	for _, v := range by {
		if v > 0 {
			t += v
		}
	}
	return t
}
