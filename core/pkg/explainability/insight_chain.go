package explainability

// Layer keys for ordered causal display (G-EXP-01 MVP — pipeline order, not full DB join).
const (
	LayerRuntimeEvent    = "runtime_event"
	LayerBehaviorFact    = "behavior_fact"
	LayerRuntimeSignal   = "runtime_signal"
	LayerRuntimeIncident = "runtime_incident"
	LayerCapability      = "capability"
	LayerRiskRule        = "risk_rule"
)

// ChainStep is one layer in the explanation chain returned to API clients.
type ChainStep struct {
	Layer string   `json:"layer"`
	Refs  []string `json:"refs"`
}

// BuildOrderedChain returns refs in Fortuna pipeline order: event → fact → signal → incident → capability → rule.
func BuildOrderedChain(eventIDs, factIDs, signalTypes, incidentTypes, capabilityIDs, ruleIDs []string) []ChainStep {
	out := make([]ChainStep, 0, 6)
	if len(eventIDs) > 0 {
		out = append(out, ChainStep{Layer: LayerRuntimeEvent, Refs: dedupe(eventIDs)})
	}
	if len(factIDs) > 0 {
		out = append(out, ChainStep{Layer: LayerBehaviorFact, Refs: dedupe(factIDs)})
	}
	if len(signalTypes) > 0 {
		out = append(out, ChainStep{Layer: LayerRuntimeSignal, Refs: dedupe(signalTypes)})
	}
	if len(incidentTypes) > 0 {
		out = append(out, ChainStep{Layer: LayerRuntimeIncident, Refs: dedupe(incidentTypes)})
	}
	if len(capabilityIDs) > 0 {
		out = append(out, ChainStep{Layer: LayerCapability, Refs: dedupe(capabilityIDs)})
	}
	if len(ruleIDs) > 0 {
		out = append(out, ChainStep{Layer: LayerRiskRule, Refs: dedupe(ruleIDs)})
	}
	return out
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
