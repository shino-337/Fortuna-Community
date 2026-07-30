package graph

import (
	"sort"
	"strings"
)

// Impact bands are string enums: LOW | MEDIUM | HIGH | CRITICAL.
// EffectiveRisk is retained for API compatibility, but it mirrors the unified
// score band. It is not an independent final-risk signal.

// PersistedAttackPathSummary is a subset of DB attack_paths used to merge Layer-3 single paths into
// resource risk signals. Multi-step chains (DetectChains) may omit a workload that only appears on
// standalone persisted paths — without this merge, HasAttackPath / MaxImpact stay LOW incorrectly.
type PersistedAttackPathSummary struct {
	PathID      string
	Description string
	TotalRisk   float64
}

// ResourceRiskSignals is the pure graph→resource interpretation contract (no DB, no runtime, no MITRE use).
type ResourceRiskSignals struct {
	HasAttackPath          bool     `json:"has_attack_path"`
	MaxImpact              string   `json:"max_impact"`
	MaxImpactSourceChainID string   `json:"max_impact_source_chain_id,omitempty"`
	PathCount              int      `json:"path_count"`
	IsEntryPoint           bool     `json:"is_entry_point"`
	IsPivot                bool     `json:"is_pivot"`
	EffectiveRisk          string   `json:"effective_risk"`
	Summary                string   `json:"summary"`
	ChainIDs               []string `json:"chain_ids"`
}

// ChainsTouchingResource returns unique chains where resourceID appears in InvolvedResources (deduped by chain id).
func ChainsTouchingResource(resourceID string, chains []AttackChain) []AttackChain {
	rid := strings.TrimSpace(resourceID)
	if rid == "" {
		return nil
	}
	seenChain := map[string]struct{}{}
	var out []AttackChain
	for i := range chains {
		if !resourceInInvolved(rid, &chains[i]) {
			continue
		}
		k := chainCanonicalID(&chains[i])
		if k == "" {
			out = append(out, chains[i])
			continue
		}
		if _, dup := seenChain[k]; dup {
			continue
		}
		seenChain[k] = struct{}{}
		out = append(out, chains[i])
	}
	return out
}

func chainCanonicalID(ch *AttackChain) string {
	if ch == nil {
		return ""
	}
	if id := strings.TrimSpace(ch.ID); id != "" {
		return id
	}
	return strings.TrimSpace(ch.ChainID)
}

func resourceInInvolved(resourceID string, ch *AttackChain) bool {
	if ch == nil {
		return false
	}
	rid := strings.TrimSpace(resourceID)
	seen := map[string]struct{}{}
	for _, id := range ch.InvolvedResources {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if strings.EqualFold(id, rid) {
			return true
		}
	}
	return false
}

// BuildResourceRiskSignals is pure: filters chains internally, does not mutate input.
// pathByID must be the same path_id map used when building chains (for pivot path counts); nil is ok (pivot uses 0 paths).
// persistedPaths may be nil; when non-empty, MaxImpact / HasAttackPath also reflect standalone DB paths for this pod.
func BuildResourceRiskSignals(resourceID string, resourceScore float64, allChains []AttackChain, pathByID map[string]AttackPath, persistedPaths []PersistedAttackPathSummary) ResourceRiskSignals {
	rid := strings.TrimSpace(resourceID)
	chains := ChainsTouchingResource(rid, allChains)
	s := ResourceRiskSignals{MaxImpact: "LOW"}

	if len(chains) > 0 {
		s = riskSignalsFromTouchingChains(rid, chains, pathByID)
	}

	pImp, pSrc := pickMaxImpactFromPersistedPaths(persistedPaths)
	if len(persistedPaths) > 0 {
		s.HasAttackPath = true
		pc := len(persistedPaths)
		if pc > s.PathCount {
			s.PathCount = pc
		}
		if len(chains) == 0 {
			s.MaxImpact = pImp
			if pSrc != "" {
				s.MaxImpactSourceChainID = "attack_path:" + pSrc
			}
		} else if impactRankOrder(pImp) > impactRankOrder(s.MaxImpact) {
			s.MaxImpact = pImp
			if pSrc != "" {
				s.MaxImpactSourceChainID = "attack_path:" + pSrc
			}
		}
	}

	if !s.HasAttackPath {
		s.EffectiveRisk = computeEffectiveRiskStrict(&s, resourceScore)
		s.Summary = BuildRiskSummary(s)
		return s
	}

	s.EffectiveRisk = computeEffectiveRiskStrict(&s, resourceScore)
	s.Summary = BuildRiskSummary(s)
	return s
}

func riskSignalsFromTouchingChains(rid string, chains []AttackChain, pathByID map[string]AttackPath) ResourceRiskSignals {
	s := ResourceRiskSignals{MaxImpact: "LOW"}
	s.HasAttackPath = true
	s.PathCount = distinctPathIDsCount(chains)

	subsetForMax := chainsForMaxImpactSubset(chains)
	maxImp, srcID := pickMaxImpactAndSourceChain(subsetForMax)
	s.MaxImpact = maxImp
	s.MaxImpactSourceChainID = srcID

	for i := range chains {
		c := &chains[i]
		if strings.EqualFold(strings.TrimSpace(c.SourceID), rid) {
			s.IsEntryPoint = true
		}
	}
	s.IsPivot = resourceIsPivot(rid, chains, pathByID)

	ordered := append([]AttackChain(nil), chains...)
	sortChainsRiskDisplayOrder(ordered)
	for i := range ordered {
		if cid := chainCanonicalID(&ordered[i]); cid != "" {
			s.ChainIDs = append(s.ChainIDs, cid)
		}
	}
	return s
}

func pickMaxImpactFromPersistedPaths(paths []PersistedAttackPathSummary) (impact, pathID string) {
	if len(paths) == 0 {
		return "LOW", ""
	}
	bestRank := 0
	var bestIdx int
	for i := range paths {
		l := impactLabelFromPersistedAttackPath(paths[i].Description, paths[i].TotalRisk)
		r := impactRankOrder(l)
		if r > bestRank {
			bestRank = r
			bestIdx = i
		} else if r == bestRank && r > 0 && paths[i].TotalRisk > paths[bestIdx].TotalRisk {
			bestIdx = i
		}
	}
	l := impactLabelFromPersistedAttackPath(paths[bestIdx].Description, paths[bestIdx].TotalRisk)
	if l == "" {
		l = "LOW"
	}
	return strings.ToUpper(strings.TrimSpace(l)), strings.TrimSpace(paths[bestIdx].PathID)
}

// impactLabelFromPersistedAttackPath maps relational path descriptions (see buildAttackPathDescription) + strength to discrete impact.
func impactLabelFromPersistedAttackPath(desc string, totalRisk float64) string {
	d := strings.ToUpper(desc)
	switch {
	case strings.Contains(d, "PRIV ESC") || strings.Contains(d, "CLUSTER-ADMIN") || strings.Contains(d, "CLUSTER_ADMIN"):
		return "CRITICAL"
	case strings.Contains(d, "ESCAPE"):
		return "HIGH"
	case strings.Contains(d, "LATERAL"):
		return "HIGH"
	case strings.Contains(d, "DATA EXFIL"):
		return "MEDIUM"
	default:
		if totalRisk >= 9.0 {
			return "CRITICAL"
		}
		if totalRisk >= 7.0 {
			return "HIGH"
		}
		if totalRisk >= 4.0 {
			return "MEDIUM"
		}
		return "LOW"
	}
}

func distinctPathIDsCount(chains []AttackChain) int {
	seen := map[string]struct{}{}
	for i := range chains {
		for _, pid := range chains[i].Paths {
			pid = strings.TrimSpace(pid)
			if pid == "" {
				continue
			}
			seen[pid] = struct{}{}
		}
	}
	return len(seen)
}

// chains with capability_validation.confidence >= 0.5; if none, fall back to all touching chains.
func chainsForMaxImpactSubset(touching []AttackChain) []AttackChain {
	var hi []AttackChain
	for i := range touching {
		c := touching[i]
		if c.CapabilityValidation != nil && c.CapabilityValidation.Confidence >= 0.5 {
			hi = append(hi, c)
		}
	}
	if len(hi) > 0 {
		return hi
	}
	out := make([]AttackChain, len(touching))
	copy(out, touching)
	return out
}

func pickMaxImpactAndSourceChain(chains []AttackChain) (maxImpact, sourceChainID string) {
	if len(chains) == 0 {
		return "LOW", ""
	}
	bestRank := 0
	for i := range chains {
		r := impactRankOrder(chains[i].Impact)
		if r > bestRank {
			bestRank = r
		}
	}
	var tier []AttackChain
	for i := range chains {
		if impactRankOrder(chains[i].Impact) == bestRank {
			tier = append(tier, chains[i])
		}
	}
	sort.SliceStable(tier, func(i, j int) bool {
		ci, cj := capConfidence(tier[i]), capConfidence(tier[j])
		if ci != cj {
			return ci > cj
		}
		return chainCanonicalID(&tier[i]) < chainCanonicalID(&tier[j])
	})
	imp := strings.ToUpper(strings.TrimSpace(tier[0].Impact))
	if imp == "" {
		imp = "LOW"
	}
	return imp, chainCanonicalID(&tier[0])
}

func impactRankOrder(impact string) int {
	switch strings.ToUpper(strings.TrimSpace(impact)) {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW":
		return 1
	default:
		return 0
	}
}

func capConfidence(c AttackChain) float64 {
	if c.CapabilityValidation == nil {
		return -1
	}
	return c.CapabilityValidation.Confidence
}

func sortChainsRiskDisplayOrder(chains []AttackChain) {
	sort.SliceStable(chains, func(i, j int) bool {
		ri, rj := impactRankOrder(chains[i].Impact), impactRankOrder(chains[j].Impact)
		if ri != rj {
			return ri > rj
		}
		ci, cj := capConfidence(chains[i]), capConfidence(chains[j])
		if ci != cj {
			return ci > cj
		}
		return chainCanonicalID(&chains[i]) < chainCanonicalID(&chains[j])
	})
}

func pathOccurrenceCountForResource(rid string, chains []AttackChain, pathByID map[string]AttackPath) int {
	if len(pathByID) == 0 {
		return 0
	}
	rid = strings.TrimSpace(rid)
	seenPath := map[string]struct{}{}
	n := 0
	for i := range chains {
		for _, pid := range chains[i].Paths {
			pid = strings.TrimSpace(pid)
			if pid == "" {
				continue
			}
			if _, ok := seenPath[pid]; ok {
				continue
			}
			p, ok := pathByID[pid]
			if !ok {
				continue
			}
			if pathContainsResourceID(p, rid) {
				seenPath[pid] = struct{}{}
				n++
			}
		}
	}
	return n
}

func pathContainsResourceID(p AttackPath, rid string) bool {
	for _, node := range p.Nodes {
		if strings.EqualFold(strings.TrimSpace(node.ID), rid) {
			return true
		}
	}
	return false
}

func resourceIsPivot(rid string, chains []AttackChain, pathByID map[string]AttackPath) bool {
	if len(chains) == 0 {
		return false
	}
	if pathOccurrenceCountForResource(rid, chains, pathByID) < 2 {
		return false
	}
	if isEntryOnAnyChain(rid, chains) || isTargetOnAnyChain(rid, chains) {
		return false
	}
	return true
}

func isEntryOnAnyChain(rid string, chains []AttackChain) bool {
	rid = strings.TrimSpace(rid)
	for i := range chains {
		if strings.EqualFold(strings.TrimSpace(chains[i].SourceID), rid) {
			return true
		}
	}
	return false
}

func isTargetOnAnyChain(rid string, chains []AttackChain) bool {
	rid = strings.TrimSpace(rid)
	for i := range chains {
		if strings.EqualFold(strings.TrimSpace(chains[i].TargetID), rid) {
			return true
		}
	}
	return false
}

func scoreBand(score float64) string {
	if score >= 70 {
		return "CRITICAL"
	}
	if score >= 40 {
		return "HIGH"
	}
	if score >= 20 {
		return "MEDIUM"
	}
	return "LOW"
}

// computeEffectiveRiskStrict returns the ADR score band only.
// Attack-path impact is exposed separately as MaxImpact; it must not override
// the single user-facing risk level derived from the unified score.
func computeEffectiveRiskStrict(s *ResourceRiskSignals, resourceScore float64) string {
	return scoreBand(resourceScore)
}

// ComputeEffectiveRisk exported for unit tests.
func ComputeEffectiveRisk(s ResourceRiskSignals, score float64) string {
	return computeEffectiveRiskStrict(&s, score)
}

// BuildRiskSummary is the fixed UI string contract (spec §2.5) — only HasAttackPath + MaxImpact.
func BuildRiskSummary(s ResourceRiskSignals) string {
	if s.HasAttackPath && strings.ToUpper(strings.TrimSpace(s.MaxImpact)) == "CRITICAL" {
		return "Can be used to take over the cluster"
	}
	if s.HasAttackPath {
		return "Part of an attack path"
	}
	return "No attack path"
}

// AttackPathSortPriority: has_path DESC, max_impact DESC, score band DESC, total_score DESC.
func AttackPathSortPriority(s ResourceRiskSignals, totalScore float64) (hasPath, impactRank, effectiveRank, scoreI int) {
	if s.HasAttackPath {
		hasPath = 1
	}
	switch strings.ToUpper(strings.TrimSpace(s.MaxImpact)) {
	case "CRITICAL":
		impactRank = 4
	case "HIGH":
		impactRank = 3
	case "MEDIUM":
		impactRank = 2
	case "LOW":
		impactRank = 1
	default:
		impactRank = 0
	}
	switch strings.ToUpper(strings.TrimSpace(s.EffectiveRisk)) {
	case "CRITICAL":
		effectiveRank = 4
	case "HIGH":
		effectiveRank = 3
	case "MEDIUM":
		effectiveRank = 2
	case "LOW":
		effectiveRank = 1
	default:
		effectiveRank = 0
	}
	scoreI = int(totalScore + 0.5)
	if scoreI < 0 {
		scoreI = 0
	}
	if scoreI > 100 {
		scoreI = 100
	}
	return hasPath, impactRank, effectiveRank, scoreI
}

func firstPathNodeID(p AttackPath) string {
	if len(p.Nodes) == 0 {
		return ""
	}
	return strings.TrimSpace(p.Nodes[0].ID)
}

func lastPathNodeID(p AttackPath) string {
	if len(p.Nodes) == 0 {
		return ""
	}
	return strings.TrimSpace(p.Nodes[len(p.Nodes)-1].ID)
}

func collectChainPathNodeIDs(a, b pathNormalized) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, p := range []AttackPath{a.Raw, b.Raw} {
		for _, n := range p.Nodes {
			add(n.ID)
		}
	}
	return out
}

func impactLabelFromScope(scope string) string {
	switch strings.ToUpper(strings.TrimSpace(scope)) {
	case "CLUSTER":
		return "CRITICAL"
	case "NODE":
		return "HIGH"
	case "NAMESPACE":
		return "MEDIUM"
	default:
		return "LOW"
	}
}
