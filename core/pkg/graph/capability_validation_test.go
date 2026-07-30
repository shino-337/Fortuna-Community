package graph

import (
	"slices"
	"testing"
)

func TestBuildCapabilityValidation_basicChain(t *testing.T) {
	steps := []AttackStep{
		{TechniqueID: "A", InputCaps: []string{}, OutputCaps: []string{"CONTAINER_ACCESS"}},
		{TechniqueID: "B", InputCaps: []string{"CONTAINER_ACCESS"}, OutputCaps: []string{"SA_TOKEN"}},
	}
	cv := BuildCapabilityValidation(steps)
	if cv == nil || cv.Confidence < 1.0 {
		t.Fatalf("unexpected validation %#v", cv)
	}
	if !cv.SoftMode {
		t.Fatal("expected soft_mode")
	}
}

func TestBuildCapabilityValidation_runtimeSatisfiesSA_TOKEN(t *testing.T) {
	steps := []AttackStep{
		{TechniqueID: "A", InputCaps: []string{"CONTAINER_ACCESS"}, OutputCaps: []string{"NETWORK_ACCESS"}},
		{TechniqueID: "B", InputCaps: []string{"SA_TOKEN"}, OutputCaps: []string{"CLUSTER_ADMIN"}},
	}
	cv := BuildCapabilityValidation(steps)
	if cv == nil || !slices.Contains(cv.Gaps, "SA_TOKEN") {
		t.Fatalf("expected structural SA_TOKEN gap first, got %#v", cv)
	}
	cv2 := BuildCapabilityValidationWithRuntimeSatisfaction(steps, []string{"SA_TOKEN"})
	if cv2 == nil || cv2.Confidence < 1.0 {
		t.Fatalf("expected full match with runtime SA_TOKEN, got %#v", cv2)
	}
	if len(cv2.Gaps) != 0 {
		t.Fatalf("gaps should clear: %v", cv2.Gaps)
	}
}
