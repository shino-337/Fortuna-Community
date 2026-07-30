package explainability

import (
	"reflect"
	"testing"
)

func TestBuildOrderedChain_OrderAndDedupe(t *testing.T) {
	ch := BuildOrderedChain(
		[]string{"10", "10"},
		[]string{"f1"},
		[]string{"SIG_A"},
		[]string{"RECON_BURST"},
		[]string{"CAP_X"},
		[]string{"rule-1"},
	)
	if len(ch) != 6 {
		t.Fatalf("expected 6 steps, got %d", len(ch))
	}
	if ch[0].Layer != LayerRuntimeEvent || !reflect.DeepEqual(ch[0].Refs, []string{"10"}) {
		t.Fatalf("event step: %+v", ch[0])
	}
	if ch[5].Layer != LayerRiskRule {
		t.Fatalf("last layer: %+v", ch[5])
	}
}

func TestBuildOrderedChain_SkipsEmptyLayers(t *testing.T) {
	ch := BuildOrderedChain(nil, []string{"f1"}, nil, nil, nil, []string{"r1"})
	if len(ch) != 2 {
		t.Fatalf("got %+v", ch)
	}
	if ch[0].Layer != LayerBehaviorFact || ch[1].Layer != LayerRiskRule {
		t.Fatalf("unexpected: %+v", ch)
	}
}

func TestFlattenChainToEvidenceRefs(t *testing.T) {
	ch := BuildOrderedChain([]string{"e1"}, []string{"f1", "f2"}, nil, nil, nil, nil)
	flat := FlattenChainToEvidenceRefs(ch)
	want := []EvidenceChainRef{
		{Layer: LayerRuntimeEvent, Ref: "e1"},
		{Layer: LayerBehaviorFact, Ref: "f1"},
		{Layer: LayerBehaviorFact, Ref: "f2"},
	}
	if !reflect.DeepEqual(flat, want) {
		t.Fatalf("got %+v want %+v", flat, want)
	}
	if FlattenChainToEvidenceRefs(nil) != nil {
		t.Fatal("nil chain should return nil")
	}
}
