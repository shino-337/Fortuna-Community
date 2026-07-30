package graph

// MitreTechniqueRef is a stable ATT&CK technique reference for UI and correlation.
type MitreTechniqueRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Tactic string `json:"tactic,omitempty"`
	URL    string `json:"url,omitempty"`
}

// MitreCoverageItem is minimal gap/status per MITRE id on a chain (Phase 3 usable subset).
type MitreCoverageItem struct {
	MitreID string `json:"mitre_id"`
	Status  string `json:"status"` // observed | inferred | not_covered | unknown
	// Confidence for inferred/observed/not_covered when derived from chain realism × avg step grounding; omitted for unknown.
	Confidence *float64 `json:"confidence,omitempty"`
	Priority   string   `json:"priority,omitempty"` // HIGH | MEDIUM | LOW
}

// AttackChainMitreSummary correlates path-derived MITRE IDs with runtime observations.
type AttackChainMitreSummary struct {
	PathMitreDistinct      int      `json:"path_mitre_distinct"`
	RuntimeMitreDistinct   int      `json:"runtime_mitre_distinct"`
	MatchedMitreIDs        []string `json:"matched_mitre_ids"`
	AlignmentRatio         float64  `json:"alignment_ratio"`
	ObservingPodCount      int      `json:"observing_pod_count"`
	// CorrelationPrecision = matched_events_for_steps / max(1, runtime_events_considered)
	CorrelationPrecision   float64 `json:"correlation_precision,omitempty"`
	RuntimeEventsConsidered int    `json:"runtime_events_considered,omitempty"`
	RuntimeEventsMatched    int    `json:"runtime_events_matched,omitempty"`
}
