package graph

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	maxChainsPerObjective = 5
)

type pathEndpoint struct {
	Type string
	ID   string
	SA   string
}

type pathNormalized struct {
	PathID       string
	Source       pathEndpoint
	Target       pathEndpoint
	Class        string
	Provides     []string
	Requires     []string
	Strength     float64
	Confidence   string
	SemanticCaps []string // derived from RBAC rules via DeriveCapabilities (FIX E)
	Raw          AttackPath
}

// DetectChains runs full chain-detection rules with confidence, dedup, and anti-explosion limits.
// Uses default cluster hardening hints (no penalties applied for unknown context).
func DetectChains(paths []AttackPath) []AttackChain {
	return DetectChainsWithHints(paths, DefaultHardeningHints())
}

// DetectChainsWithHints is the full implementation used when cluster hardening
// context is available (e.g. from BuildAttackPathsViewBundle via clusterPathSnapshot).
// FIX I: passes hints into computeChainConfidenceWithHints for context-aware realism.
func DetectChainsWithHints(paths []AttackPath, hints ClusterHardeningHints) []AttackChain {
	norm := normalizePaths(paths)
	if len(norm) < 2 {
		return nil
	}
	out := make([]AttackChain, 0)
	seen := map[string]bool{}

	for i := range norm {
		for j := range norm {
			if i == j {
				continue
			}
			a, b := norm[i], norm[j]
			ruleType, ruleFactor, ok := chainRuleMatch(a, b)
			if !ok {
				continue
			}
			flow := capabilityIntersection(a.Provides, b.Requires)
			if len(flow) == 0 {
				continue
			}
			softPenalty := 1.0
			if hasSoftEdge(a.Raw) || hasSoftEdge(b.Raw) {
				softPenalty = 0.8
			}
			baseChainFactor := ruleFactor * softPenalty
			// FIX H+I+K+L: dependency-aware + context-aware confidence.
			conf, adjustedFactor := computeChainConfidenceWithHints(a, b, ruleType, baseChainFactor, hints)
			strength := clampFloat(math.Min(a.Strength, b.Strength)*adjustedFactor, 0, 1)
			finalTarget := fmt.Sprintf("%s:%s", b.Target.Type, b.Target.ID)
			objective := objectiveFromChainContext(b)
			explanation := buildChainExplanation(ruleType, a, b)
			key := fmt.Sprintf("%s|%s|%s|%s", a.PathID, b.PathID, finalTarget, strings.Join(flow, ","))
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, AttackChain{
				ChainID:       fmt.Sprintf("chain:%s:%s", a.PathID, b.PathID),
				Paths:         []string{a.PathID, b.PathID},
				Objective:     objective,
				ChainStrength: math.Round(strength*1000) / 1000,
				Confidence:    conf,
				FinalTarget:   finalTarget,
				Explanation:   explanation,
				Type:          ruleType,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Objective != out[j].Objective {
			return out[i].Objective < out[j].Objective
		}
		if out[i].FinalTarget != out[j].FinalTarget {
			return out[i].FinalTarget < out[j].FinalTarget
		}
		return out[i].ChainStrength > out[j].ChainStrength
	})
	// FIX J: blast-radius-aware semantic dedup.
	out = deduplicateChainsSemantically(out, norm)

	// Count distinct node-type nodes in the normalized paths for blast radius calculation.
	totalNodes := countDistinctNodes(norm)

	// Phase 2: enrich each chain with Realism, ExploitCost, BlastRadius, Validity, Story.
	// Resolve the pair from chain.Paths after sort/dedup so provenance stays attached
	// to the same path IDs shown in the API/UI.
	for idx := range out {
		if a, b, ok := normalizedPairForChain(out[idx], norm); ok {
			enrichChain(&out[idx], a, b, hints, norm, totalNodes)
		}
	}

	// Phase 2: mark Primary=true per objective (highest priority score).
	out = selectPrimaryChains(out)

	return enforceObjectiveLimit(out, maxChainsPerObjective)
}

func normalizedPairForChain(ch AttackChain, norm []pathNormalized) (pathNormalized, pathNormalized, bool) {
	if len(ch.Paths) < 2 {
		return pathNormalized{}, pathNormalized{}, false
	}
	var a, b pathNormalized
	var foundA, foundB bool
	for _, p := range norm {
		switch p.PathID {
		case ch.Paths[0]:
			a = p
			foundA = true
		case ch.Paths[1]:
			b = p
			foundB = true
		}
	}
	return a, b, foundA && foundB
}

// countDistinctNodes counts unique Kubernetes node IDs across all normalized paths.
func countDistinctNodes(norm []pathNormalized) int {
	nodes := map[string]bool{}
	for _, p := range norm {
		for _, n := range p.Raw.Nodes {
			if strings.EqualFold(n.Type, NodeTypeNode) {
				nodes[n.ID] = true
			}
		}
	}
	if len(nodes) == 0 {
		return 1 // avoid division-by-zero; single node minimum
	}
	return len(nodes)
}

func normalizePaths(paths []AttackPath) []pathNormalized {
	out := make([]pathNormalized, 0, len(paths))
	for idx, p := range paths {
		if len(p.Nodes) < 2 {
			continue
		}
		sourceNode := p.Nodes[0]
		targetNode := p.Nodes[len(p.Nodes)-1]
		source := pathEndpoint{
			Type: strings.ToUpper(strings.TrimSpace(sourceNode.Type)),
			ID:   sourceNode.ID,
			SA:   saFromPath(p),
		}
		target := pathEndpoint{
			Type: strings.ToUpper(strings.TrimSpace(targetNode.Type)),
			ID:   targetNode.ID,
		}
		cls := pathClassFromPath(p)
		strength := clampFloat(p.TotalRisk/10.0, 0, 1)
		conf := "low"
		if p.Explainability != nil {
			strength = clampFloat(p.Explainability.Strength, 0, 1)
			conf = confidenceBand(p.Explainability.Feasibility.Confidence)
		}
		// FIX E: read SemanticCaps from target node properties
		// (written by semanticCapsFromRules via convertGraphPathsToAttackPaths).
		var semanticCaps []string
		if lastNode := p.Nodes[len(p.Nodes)-1]; lastNode.Properties != nil {
			if sc, ok := lastNode.Properties["semantic_caps"].([]string); ok {
				semanticCaps = sc
			}
		}
		provides := deriveProvides(p, source, target, cls)
		// Merge SemanticCaps into provides so chain rules can match them directly.
		if len(semanticCaps) > 0 {
			provides = uniqStrings(append(provides, semanticCaps...))
		}
		requires := deriveRequires(p, source, target, cls)
		out = append(out, pathNormalized{
			PathID:       fmt.Sprintf("p%d", idx),
			Source:       source,
			Target:       target,
			Class:        cls,
			Provides:     provides,
			Requires:     requires,
			Strength:     strength,
			Confidence:   conf,
			SemanticCaps: semanticCaps,
			Raw:          p,
		})
	}
	return out
}

func deriveProvides(p AttackPath, source, target pathEndpoint, cls string) []string {
	provides := []string{}
	if source.SA != "" {
		provides = append(provides, "SA_TOKEN")
	}
	switch cls {
	case "ESCAPE":
		if target.Type == strings.ToUpper(NodeTypeNode) {
			provides = append(provides, "NODE_ACCESS")
		}
	case "LATERAL":
		if target.Type == strings.ToUpper(NodeTypePod) {
			provides = append(provides, "NETWORK_ACCESS:"+target.ID)
		}
	case "PRIV_ESC":
		if target.Type == strings.ToUpper(NodeTypeClusterRole) || target.Type == strings.ToUpper(NodeTypeRole) {
			provides = append(provides, "ROLE")
			provides = append(provides, "ROLE:"+target.ID)
		}
	case "DATA_EXFIL":
		provides = append(provides, "DATA_ACCESS:"+target.ID)
	}
	if target.Type == strings.ToUpper(NodeTypePod) {
		provides = append(provides, "NETWORK_ACCESS:"+target.ID)
	}
	for _, n := range p.Nodes {
		if strings.EqualFold(n.Type, NodeTypeNode) {
			provides = append(provides, "NODE_ACCESS")
		}
	}
	// Phase 3 (spec §3.3): NO implicit SA_TOKEN derivation.
	// Previously NODE_ACCESS would auto-emit SA_TOKEN:* — this was the root of
	// over-chaining. Now, the tiered access token is emitted (NODE_SHELL_ACCESS or
	// CONTAINER_RUNTIME_ACCESS), but SA_TOKEN is NOT added automatically.
	// SA_TOKEN must come from an explicit technique step: KUBELET_TOKEN_HARVEST
	// or RUNTIME_TOKEN_HARVEST (registered in technique_registry.go).
	if hasExact(provides, "NODE_ACCESS") {
		for _, n := range p.Nodes {
			if n.Type != NodeTypeCapability {
				continue
			}
			switch nodeAccessFromCapability(n.ID) {
			case NodeAccessShell:
				provides = append(provides, NodeAccessShell)
			case NodeAccessRuntime:
				provides = append(provides, NodeAccessRuntime)
			}
		}
		// Emit NodeAccessKubelet as a potential — requires KUBELET_API_PROBE technique.
		if hasExact(provides, NodeAccessShell) {
			provides = append(provides, NodeAccessKubelet)
		}
		// ⚠️ SA_TOKEN is intentionally NOT emitted here.
		// It requires explicit technique: KUBELET_TOKEN_HARVEST or RUNTIME_TOKEN_HARVEST.
	}
	return uniqStrings(provides)
}

func deriveRequires(_ AttackPath, source pathEndpoint, target pathEndpoint, cls string) []string {
	requires := []string{}
	switch cls {
	case "PRIV_ESC":
		requires = append(requires, "ROLE")
		requires = append(requires, "SA_TOKEN")
		requires = append(requires, "NODE_ACCESS")
	case "DATA_EXFIL":
		requires = append(requires, "SA_TOKEN")
	}
	if source.ID != "" {
		requires = append(requires, "NETWORK_ACCESS:"+source.ID)
	}
	if target.Type == strings.ToUpper(NodeTypePod) {
		requires = append(requires, "NETWORK_ACCESS:"+target.ID)
	}
	return uniqStrings(requires)
}

func chainRuleMatch(a, b pathNormalized) (ruleType string, factor float64, ok bool) {
	// C1: ESCAPE -> PRIV_ESC (strong)
	if a.Class == "ESCAPE" && b.Class == "PRIV_ESC" && a.Target.Type == strings.ToUpper(NodeTypeNode) {
		return "ESCAPE_TO_PRIV_ESC", 1.0, true
	}
	// C2: LATERAL -> PRIV_ESC
	if a.Class == "LATERAL" && b.Class == "PRIV_ESC" && a.Target.ID == b.Source.ID {
		return "LATERAL_TO_PRIV_ESC", 0.9, true
	}
	// C3: PRIV_ESC -> PRIV_ESC escalation ladder
	if a.Class == "PRIV_ESC" && b.Class == "PRIV_ESC" && privilegeLevelFromPath(b) > privilegeLevelFromPath(a) {
		return "PRIV_ESC_LADDER", 0.9, true
	}
	// C4: ANY -> DATA_EXFIL based on access foothold
	if b.Class == "DATA_EXFIL" && hasAnyPrefix(a.Provides, "NODE_ACCESS", "ROLE:", "NETWORK_ACCESS:") {
		return "ACCESS_TO_DATA_EXFIL", 0.8, true
	}
	// C5: NETWORK bridge
	for _, p := range a.Provides {
		if strings.HasPrefix(p, "NETWORK_ACCESS:") {
			podID := strings.TrimPrefix(p, "NETWORK_ACCESS:")
			if podID == b.Source.ID {
				return "NETWORK_BRIDGE", 0.8, true
			}
		}
	}
	// C6: SA token reuse — SA_TOKEN must now always be technique-derived (spec §3.3).
	// No wildcard logic needed since SA_TOKEN:* is no longer implicitly emitted.
	// Match: path A provides SA_TOKEN AND path B requires SA_TOKEN AND paths share SA or are reachable.
	if hasExact(a.Provides, "SA_TOKEN") && hasExact(b.Requires, "SA_TOKEN") {
		sameSA := a.Source.SA != "" && a.Source.SA == b.Source.SA
		reachable := a.Target.ID == b.Source.ID ||
			hasExact(a.Provides, "NETWORK_ACCESS:"+b.Source.ID)
		nodeReachable := reachable || hasExact(a.Provides, "NODE_ACCESS")
		if sameSA || (nodeReachable && hasExact(a.Provides, NodeAccessShell)) {
			return "SA_TOKEN_REUSE", 0.75, true
		}
	}
	// C7: Same ServiceAccount shortcut (weak)
	if a.Source.SA != "" && a.Source.SA == b.Source.SA {
		return "SAME_SERVICE_ACCOUNT", 0.6, true
	}
	// C8: Node dominance shortcut (weak)
	if hasExact(a.Provides, "NODE_ACCESS") {
		return "NODE_DOMINANCE", 0.6, true
	}
	return "", 0, false
}

func hasSoftEdge(p AttackPath) bool {
	for _, e := range p.Edges {
		if e.Type == EdgeTypeNetworkReachSoft {
			return true
		}
	}
	return false
}

func objectiveFromChainContext(b pathNormalized) string {
	t := strings.ToUpper(b.Target.Type)
	switch t {
	case strings.ToUpper(NodeTypeNode):
		return "NODE_COMPROMISE"
	case strings.ToUpper(NodeTypeClusterRole):
		if privilegeLevelFromID(b.Target.ID) >= 4 {
			return "CLUSTER_TAKEOVER"
		}
		// FIX E: prefer SemanticCaps-derived objective over name-based fallback.
		if obj := objectiveFromSemanticCaps(b.SemanticCaps); obj != "" {
			return obj
		}
		return classifyObjectiveByRole(b.Target.ID)
	case strings.ToUpper(NodeTypeRole):
		if obj := objectiveFromSemanticCaps(b.SemanticCaps); obj != "" {
			return obj
		}
		return classifyObjectiveByRole(b.Target.ID)
	default:
		return "DATA_EXFILTRATION"
	}
}

// objectiveFromSemanticCaps maps DeriveCapabilities output to an attack objective.
// Returns empty string when no mapping is found (caller falls back to name-based classifyObjectiveByRole).
//
// Priority tiers (checked in order, highest first):
//  1. IDENTITY_FORGE  → CLUSTER_TAKEOVER
//  2. NODE_ACCESS     → NODE_COMPROMISE
//  3. WORKLOAD_CONTROL / EXECUTION → WORKLOAD_CONTROL
//  4. DATA_ACCESS:secrets → SECRET_EXFIL
//
// We check each tier independently of caps slice order to ensure the most
// severe capability wins, regardless of how DeriveCapabilities ordered the output.
func objectiveFromSemanticCaps(caps []string) string {
	if len(caps) == 0 {
		return ""
	}
	capSet := make(map[string]bool, len(caps))
	for _, c := range caps {
		capSet[c] = true
	}
	// Tier 1: full cluster identity control
	if capSet["IDENTITY_FORGE"] {
		return "CLUSTER_TAKEOVER"
	}
	// Tier 2: node-level control
	if capSet["NODE_ACCESS"] || capSet[NodeAccessKubelet] {
		return "NODE_COMPROMISE"
	}
	// Tier 3: workload manipulation
	if capSet["WORKLOAD_CONTROL"] || capSet["EXECUTION"] {
		return "WORKLOAD_CONTROL"
	}
	// Tier 4: data access
	if capSet["DATA_ACCESS:secrets"] {
		return "SECRET_EXFIL"
	}
	return ""
}

// classifyObjectiveByRole maps a role ID to a semantic attack objective.
// Cases are ordered by severity (highest first) to handle compound role names
// correctly (e.g. "secret-storage-admin" → CLUSTER_PRIVILEGE_ESCALATION, not SECRET_EXFIL).
func classifyObjectiveByRole(roleID string) string {
	lower := strings.ToLower(strings.TrimPrefix(roleID, "role:"))
	switch {
	case strings.Contains(lower, "admin"):
		return "CLUSTER_PRIVILEGE_ESCALATION"
	case strings.Contains(lower, "deploy") || strings.Contains(lower, "workload") || strings.Contains(lower, "daemonset"):
		return "WORKLOAD_CONTROL"
	case strings.Contains(lower, "secret"):
		return "SECRET_EXFIL"
	case strings.Contains(lower, "storage") || strings.Contains(lower, "pv") || strings.Contains(lower, "volume"):
		return "STORAGE_ABUSE"
	default:
		return "LIMITED_RBAC_IMPACT"
	}
}

func buildChainExplanation(rule string, a, b pathNormalized) string {
	switch rule {
	case "ESCAPE_TO_PRIV_ESC":
		return "Container escape provides node access; node control enables token harvest and privilege escalation via role binding chain."
	case "LATERAL_TO_PRIV_ESC":
		return "Lateral movement reaches another pod and continues to privilege escalation path."
	case "PRIV_ESC_LADDER":
		return "Initial role escalation unlocks a stronger role, creating a privilege-escalation ladder."
	case "ACCESS_TO_DATA_EXFIL":
		return "Foothold capabilities from the first path enable follow-on data exfiltration."
	case "NETWORK_BRIDGE":
		return "Network access from one path bridges into another pod-specific privilege path."
	case "SA_TOKEN_REUSE":
		return "Stolen or mounted service-account token is reused to execute a second-stage path."
	case "SAME_SERVICE_ACCOUNT":
		return "Paths share the same service account identity, enabling weaker but plausible chaining."
	case "NODE_DOMINANCE":
		return "Node-level control from the first path makes additional follow-on paths feasible."
	default:
		return "Chain combines two feasible paths into a stronger attack objective."
	}
}

func capabilityIntersection(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	out := make([]string, 0)
	for _, y := range b {
		if set[y] {
			out = append(out, y)
		}
	}
	return uniqStrings(out)
}

func minConfidence(a, b string) string {
	rank := func(v string) int {
		switch strings.ToLower(v) {
		case "high":
			return 3
		case "medium":
			return 2
		default:
			return 1
		}
	}
	if rank(a) < rank(b) {
		return strings.ToLower(a)
	}
	return strings.ToLower(b)
}

func pathClassFromPath(p AttackPath) string {
	if p.Explainability != nil && p.Explainability.Class != "" {
		switch p.Explainability.Class {
		case PathClassEscape:
			return "ESCAPE"
		case PathClassLateral:
			return "LATERAL"
		case PathClassPrivEsc:
			return "PRIV_ESC"
		case PathClassDataExfil:
			return "DATA_EXFIL"
		}
	}
	desc := strings.ToUpper(p.Description)
	switch {
	case strings.Contains(desc, "ESCAPE_PATH"):
		return "ESCAPE"
	case strings.Contains(desc, "LATERAL_PATH"):
		return "LATERAL"
	case strings.Contains(desc, "PRIV_ESC_PATH"):
		return "PRIV_ESC"
	default:
		return "DATA_EXFIL"
	}
}

func saFromPath(p AttackPath) string {
	for _, n := range p.Nodes {
		if strings.EqualFold(n.Type, NodeTypeServiceAccount) {
			return n.ID
		}
	}
	return ""
}

func uniqStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		k := strings.TrimSpace(v)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func hasExact(values []string, needle string) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}

func hasAnyPrefix(values []string, prefixes ...string) bool {
	for _, v := range values {
		for _, p := range prefixes {
			if strings.HasPrefix(v, p) {
				return true
			}
		}
	}
	return false
}

func privilegeLevelFromID(id string) int {
	role := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(id), "role:"))
	switch {
	case strings.Contains(role, "cluster-admin"):
		return 5
	case strings.Contains(role, "admin"):
		return 4
	case strings.Contains(role, "edit"):
		return 3
	case strings.Contains(role, "view"), strings.Contains(role, "read"):
		return 2
	default:
		return 1
	}
}

func privilegeLevelFromPath(p pathNormalized) int {
	return privilegeLevelFromID(p.Target.ID)
}

// validateChainCompleteness downgrades confidence when a chain skips
// mandatory intermediate steps (e.g. NODE → RBAC without token harvest).
func validateChainCompleteness(a, b pathNormalized, ruleType string) string {
	if ruleType != "ESCAPE_TO_PRIV_ESC" && ruleType != "NODE_DOMINANCE" {
		return ""
	}
	if !hasExact(a.Provides, "NODE_ACCESS") {
		return ""
	}
	hasTokenStep := false
	for _, n := range a.Raw.Nodes {
		if strings.EqualFold(n.Type, NodeTypeServiceAccount) {
			hasTokenStep = true
			break
		}
	}
	if !hasTokenStep {
		for _, n := range b.Raw.Nodes {
			if strings.EqualFold(n.Type, NodeTypeServiceAccount) {
				hasTokenStep = true
				break
			}
		}
	}
	if !hasTokenStep {
		return "low"
	}
	return ""
}

func enforceObjectiveLimit(chains []AttackChain, maxPerTarget int) []AttackChain {
	if maxPerTarget <= 0 || len(chains) == 0 {
		return chains
	}
	count := map[string]int{}
	out := make([]AttackChain, 0, len(chains))
	for _, ch := range chains {
		key := ch.Objective + "|" + ch.FinalTarget
		if count[key] >= maxPerTarget {
			continue
		}
		count[key]++
		out = append(out, ch)
	}
	return out
}
