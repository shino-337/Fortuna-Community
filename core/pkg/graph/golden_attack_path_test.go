package graph

import "testing"

// Golden-style regression hooks: extend with graph fixtures when CI golden snapshots land.
func TestSumCKDBRiskForCapabilityIDs_dedupAndCap(t *testing.T) {
	x := SumCKDBRiskForCapabilityIDs([]string{"SA_TOKEN", "SA_TOKEN", "CONTAINER_ACCESS"})
	if x <= 0 || x > 4.01 {
		t.Fatalf("unexpected ckdb sum %v", x)
	}
}

func TestApplyCapabilityValidationToRealism_adjustsDown(t *testing.T) {
	ch := &AttackChain{
		Realism: 1.0,
		CapabilityValidation: &CapabilityValidationResult{
			Confidence: 0,
			SoftMode:     true,
		},
	}
	applyCapabilityValidationToRealism(ch)
	if ch.Realism < 0.59 || ch.Realism > 0.61 {
		t.Fatalf("realism want ~0.6 for conf=0 (0.6+0.4*0), got %v", ch.Realism)
	}
}
