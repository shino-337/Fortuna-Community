package rep

import (
	"strings"

	"github.com/fortuna/core/pkg/models"
)

type synthesizedSignal struct {
	SignalType string
	Category   string
	Confidence float64
}

// synthesizeSignalsFromFacts is REP v2 minimal (compare-only in P0):
// behavior facts -> candidate semantic signals.
func synthesizeSignalsFromFacts(facts []models.RuntimeBehaviorFact) []synthesizedSignal {
	if len(facts) == 0 {
		return nil
	}
	seen := map[string]synthesizedSignal{}
	add := func(sig string) {
		meta, ok := lookupRuntimeSignalMeta(sig)
		if !ok {
			return
		}
		if cur, ok := seen[sig]; ok {
			if meta.Confidence > cur.Confidence {
				cur.Confidence = meta.Confidence
				cur.Category = meta.Category
				seen[sig] = cur
			}
			return
		}
		seen[sig] = synthesizedSignal{
			SignalType: meta.SignalType,
			Category:   meta.Category,
			Confidence: meta.Confidence,
		}
	}

	for i := range facts {
		ft := strings.ToUpper(strings.TrimSpace(facts[i].FactType))
		switch ft {
		case "INTERACTIVE_SHELL":
			add("INTERACTIVE_SHELL_EXEC")
			add("SUSPICIOUS_EXEC_FROM_SNAPSHOT")
		case "TMP_BINARY_EXEC":
			add("TMP_BINARY_EXECUTION")
			// Keep compatibility with existing runtime rules in P0.
			add("SUSPICIOUS_EXEC_FROM_SNAPSHOT")
		case "REMOTE_TOOL_EXEC":
			add("REMOTE_PAYLOAD_FETCH")
		case "EXTERNAL_CONNECT", "NETWORK_CONNECT":
			add("EXTERNAL_EGRESS")
			add("NETWORK_QUEUE_ANOMALY")
		case "SERVICEACCOUNT_TOKEN_READ":
			add("SERVICEACCOUNT_TOKEN_READ")
		case "HOST_PATH_TOUCH":
			add("HOST_PATH_ACCESS")
		}
	}

	out := make([]synthesizedSignal, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	return out
}
