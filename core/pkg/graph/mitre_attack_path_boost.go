package graph

import (
	"strings"
)

// attackPathMitreTacticRealityMultiplier biases runtime MITRE boost toward post-access tactics
// (execution, escalation, impact) vs scan-heavy discovery noise.
func attackPathMitreTacticRealityMultiplier(tactic string) float64 {
	t := strings.ToLower(strings.TrimSpace(tactic))
	switch t {
	case "execution", "privilege escalation", "impact":
		return 1.4
	case "lateral movement":
		return 1.2
	case "discovery":
		return 0.8
	default:
		return 1.0
	}
}
