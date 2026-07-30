package graph

import (
	"math"
	"strings"
)

// BuildCapabilityValidation performs structural validation of requires ⊆ state across ordered steps,
// optionally treating runtimeSatisfied capability IDs as already satisfied preconditions.
func BuildCapabilityValidation(steps []AttackStep) *CapabilityValidationResult {
	return BuildCapabilityValidationWithRuntimeWeights(steps, nil)
}

// BuildCapabilityValidationWithRuntimeSatisfaction closes gaps when runtime lists full satisfiers (weight 1).
func BuildCapabilityValidationWithRuntimeSatisfaction(steps []AttackStep, runtimeSatisfied []string) *CapabilityValidationResult {
	if len(runtimeSatisfied) == 0 {
		return BuildCapabilityValidationWithRuntimeWeights(steps, nil)
	}
	w := make(map[string]float64, len(runtimeSatisfied))
	for _, x := range runtimeSatisfied {
		k := strings.ToUpper(strings.TrimSpace(x))
		if k != "" {
			w[k] = 1.0
		}
	}
	return BuildCapabilityValidationWithRuntimeWeights(steps, w)
}

// BuildCapabilityValidationWithRuntimeWeights applies fractional runtime closure (e.g. decayed SA_TOKEN).
func BuildCapabilityValidationWithRuntimeWeights(steps []AttackStep, satWeights map[string]float64) *CapabilityValidationResult {
	if len(steps) == 0 {
		return nil
	}
	state := map[string]bool{
		strings.ToUpper(strings.TrimSpace("CONTAINER_ACCESS")): true,
		strings.ToUpper(strings.TrimSpace("NETWORK_ACCESS")):   true,
	}
	var gaps []string
	totalReq := 0
	matchedSum := 0.0
	for _, step := range steps {
		for _, req := range step.InputCaps {
			r := strings.ToUpper(strings.TrimSpace(req))
			totalReq++
			if state[r] {
				matchedSum += 1.0
				continue
			}
			if satWeights != nil {
				if w := satWeights[r]; w > 0 {
					matchedSum += math.Min(1.0, w)
					continue
				}
			}
			gaps = append(gaps, r)
		}
		for _, p := range step.OutputCaps {
			state[strings.ToUpper(strings.TrimSpace(p))] = true
		}
	}
	conf := 1.0
	if totalReq > 0 {
		conf = matchedSum / float64(totalReq)
	}
	return &CapabilityValidationResult{
		Confidence: math.Round(conf*1000) / 1000,
		Gaps:       uniqStrings(gaps),
		SoftMode:   true,
	}
}
