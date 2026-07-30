package graph

import (
	"strings"
	"testing"
)

func TestDetectChains_EscapeToPrivEsc(t *testing.T) {
	paths := []AttackPath{
		{
			Nodes: []PathNode{
				{ID: "pod-a", Type: NodeTypePod},
				{ID: "node-x", Type: NodeTypeNode},
			},
			Edges:     []PathEdge{{Type: EdgeTypeContainerEscape, Source: "pod-a", Target: "node-x"}},
			TotalRisk: 8.5,
			Explainability: &PathExplainability{
				Class:       PathClassEscape,
				Strength:    0.82,
				Feasibility: PathFeasibility{Confidence: 0.9},
			},
		},
		{
			Nodes: []PathNode{
				{ID: "pod-b", Type: NodeTypePod},
				{ID: "role:cluster-admin", Type: NodeTypeClusterRole},
			},
			Edges:     []PathEdge{{Type: EdgeTypeGrantsRole, Source: "pod-b", Target: "role:cluster-admin"}},
			TotalRisk: 7.4,
			Explainability: &PathExplainability{
				Class:       PathClassPrivEsc,
				Strength:    0.74,
				Feasibility: PathFeasibility{Confidence: 0.8},
			},
		},
	}

	chains := DetectChains(paths)
	if len(chains) == 0 {
		t.Fatalf("expected at least one chain")
	}
	found := false
	for _, c := range chains {
		if c.Type == "ESCAPE_TO_PRIV_ESC" {
			found = true
			if c.Objective != "CLUSTER_TAKEOVER" {
				t.Fatalf("unexpected objective: %s", c.Objective)
			}
		}
	}
	if !found {
		t.Fatalf("expected ESCAPE_TO_PRIV_ESC chain")
	}
}

func TestDetectChains_EnrichesRepresentativeByChainPathsAfterSort(t *testing.T) {
	paths := []AttackPath{
		{
			PathID: "p0",
			Nodes: []PathNode{
				{ID: "broken-uid", Type: NodeTypePod, Properties: map[string]interface{}{"name": "broken-chain"}},
				{ID: "role:cluster-admin", Type: NodeTypeClusterRole, Properties: map[string]interface{}{"name": "cluster-admin"}},
			},
			Edges: []PathEdge{{Type: EdgeTypeNetworkReachSoft, Source: "broken-uid", Target: "role:cluster-admin"}},
			Explainability: &PathExplainability{
				Class:       PathClassPrivEsc,
				Strength:    0.10,
				Feasibility: PathFeasibility{NetworkDecision: "soft_allow", Confidence: 0.35},
			},
		},
		{
			PathID: "p1",
			Nodes: []PathNode{
				{ID: "lateral-uid", Type: NodeTypePod, Properties: map[string]interface{}{"name": "lateral-pod"}},
				{ID: "role:cluster-admin", Type: NodeTypeClusterRole, Properties: map[string]interface{}{"name": "cluster-admin"}},
			},
			Edges: []PathEdge{{Type: EdgeTypeNetworkReachSoft, Source: "lateral-uid", Target: "role:cluster-admin"}},
			Explainability: &PathExplainability{
				Class:       PathClassPrivEsc,
				Strength:    0.10,
				Feasibility: PathFeasibility{NetworkDecision: "soft_allow", Confidence: 0.35},
			},
		},
		{
			PathID: "p3",
			Nodes: []PathNode{
				{ID: "rbac-uid", Type: NodeTypePod, Properties: map[string]interface{}{"name": "rbac-pod"}},
				{ID: "sa-rbac", Type: NodeTypeServiceAccount, Properties: map[string]interface{}{"name": "sa-rbac"}},
				{ID: "binding:/crb-rbac-admin", Type: NodeTypeClusterBinding, Properties: map[string]interface{}{"name": "crb-rbac-admin"}},
				{ID: "role:cluster-admin", Type: NodeTypeClusterRole, Properties: map[string]interface{}{"name": "cluster-admin"}},
			},
			Edges: []PathEdge{
				{Type: EdgeTypeServiceAccount, Source: "rbac-uid", Target: "sa-rbac"},
				{Type: EdgeTypeRbacBinding, Source: "sa-rbac", Target: "binding:/crb-rbac-admin"},
				{Type: EdgeTypeGrantsRole, Source: "binding:/crb-rbac-admin", Target: "role:cluster-admin"},
			},
			Explainability: &PathExplainability{
				Class:       PathClassPrivEsc,
				Strength:    0.31,
				Feasibility: PathFeasibility{NetworkDecision: "n/a", Confidence: 0.9},
			},
		},
		{
			PathID: "p7",
			Nodes: []PathNode{
				{ID: "kube-proxy-uid", Type: NodeTypePod, Properties: map[string]interface{}{"name": "kube-proxy-gpt6h"}},
				{ID: "cap:kube-proxy-uid:ESC_HOSTPATH_NODE", Type: NodeTypeCapability, Properties: map[string]interface{}{"name": "ESC_HOSTPATH_NODE"}},
				{ID: "node:k8s-master", Type: NodeTypeNode, Properties: map[string]interface{}{"name": "k8s-master"}},
			},
			Edges: []PathEdge{
				{Type: EdgeTypeHostAccess, Source: "kube-proxy-uid", Target: "cap:kube-proxy-uid:ESC_HOSTPATH_NODE"},
				{Type: EdgeTypeLateralMove, Source: "cap:kube-proxy-uid:ESC_HOSTPATH_NODE", Target: "node:k8s-master"},
			},
			Explainability: &PathExplainability{
				Class:       PathClassEscape,
				Strength:    0.40,
				Feasibility: PathFeasibility{NetworkDecision: "n/a", Confidence: 0.9},
			},
		},
	}

	chains := DetectChainsWithHints(paths, DefaultHardeningHints())
	var target *AttackChain
	for i := range chains {
		if chains[i].Type == "ESCAPE_TO_PRIV_ESC" && chains[i].SourceID == "kube-proxy-uid" {
			target = &chains[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("expected kube-proxy escape-to-priv-esc chain, got %#v", chains)
	}
	if target.SourceID != "kube-proxy-uid" {
		t.Fatalf("SourceID = %q, want kube-proxy-uid", target.SourceID)
	}
	if target.TargetID != "role:cluster-admin" {
		t.Fatalf("TargetID = %q, want role:cluster-admin", target.TargetID)
	}
	if target.Story.Narrative == "" || !containsAll(target.Story.Narrative, "kube-proxy-gpt6h", "k8s-master") {
		t.Fatalf("story narrative does not describe selected paths: %q", target.Story.Narrative)
	}
	for _, ev := range target.Evidence {
		if ev.SourceRef == "pod/broken-chain" || ev.SourceRef == "pod/lateral-pod" {
			t.Fatalf("evidence leaked from unrelated chain pair: %#v", target.Evidence)
		}
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

func TestDetectChains_LateralToPrivEsc(t *testing.T) {
	paths := []AttackPath{
		{
			Nodes: []PathNode{
				{ID: "pod-a", Type: NodeTypePod},
				{ID: "pod-b", Type: NodeTypePod},
			},
			Edges:     []PathEdge{{Type: EdgeTypeNetworkReach, Source: "pod-a", Target: "pod-b"}},
			TotalRisk: 6.0,
			Explainability: &PathExplainability{
				Class:       PathClassLateral,
				Strength:    0.6,
				Feasibility: PathFeasibility{Confidence: 0.85},
			},
		},
		{
			Nodes: []PathNode{
				{ID: "pod-b", Type: NodeTypePod},
				{ID: "role:risky", Type: NodeTypeRole},
			},
			Edges:     []PathEdge{{Type: EdgeTypeGrantsRole, Source: "pod-b", Target: "role:risky"}},
			TotalRisk: 5.5,
			Explainability: &PathExplainability{
				Class:       PathClassPrivEsc,
				Strength:    0.55,
				Feasibility: PathFeasibility{Confidence: 0.7},
			},
		},
	}

	chains := DetectChains(paths)
	found := false
	for _, c := range chains {
		if c.Type == "LATERAL_TO_PRIV_ESC" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected LATERAL_TO_PRIV_ESC chain")
	}
}

func TestDetectChains_NetworkBridge(t *testing.T) {
	paths := []AttackPath{
		{
			Nodes: []PathNode{
				{ID: "pod-a", Type: NodeTypePod},
				{ID: "pod-b", Type: NodeTypePod},
			},
			Edges:     []PathEdge{{Type: EdgeTypeNetworkReach, Source: "pod-a", Target: "pod-b"}},
			TotalRisk: 5.0,
			Explainability: &PathExplainability{
				Class:       PathClassDataExfil,
				Strength:    0.5,
				Feasibility: PathFeasibility{Confidence: 0.7},
			},
		},
		{
			Nodes: []PathNode{
				{ID: "pod-b", Type: NodeTypePod},
				{ID: "role:risky", Type: NodeTypeRole},
			},
			Edges:     []PathEdge{{Type: EdgeTypeGrantsRole, Source: "pod-b", Target: "role:risky"}},
			TotalRisk: 5.5,
			Explainability: &PathExplainability{
				Class:       PathClassPrivEsc,
				Strength:    0.55,
				Feasibility: PathFeasibility{Confidence: 0.7},
			},
		},
	}

	chains := DetectChains(paths)
	found := false
	for _, c := range chains {
		if c.Type == "NETWORK_BRIDGE" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected NETWORK_BRIDGE chain")
	}
}

func TestDetectChains_PrivEscLadder(t *testing.T) {
	paths := []AttackPath{
		{
			Nodes: []PathNode{
				{ID: "pod-a", Type: NodeTypePod},
				{ID: "role:edit", Type: NodeTypeRole},
			},
			Edges: []PathEdge{{Type: EdgeTypeGrantsRole, Source: "pod-a", Target: "role:edit"}},
			Explainability: &PathExplainability{
				Class:       PathClassPrivEsc,
				Strength:    0.62,
				Feasibility: PathFeasibility{Confidence: 0.9},
			},
		},
		{
			Nodes: []PathNode{
				{ID: "pod-a", Type: NodeTypePod},
				{ID: "role:cluster-admin", Type: NodeTypeClusterRole},
			},
			Edges: []PathEdge{{Type: EdgeTypeGrantsRole, Source: "pod-a", Target: "role:cluster-admin"}},
			Explainability: &PathExplainability{
				Class:       PathClassPrivEsc,
				Strength:    0.57,
				Feasibility: PathFeasibility{Confidence: 0.85},
			},
		},
	}

	chains := DetectChains(paths)
	found := false
	for _, c := range chains {
		if c.Type == "PRIV_ESC_LADDER" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected PRIV_ESC_LADDER chain")
	}
}

// ─── classifyObjectiveByRole ──────────────────────────────────────────────────

func TestClassifyObjectiveByRole_Priority(t *testing.T) {
	cases := []struct {
		roleID string
		want   string
	}{
		// admin beats secret/storage/deploy in compound names
		{"role:secret-storage-admin", "CLUSTER_PRIVILEGE_ESCALATION"},
		{"role:deploy-admin", "CLUSTER_PRIVILEGE_ESCALATION"},
		{"role:daemonset-admin", "CLUSTER_PRIVILEGE_ESCALATION"},
		// non-admin buckets
		{"role:secret-reader", "SECRET_EXFIL"},
		{"role:pv-manager", "STORAGE_ABUSE"},
		{"role:volume-backup", "STORAGE_ABUSE"},
		{"role:deploy-operator", "WORKLOAD_CONTROL"},
		{"role:workload-creator", "WORKLOAD_CONTROL"},
		// fallback
		{"role:monitoring-scraper", "LIMITED_RBAC_IMPACT"},
		// no role: prefix
		{"cluster-admin", "CLUSTER_PRIVILEGE_ESCALATION"},
	}
	for _, tc := range cases {
		got := classifyObjectiveByRole(tc.roleID)
		if got != tc.want {
			t.Errorf("classifyObjectiveByRole(%q) = %q, want %q", tc.roleID, got, tc.want)
		}
	}
}

// ─── validateChainCompleteness ────────────────────────────────────────────────

func TestValidateChainCompleteness_DowngradesWhenNoSAStep(t *testing.T) {
	a := pathNormalized{
		PathID:   "p0",
		Class:    "ESCAPE",
		Provides: []string{"NODE_ACCESS", "SA_TOKEN", "SA_TOKEN:*", "KUBELET_ACCESS"},
		Raw: AttackPath{
			Nodes: []PathNode{
				{ID: "pod-a", Type: NodeTypePod},
				{ID: "node-x", Type: NodeTypeNode},
			},
		},
	}
	b := pathNormalized{
		PathID: "p1",
		Class:  "PRIV_ESC",
		Target: pathEndpoint{Type: "CLUSTERROLE", ID: "role:cluster-admin"},
		Raw: AttackPath{
			Nodes: []PathNode{
				{ID: "pod-b", Type: NodeTypePod},
				{ID: "role:cluster-admin", Type: NodeTypeClusterRole},
			},
		},
	}
	got := validateChainCompleteness(a, b, "ESCAPE_TO_PRIV_ESC")
	if got != "low" {
		t.Errorf("expected 'low' confidence when no SA token step, got %q", got)
	}
}

func TestValidateChainCompleteness_NoOverrideWhenSAPresent(t *testing.T) {
	a := pathNormalized{
		PathID:   "p0",
		Class:    "ESCAPE",
		Provides: []string{"NODE_ACCESS", "SA_TOKEN"},
		Raw: AttackPath{
			Nodes: []PathNode{
				{ID: "pod-a", Type: NodeTypePod},
				{ID: "sa-default", Type: NodeTypeServiceAccount},
				{ID: "node-x", Type: NodeTypeNode},
			},
		},
	}
	b := pathNormalized{
		PathID: "p1",
		Raw:    AttackPath{Nodes: []PathNode{{ID: "pod-b", Type: NodeTypePod}}},
	}
	got := validateChainCompleteness(a, b, "ESCAPE_TO_PRIV_ESC")
	if got != "" {
		t.Errorf("expected no override when SA step present, got %q", got)
	}
}

func TestValidateChainCompleteness_NonEscapeRulesSkipped(t *testing.T) {
	a := pathNormalized{Provides: []string{"NODE_ACCESS"}}
	b := pathNormalized{}
	got := validateChainCompleteness(a, b, "LATERAL_TO_PRIV_ESC")
	if got != "" {
		t.Errorf("expected no override for non-escape rule, got %q", got)
	}
}

// ─── SA_TOKEN_REUSE guard ─────────────────────────────────────────────────────

// Node-derived SA_TOKEN:* must NOT allow chaining to a different SA's path.
func TestChainRuleMatch_SATokenReuse_NodeWildcardRequiresSameSA(t *testing.T) {
	a := pathNormalized{
		PathID:   "p0",
		Class:    "ESCAPE",
		Source:   pathEndpoint{SA: "sa-evil"},
		Target:   pathEndpoint{ID: "node-x", Type: "NODE"},
		Provides: []string{"NODE_ACCESS", "SA_TOKEN", "SA_TOKEN:*", "KUBELET_ACCESS"},
	}
	b := pathNormalized{
		PathID:   "p1",
		Class:    "PRIV_ESC",
		Source:   pathEndpoint{ID: "pod-victim", SA: "sa-victim"},
		Target:   pathEndpoint{ID: "role:cluster-admin", Type: "CLUSTERROLE"},
		Requires: []string{"SA_TOKEN", "ROLE"},
	}
	ruleType, _, ok := chainRuleMatch(a, b)
	if ok && ruleType == "SA_TOKEN_REUSE" {
		t.Errorf("SA_TOKEN_REUSE must NOT fire when wildcard is only node-derived and SAs differ")
	}
}

func TestChainRuleMatch_SATokenReuse_SameSAWithNodeAccess(t *testing.T) {
	// Use DATA_EXFIL class so C1 (ESCAPE_TO_PRIV_ESC) does not match first.
	a := pathNormalized{
		PathID:   "p0",
		Class:    "DATA_EXFIL",
		Source:   pathEndpoint{SA: "sa-shared"},
		Target:   pathEndpoint{ID: "node-x", Type: NodeTypeNode},
		Provides: []string{"NODE_ACCESS", "SA_TOKEN", "SA_TOKEN:*", "KUBELET_ACCESS"},
	}
	b := pathNormalized{
		PathID:   "p1",
		Class:    "PRIV_ESC",
		Source:   pathEndpoint{ID: "pod-b", SA: "sa-shared"},
		Target:   pathEndpoint{ID: "role:cluster-admin", Type: NodeTypeClusterRole},
		Requires: []string{"SA_TOKEN", "ROLE"},
	}
	ruleType, _, ok := chainRuleMatch(a, b)
	if !ok || ruleType != "SA_TOKEN_REUSE" {
		t.Errorf("SA_TOKEN_REUSE should fire for same SA with node access; got ok=%v rule=%q", ok, ruleType)
	}
}

// ─── objectiveFromChainContext ────────────────────────────────────────────────

func TestObjectiveFromChainContext_ClusterTakeover(t *testing.T) {
	// NodeTypeClusterRole = "cluster_role"; strings.ToUpper → "CLUSTER_ROLE"
	b := pathNormalized{Target: pathEndpoint{Type: NodeTypeClusterRole, ID: "role:cluster-admin"}}
	got := objectiveFromChainContext(b)
	if got != "CLUSTER_TAKEOVER" {
		t.Errorf("want CLUSTER_TAKEOVER for cluster-admin, got %q", got)
	}
}

func TestObjectiveFromChainContext_NodeCompromise(t *testing.T) {
	// NodeTypeNode = "node"
	b := pathNormalized{Target: pathEndpoint{Type: NodeTypeNode, ID: "node-1"}}
	got := objectiveFromChainContext(b)
	if got != "NODE_COMPROMISE" {
		t.Errorf("want NODE_COMPROMISE, got %q", got)
	}
}

func TestObjectiveFromChainContext_LowPrivClusterRole(t *testing.T) {
	// ClusterRole with privilege level < 4 → classifyObjectiveByRole
	b := pathNormalized{Target: pathEndpoint{Type: NodeTypeClusterRole, ID: "role:secret-reader"}}
	got := objectiveFromChainContext(b)
	if got != "SECRET_EXFIL" {
		t.Errorf("want SECRET_EXFIL for low-priv secret-reader clusterrole, got %q", got)
	}
}

// ─── Phase 2: Exploit Cost (spec §3.7) ────────────────────────────────────────

func TestComputeExploitCost_EscapeToPrivEsc(t *testing.T) {
	cost := computeExploitCost("ESCAPE_TO_PRIV_ESC", false)
	// escape(3) + node(2) + harvest(3) + RBAC(1) = 9
	if cost != 9 {
		t.Errorf("ESCAPE_TO_PRIV_ESC without explicit token: want 9, got %d", cost)
	}
}

func TestComputeExploitCost_EscapeToPrivEsc_ExplicitToken(t *testing.T) {
	cost := computeExploitCost("ESCAPE_TO_PRIV_ESC", true)
	// 9 - 1 discount = 8
	if cost != 8 {
		t.Errorf("ESCAPE_TO_PRIV_ESC with explicit token: want 8, got %d", cost)
	}
}

func TestComputeExploitCost_PrivEscLadder_Cheapest(t *testing.T) {
	cost := computeExploitCost("PRIV_ESC_LADDER", false)
	// RBAC × 2 = 2
	if cost != 2 {
		t.Errorf("PRIV_ESC_LADDER: want 2 (cheapest), got %d", cost)
	}
}

func TestComputeExploitCost_SATokenReuse(t *testing.T) {
	cost := computeExploitCost("SA_TOKEN_REUSE", false)
	// harvest(3) + RBAC(1) = 4
	if cost != 4 {
		t.Errorf("SA_TOKEN_REUSE: want 4, got %d", cost)
	}
}

// ─── Phase 2: Blast Radius (spec §3.8) ────────────────────────────────────────

func TestComputeBlastRadius_NoVariants(t *testing.T) {
	r := computeBlastRadius(nil, 10)
	if r != 0 {
		t.Errorf("no variant nodes: want 0, got %f", r)
	}
}

func TestComputeBlastRadius_TwoOfThree(t *testing.T) {
	r := computeBlastRadius([]string{"node-a", "node-b"}, 3)
	want := 2.0 / 3.0
	if abs(r-want) > 0.001 {
		t.Errorf("2/3 blast radius: want %.3f, got %.3f", want, r)
	}
}

func TestComputeBlastRadius_AllNodes(t *testing.T) {
	r := computeBlastRadius([]string{"node-a", "node-b", "node-c"}, 3)
	if r != 1.0 {
		t.Errorf("full blast radius: want 1.0, got %f", r)
	}
}

func TestComputeBlastRadius_ZeroTotalNodes(t *testing.T) {
	r := computeBlastRadius([]string{"node-a"}, 0)
	if r != 0 {
		t.Errorf("zero total nodes: want 0, got %f", r)
	}
}

// ─── Phase 2: Hard Gate Validation (FIX L) ────────────────────────────────────

func TestCheckHardGates_SATokenReuse_MissingToken(t *testing.T) {
	a := pathNormalized{Provides: []string{"NODE_ACCESS"}} // no SA_TOKEN
	b := pathNormalized{}
	valid, reason := checkHardGates(a, b, "SA_TOKEN_REUSE")
	if valid {
		t.Error("SA_TOKEN_REUSE without SA_TOKEN should be invalid (HARD gate)")
	}
	if reason == "" {
		t.Error("reason must be non-empty for HARD gate failure")
	}
}

func TestCheckHardGates_SATokenReuse_WithToken(t *testing.T) {
	a := pathNormalized{Provides: []string{"SA_TOKEN", "NODE_ACCESS"}}
	b := pathNormalized{}
	valid, _ := checkHardGates(a, b, "SA_TOKEN_REUSE")
	if !valid {
		t.Error("SA_TOKEN_REUSE with SA_TOKEN should pass hard gate")
	}
}

func TestCheckHardGates_EscapeToPrivEsc_MissingNodeAccess(t *testing.T) {
	a := pathNormalized{Provides: []string{"SA_TOKEN"}} // no node access
	b := pathNormalized{}
	valid, reason := checkHardGates(a, b, "ESCAPE_TO_PRIV_ESC")
	if valid {
		t.Error("ESCAPE_TO_PRIV_ESC without any node access should be invalid")
	}
	if reason == "" {
		t.Error("reason must be set for HARD gate failure")
	}
}

func TestCheckHardGates_EscapeToPrivEsc_WithShellAccess(t *testing.T) {
	a := pathNormalized{Provides: []string{"SA_TOKEN", NodeAccessShell}}
	b := pathNormalized{}
	valid, _ := checkHardGates(a, b, "ESCAPE_TO_PRIV_ESC")
	if !valid {
		t.Error("ESCAPE_TO_PRIV_ESC with NODE_SHELL_ACCESS should pass")
	}
}

// ─── Phase 2: Dependency-Aware Confidence (FIX H) ─────────────────────────────

func TestClassifyStepGroups_EscapeChain(t *testing.T) {
	a := pathNormalized{
		Class:      "ESCAPE",
		Confidence: "high",
		Provides:   []string{"NODE_ACCESS", NodeAccessShell, "SA_TOKEN", "SA_TOKEN:*"},
		Raw: AttackPath{
			Nodes: []PathNode{
				{Type: NodeTypePod, ID: "pod-a"},
				{Type: NodeTypeNode, ID: "node-x"},
			},
		},
	}
	b := pathNormalized{
		Class:      "PRIV_ESC",
		Confidence: "high",
		Provides:   []string{"ROLE"},
	}
	g0, g1, g2 := classifyStepGroups(a, b)

	// g0 = escape group, g2 = RBAC group
	if len(g0.confidences) == 0 {
		t.Error("g0 (escape group) must have at least one confidence")
	}
	// g1 = token group; implicit since no SA node in path but has NODE_ACCESS
	_ = g1
	if len(g2.confidences) == 0 {
		t.Error("g2 (exploit group) must have at least one confidence")
	}

	// Chain confidence = g0.min × g1.min × g2.min; should be < 0.9 × 0.9 = 0.81
	g0Conf := g0.confidence()
	g2Conf := g2.confidence()
	if g0Conf > 1.0 || g2Conf > 1.0 {
		t.Errorf("confidence values must be ≤ 1.0, got g0=%f g2=%f", g0Conf, g2Conf)
	}
}

func TestClassifyStepGroups_LateralChain(t *testing.T) {
	a := pathNormalized{Class: "LATERAL", Confidence: "medium"}
	b := pathNormalized{Class: "PRIV_ESC", Confidence: "high"}
	g0, g1, g2 := classifyStepGroups(a, b)
	_ = g1
	// For LATERAL, g0=step_a and g2=step_b; token group empty
	if g0.confidence() <= 0 || g2.confidence() <= 0 {
		t.Error("LATERAL groups must have positive confidences")
	}
}

// ─── Phase 2: Context-aware Realism (FIX I) ───────────────────────────────────

func TestContextRealismFactor_HardenedCluster(t *testing.T) {
	hints := ClusterHardeningHints{
		KubeletAuthEnabled:  true,
		ProjectedTokensOnly: true,
		PodSecurityLevel:    "restricted",
		AutomountDefault:    false,
	}
	factor := contextRealismFactor(hints, true, true)
	// KubeletAuth(×0.65) × Projected(×0.75) × Restricted(×0.50) × NoAutomount(×0.85)
	// = 0.65 × 0.75 × 0.50 × 0.85 ≈ 0.207 → floor to 0.40
	if factor > 0.50 {
		t.Errorf("hardened cluster should have low realism factor (≤0.50), got %f", factor)
	}
	if factor < 0.40 {
		t.Errorf("realism floor is 0.40, got %f", factor)
	}
}

func TestContextRealismFactor_DefaultCluster(t *testing.T) {
	hints := DefaultHardeningHints()
	factor := contextRealismFactor(hints, false, false)
	if factor != 1.0 {
		t.Errorf("default cluster with no node/token involvement: want 1.0, got %f", factor)
	}
}

func TestContextRealismFactor_NodeChain_NoHardening(t *testing.T) {
	hints := DefaultHardeningHints()
	factor := contextRealismFactor(hints, true, true)
	// No hardening signals → 1.0
	if factor != 1.0 {
		t.Errorf("no hardening signals: want 1.0, got %f", factor)
	}
}

// ─── Phase 2: Primary Selection (spec §3.9) ───────────────────────────────────

func TestSelectPrimaryChains_OnePrimaryPerObjective(t *testing.T) {
	chains := []AttackChain{
		{Objective: "CLUSTER_TAKEOVER", Confidence: "high", Realism: 0.80, ExploitCost: 9},
		{Objective: "CLUSTER_TAKEOVER", Confidence: "medium", Realism: 0.60, ExploitCost: 5},
		{Objective: "SECRET_EXFIL", Confidence: "medium", Realism: 0.70, ExploitCost: 4},
		{Objective: "SECRET_EXFIL", Confidence: "low", Realism: 0.40, ExploitCost: 4},
	}
	result := selectPrimaryChains(chains)

	primaryCount := map[string]int{}
	for _, ch := range result {
		if ch.Primary {
			primaryCount[ch.Objective]++
		}
	}
	for obj, count := range primaryCount {
		if count != 1 {
			t.Errorf("objective %s: want exactly 1 primary, got %d", obj, count)
		}
	}
}

func TestSelectPrimaryChains_HighRealismWins(t *testing.T) {
	chains := []AttackChain{
		// Both CLUSTER_TAKEOVER; second has lower cost + higher realism → should win
		{ChainID: "chain-a", Objective: "CLUSTER_TAKEOVER", Confidence: "medium", Realism: 0.50, ExploitCost: 9},
		{ChainID: "chain-b", Objective: "CLUSTER_TAKEOVER", Confidence: "medium", Realism: 0.85, ExploitCost: 3},
	}
	result := selectPrimaryChains(chains)
	for _, ch := range result {
		if ch.ChainID == "chain-b" && !ch.Primary {
			t.Error("chain-b (lower cost + higher realism) should be primary")
		}
		if ch.ChainID == "chain-a" && ch.Primary {
			t.Error("chain-a (higher cost + lower realism) should NOT be primary")
		}
	}
}

// ─── Phase 2: Story Engine (spec §7) ──────────────────────────────────────────

func TestGenerateStory_Headline_EscapeToPrivEsc(t *testing.T) {
	ch := AttackChain{
		Type:        "ESCAPE_TO_PRIV_ESC",
		Objective:   "CLUSTER_TAKEOVER",
		FinalTarget: "cluster_role:role:cluster-admin",
		ExploitCost: 9,
		Realism:     0.75,
	}
	story := GenerateStory(ch, nil, 3)
	if !contains(story.Headline, "Container escape") {
		t.Errorf("ESCAPE_TO_PRIV_ESC headline should mention container escape, got: %q", story.Headline)
	}
	if !contains(story.Headline, "🔥") {
		t.Errorf("CLUSTER_TAKEOVER headline should have 🔥 emoji, got: %q", story.Headline)
	}
}

func TestGenerateStory_Narrative_MentionsSource(t *testing.T) {
	ch := AttackChain{
		Type:        "SA_TOKEN_REUSE",
		Objective:   "SECRET_EXFIL",
		FinalTarget: "role:role:secret-reader",
		ExploitCost: 4,
		Realism:     0.80,
	}
	norm := []pathNormalized{
		{
			PathID: "p0",
			Raw: AttackPath{
				Nodes: []PathNode{
					{Type: NodeTypePod, ID: "pod-abc", Properties: map[string]interface{}{"name": "kube-proxy-xyz"}},
				},
			},
		},
	}
	ch.Paths = []string{"p0"}
	story := GenerateStory(ch, norm, 3)
	if !contains(story.Narrative, "kube-proxy-xyz") {
		t.Errorf("narrative should mention source pod name, got: %q", story.Narrative)
	}
}

func TestGenerateStory_ImpactText_ClusterTakeover(t *testing.T) {
	ch := AttackChain{Objective: "CLUSTER_TAKEOVER", Type: "ESCAPE_TO_PRIV_ESC", Realism: 0.7, ExploitCost: 9}
	story := GenerateStory(ch, nil, 3)
	if story.ImpactText == "" {
		t.Error("impact text should not be empty for CLUSTER_TAKEOVER")
	}
	if !contains(story.ImpactText, "control") && !contains(story.ImpactText, "workload") {
		t.Errorf("CLUSTER_TAKEOVER impact text should mention control/workloads, got: %q", story.ImpactText)
	}
}

func TestGenerateStory_ExploitText_HardChain(t *testing.T) {
	ch := AttackChain{Type: "ESCAPE_TO_PRIV_ESC", Objective: "CLUSTER_TAKEOVER", ExploitCost: 9, Realism: 0.6}
	story := GenerateStory(ch, nil, 3)
	if story.ExploitText == "" {
		t.Error("exploit text must not be empty")
	}
	if !contains(story.ExploitText, "escape") {
		t.Errorf("exploit text for ESCAPE chain should mention escape, got: %q", story.ExploitText)
	}
}

func TestGenerateStory_Steps_NonEmpty(t *testing.T) {
	ch := AttackChain{Type: "ESCAPE_TO_PRIV_ESC", Objective: "CLUSTER_TAKEOVER", ExploitCost: 9, Realism: 0.7,
		FinalTarget: "cluster_role:role:cluster-admin"}
	story := GenerateStory(ch, nil, 3)
	if len(story.Steps) == 0 {
		t.Error("Steps must be non-empty for ESCAPE_TO_PRIV_ESC chain")
	}
}

func TestGenerateStory_BlastRadius_VariantMentioned(t *testing.T) {
	ch := AttackChain{
		Type:         "ESCAPE_TO_PRIV_ESC",
		Objective:    "CLUSTER_TAKEOVER",
		FinalTarget:  "cluster_role:role:cluster-admin",
		ExploitCost:  9,
		Realism:      0.75,
		VariantNodes: []string{"node-a", "node-b", "node-c"},
	}
	story := GenerateStory(ch, nil, 3)
	if !contains(story.Narrative, "2 other node") {
		t.Errorf("narrative should mention variant nodes (2 others), got: %q", story.Narrative)
	}
}

// ─── Phase 2: objectiveFromSemanticCaps priority ──────────────────────────────

func TestObjectiveFromSemanticCaps_IdentityForge_Wins(t *testing.T) {
	// IDENTITY_FORGE should take priority over DATA_ACCESS
	caps := []string{"DATA_ACCESS:secrets", "IDENTITY_FORGE", "WORKLOAD_CONTROL"}
	got := objectiveFromSemanticCaps(caps)
	if got != "CLUSTER_TAKEOVER" {
		t.Errorf("IDENTITY_FORGE should map to CLUSTER_TAKEOVER, got %q", got)
	}
}

func TestObjectiveFromSemanticCaps_SecretRead_Fallback(t *testing.T) {
	caps := []string{"DATA_ACCESS:secrets"}
	got := objectiveFromSemanticCaps(caps)
	if got != "SECRET_EXFIL" {
		t.Errorf("DATA_ACCESS:secrets should map to SECRET_EXFIL, got %q", got)
	}
}

func TestObjectiveFromSemanticCaps_Empty_ReturnsEmpty(t *testing.T) {
	got := objectiveFromSemanticCaps(nil)
	if got != "" {
		t.Errorf("empty caps should return empty string, got %q", got)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
