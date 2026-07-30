package risk

import (
	"github.com/fortuna/core/pkg/models"
)

// EvaluateK8sAPICorroboration is the truth-layer wrapper for K8s API claims (spec IV).
func EvaluateK8sAPICorroboration(events []models.RuntimeEvent, signals []models.RuntimeSignal) CorroborationResult {
	if CorroboratesK8sAPIAccess(events, signals) {
		return CorroborationResult{
			IsValid:    true,
			Confidence: 0.88,
			Reason:     "k8s_api_strong_or_corroborated_weak",
		}
	}
	return CorroborationResult{
		IsValid:    false,
		Confidence: 0.22,
		Reason:     "no_k8s_api_corroboration",
	}
}
