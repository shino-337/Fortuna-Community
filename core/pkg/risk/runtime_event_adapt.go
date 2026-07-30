package risk

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fortuna/core/pkg/models"
)

// AdaptRuntimeEventContract maps a stored row to the canonical contract.
// Confidence must be persisted on the row (> 0); there is no adapter-side default.
func AdaptRuntimeEventContract(e models.RuntimeEvent) (RuntimeEventContract, error) {
	if e.Confidence <= 0 {
		return RuntimeEventContract{}, fmt.Errorf("runtime event confidence required and must be > 0")
	}
	ts := e.CreatedAt
	if e.ObservedAt != nil && !e.ObservedAt.IsZero() {
		ts = *e.ObservedAt
	}
	evidence := map[string]any{}
	if strings.TrimSpace(e.PayloadJSON) != "" && e.PayloadJSON != "{}" {
		_ = json.Unmarshal([]byte(e.PayloadJSON), &evidence)
	}
	return RuntimeEventContract{
		Timestamp:  ts,
		SourceRule: e.SourceRule,
		SignalType: e.Signal,
		Severity:   e.Severity,
		Confidence: e.Confidence,
		Target:     firstNonEmpty(e.TargetPath, e.Capability),
		Context: RuntimeContext{
			Namespace: e.Namespace,
			Pod:       e.PodName,
			PodUID:    e.PodUID,
			Node:      e.NodeName,
		},
		Evidence: evidence,
	}, nil
}

func firstNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}
