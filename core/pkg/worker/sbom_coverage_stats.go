package worker

// SBOMCoverageStats is a pure, metrics-independent view of SBOM match coverage.
// It is used as a single source of truth to keep ratios and counters consistent.
type SBOMCoverageStats struct {
	TotalComponents      int
	MatchedComponents    int
	NotMatchedComponents int
}

// RawMatchRatio returns how many SBOM components produced any CVE match (raw, before severity filter).
func (s SBOMCoverageStats) RawMatchRatio() float64 {
	if s.TotalComponents == 0 {
		return 0
	}
	return float64(s.MatchedComponents) / float64(s.TotalComponents)
}

// CoverageComponent is a minimal representation of an SBOM component for coverage math.
type CoverageComponent struct {
	ID string
}

// CoverageMatch is a minimal representation of a CVE match.
// Only its existence matters for coverage stats; duplicates don't change matched-components count.
type CoverageMatch struct {
	ID string
}

// ComputeSBOMCoverageStats computes match coverage per component.
//
// Invariant:
// - A component is "matched" if matches[component.ID] has length > 0.
// - duplicates in matches don't increase MatchedComponents; it remains per-component.
func ComputeSBOMCoverageStats(components []CoverageComponent, matches map[string][]CoverageMatch) SBOMCoverageStats {
	stats := SBOMCoverageStats{
		TotalComponents: len(components),
	}

	for _, c := range components {
		if c.ID == "" {
			// Treat empty IDs as unmatched (still counted in total).
			stats.NotMatchedComponents++
			continue
		}
		if list, ok := matches[c.ID]; ok && len(list) > 0 {
			stats.MatchedComponents++
		} else {
			stats.NotMatchedComponents++
		}
	}
	return stats
}

