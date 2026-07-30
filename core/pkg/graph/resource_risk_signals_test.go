package graph

import "testing"

func TestComputeEffectiveRisk_noPathLowScore(t *testing.T) {
	s := ResourceRiskSignals{HasAttackPath: false}
	if got := ComputeEffectiveRisk(s, 9); got != "LOW" {
		t.Fatalf("got %q want LOW", got)
	}
}

func TestComputeEffectiveRisk_criticalPathLowScore(t *testing.T) {
	s := ResourceRiskSignals{HasAttackPath: true, MaxImpact: "CRITICAL"}
	if got := ComputeEffectiveRisk(s, 9); got != "LOW" {
		t.Fatalf("got %q want LOW", got)
	}
}

func TestComputeEffectiveRisk_pathNonCriticalNoInflation(t *testing.T) {
	s := ResourceRiskSignals{HasAttackPath: true, MaxImpact: "MEDIUM"}
	if got := ComputeEffectiveRisk(s, 9); got != "LOW" {
		t.Fatalf("got %q want LOW (score band only)", got)
	}
	if got := ComputeEffectiveRisk(s, 45); got != "HIGH" {
		t.Fatalf("got %q want HIGH", got)
	}
}

func TestComputeEffectiveRisk_highPathLowScore_noPathUplift(t *testing.T) {
	s := ResourceRiskSignals{HasAttackPath: true, MaxImpact: "HIGH"}
	if got := ComputeEffectiveRisk(s, 9); got != "LOW" {
		t.Fatalf("got %q want LOW", got)
	}
	if got := ComputeEffectiveRisk(s, 45); got != "HIGH" {
		t.Fatalf("got %q want HIGH", got)
	}
}

func TestBuildRiskSummary_critical(t *testing.T) {
	s := ResourceRiskSignals{HasAttackPath: true, MaxImpact: "CRITICAL"}
	if got := BuildRiskSummary(s); got != "Can be used to take over the cluster" {
		t.Fatalf("got %q", got)
	}
}

func TestChainsTouchingResource_involvedOnly(t *testing.T) {
	ch := AttackChain{
		ChainID: "c1", SourceID: "pod-a", TargetID: "role-x",
		InvolvedResources: []string{"pod-a", "sa-1"},
	}
	got := ChainsTouchingResource("pod-a", []AttackChain{ch})
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	if len(ChainsTouchingResource("pod-z", []AttackChain{ch})) != 0 {
		t.Fatal("should not touch")
	}
}

func TestChainsTouchingResource_uniqueByChainID(t *testing.T) {
	ch := AttackChain{
		ChainID: "c1", ID: "c1", InvolvedResources: []string{"pod-a"},
	}
	dup := ch
	got := ChainsTouchingResource("pod-a", []AttackChain{ch, dup})
	if len(got) != 1 {
		t.Fatalf("want unique chains, got %d", len(got))
	}
}

func TestBuildResourceRiskSignals_entryVsPivot(t *testing.T) {
	pathByID := map[string]AttackPath{
		"p0": {PathID: "p0", Nodes: []PathNode{{ID: "pod-a"}, {ID: "node-1"}, {ID: "mid"}}},
		"p1": {PathID: "p1", Nodes: []PathNode{{ID: "node-1"}, {ID: "role-z"}}},
	}
	ch := AttackChain{
		ChainID: "chain:0:1", ID: "chain:0:1", SourceID: "pod-a", TargetID: "role-z",
		Impact: "CRITICAL", InvolvedResources: []string{"pod-a", "node-1", "role-z"},
		Paths: []string{"p0", "p1"},
	}
	s := BuildResourceRiskSignals("pod-a", 9, []AttackChain{ch}, pathByID, nil)
	if !s.IsEntryPoint || s.IsPivot {
		t.Fatalf("entry only: %+v", s)
	}
	if len(s.ChainIDs) != 1 || s.ChainIDs[0] != "chain:0:1" {
		t.Fatalf("chain ids: %+v", s.ChainIDs)
	}
	if s.PathCount != 2 {
		t.Fatalf("path_count: %d", s.PathCount)
	}
	s2 := BuildResourceRiskSignals("node-1", 50, []AttackChain{ch}, pathByID, nil)
	if s2.IsEntryPoint || !s2.IsPivot {
		t.Fatalf("pivot: %+v", s2)
	}
	sTarget := BuildResourceRiskSignals("role-z", 50, []AttackChain{ch}, pathByID, nil)
	if sTarget.IsPivot {
		t.Fatalf("target must not be pivot: %+v", sTarget)
	}
}

func TestBuildResourceRiskSignals_falsePivot_singlePath(t *testing.T) {
	pathByID := map[string]AttackPath{
		"p0": {PathID: "p0", Nodes: []PathNode{{ID: "pod-a"}, {ID: "node-1"}, {ID: "role-z"}}},
	}
	ch := AttackChain{
		ChainID: "c1", ID: "c1", SourceID: "pod-a", TargetID: "role-z",
		Impact: "HIGH", InvolvedResources: []string{"pod-a", "node-1", "role-z"},
		Paths: []string{"p0"},
	}
	s := BuildResourceRiskSignals("node-1", 50, []AttackChain{ch}, pathByID, nil)
	if s.IsPivot {
		t.Fatalf("one path only → not pivot: %+v", s)
	}
}

func TestBuildResourceRiskSignals_multiChainConfidence_prefersHighOverWeakCritical(t *testing.T) {
	chCrit := AttackChain{
		ChainID: "c-weak", ID: "c-weak", SourceID: "pod-x", TargetID: "r1",
		Impact: "CRITICAL", InvolvedResources: []string{"pod-x", "sa-1"},
		CapabilityValidation: &CapabilityValidationResult{Confidence: 0.2},
	}
	chHigh := AttackChain{
		ChainID: "c-strong", ID: "c-strong", SourceID: "pod-x", TargetID: "r2",
		Impact: "HIGH", InvolvedResources: []string{"pod-x", "sa-2"},
		CapabilityValidation: &CapabilityValidationResult{Confidence: 0.9},
	}
	s := BuildResourceRiskSignals("pod-x", 10, []AttackChain{chCrit, chHigh}, nil, nil)
	if s.MaxImpact != "HIGH" {
		t.Fatalf("MaxImpact want HIGH got %q", s.MaxImpact)
	}
	if s.MaxImpactSourceChainID != "c-strong" {
		t.Fatalf("max_impact_source_chain_id: %q", s.MaxImpactSourceChainID)
	}
}

func TestBuildResourceRiskSignals_chainIDs_sortedByImpactDesc(t *testing.T) {
	chMed := AttackChain{
		ChainID: "m", ID: "m", SourceID: "p", TargetID: "t1",
		Impact: "MEDIUM", InvolvedResources: []string{"p"},
		CapabilityValidation: &CapabilityValidationResult{Confidence: 0.99},
	}
	chHi := AttackChain{
		ChainID: "h", ID: "h", SourceID: "p", TargetID: "t2",
		Impact: "HIGH", InvolvedResources: []string{"p"},
		CapabilityValidation: &CapabilityValidationResult{Confidence: 0.5},
	}
	s := BuildResourceRiskSignals("p", 50, []AttackChain{chMed, chHi}, nil, nil)
	if len(s.ChainIDs) != 2 || s.ChainIDs[0] != "h" || s.ChainIDs[1] != "m" {
		t.Fatalf("chain order: %v", s.ChainIDs)
	}
}

func TestBuildResourceRiskSignals_mixedImpactMediumPathLowScore_effectiveLow(t *testing.T) {
	ch := AttackChain{
		ChainID: "c1", ID: "c1", SourceID: "pod-a", TargetID: "r",
		Impact: "MEDIUM", InvolvedResources: []string{"pod-a"},
		CapabilityValidation: &CapabilityValidationResult{Confidence: 0.8},
	}
	s := BuildResourceRiskSignals("pod-a", 9, []AttackChain{ch}, nil, nil)
	if s.EffectiveRisk != "LOW" {
		t.Fatalf("effective: %+v", s)
	}
}

func TestBuildResourceRiskSignals_mixedImpactHighPathLowScore_effectiveLow(t *testing.T) {
	ch := AttackChain{
		ChainID: "c1", ID: "c1", SourceID: "pod-a", TargetID: "r",
		Impact: "HIGH", InvolvedResources: []string{"pod-a"},
		CapabilityValidation: &CapabilityValidationResult{Confidence: 0.8},
	}
	s := BuildResourceRiskSignals("pod-a", 9, []AttackChain{ch}, nil, nil)
	if s.EffectiveRisk != "LOW" {
		t.Fatalf("effective: %+v", s)
	}
}

func TestBuildResourceRiskSignals_rbacLikeClusterAdmin(t *testing.T) {
	podUID := "fixture-rbac-pod-1"
	ch := AttackChain{
		ChainID: "chain-rbac-1", ID: "chain-rbac-1",
		SourceID: podUID, TargetID: "role:cluster-admin",
		Type: "PRIV_ESC_LADDER", Impact: "CRITICAL",
		InvolvedResources:    []string{podUID, "fixture-rbac-sa-1", "fixture-rbac-crb", "role:cluster-admin"},
		CapabilityValidation: &CapabilityValidationResult{Confidence: 0.8},
		Paths:                []string{"p0", "p1"},
	}
	s := BuildResourceRiskSignals(podUID, 6.5, []AttackChain{ch}, nil, nil)
	if !s.HasAttackPath || s.MaxImpact != "CRITICAL" || s.EffectiveRisk != "LOW" {
		t.Fatalf("rbac-like: %+v", s)
	}
	if s.Summary != "Can be used to take over the cluster" {
		t.Fatalf("summary: %q", s.Summary)
	}
	if s.MaxImpactSourceChainID != "chain-rbac-1" {
		t.Fatalf("source chain: %q", s.MaxImpactSourceChainID)
	}
}

func TestBuildResourceRiskSignals_persistedOnly_escapePod(t *testing.T) {
	pod := "escape-pod-uid"
	persisted := []PersistedAttackPathSummary{
		{PathID: "p-esc-1", Description: "ESCAPE: workload → host", TotalRisk: 5},
	}
	s := BuildResourceRiskSignals(pod, 10, nil, nil, persisted)
	if !s.HasAttackPath || s.MaxImpact != "HIGH" {
		t.Fatalf("persisted escape: %+v", s)
	}
	if s.MaxImpactSourceChainID != "attack_path:p-esc-1" {
		t.Fatalf("source: %q", s.MaxImpactSourceChainID)
	}
	if s.PathCount != 1 {
		t.Fatalf("path_count: %d", s.PathCount)
	}
}

func TestBuildResourceRiskSignals_persistedBeatsWeakerChain(t *testing.T) {
	pod := "lateral-pod-uid"
	ch := AttackChain{
		ChainID: "c-low", ID: "c-low", SourceID: pod, TargetID: "x",
		Impact: "MEDIUM", InvolvedResources: []string{pod},
		CapabilityValidation: &CapabilityValidationResult{Confidence: 0.9},
		Paths:                []string{"p0"},
	}
	persisted := []PersistedAttackPathSummary{
		{PathID: "p-lat", Description: "LATERAL: sa → workload", TotalRisk: 6},
	}
	s := BuildResourceRiskSignals(pod, 20, []AttackChain{ch}, nil, persisted)
	if s.MaxImpact != "HIGH" {
		t.Fatalf("persisted lateral should win over MEDIUM chain: %+v", s)
	}
	if s.MaxImpactSourceChainID != "attack_path:p-lat" {
		t.Fatalf("source: %q", s.MaxImpactSourceChainID)
	}
}

func TestAttackPathSortPriority_order(t *testing.T) {
	a := ResourceRiskSignals{HasAttackPath: true, MaxImpact: "CRITICAL", EffectiveRisk: "HIGH"}
	b := ResourceRiskSignals{HasAttackPath: false, MaxImpact: "LOW", EffectiveRisk: "LOW"}
	ha, ia, ea, sa := AttackPathSortPriority(a, 10)
	hb, ib, eb, sb := AttackPathSortPriority(b, 99)
	if ha <= hb || ia <= ib {
		t.Fatalf("path+impact should sort first: %d,%d vs %d,%d", ha, ia, hb, ib)
	}
	_ = ea
	_ = eb
	_ = sa
	_ = sb
	c := ResourceRiskSignals{HasAttackPath: true, MaxImpact: "CRITICAL", EffectiveRisk: "HIGH"}
	d := ResourceRiskSignals{HasAttackPath: true, MaxImpact: "MEDIUM", EffectiveRisk: "MEDIUM"}
	_, ic, _, _ := AttackPathSortPriority(c, 0)
	_, id, _, _ := AttackPathSortPriority(d, 0)
	if ic <= id {
		t.Fatalf("CRITICAL should beat MEDIUM in max_impact rank")
	}
}
