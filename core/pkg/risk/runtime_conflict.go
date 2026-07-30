package risk

import (
	"strings"

	"github.com/fortuna/core/pkg/models"
)

// RuntimeConflictMultiplier down-weights runtime threat when loud network + execution
// coexist without K8s corroboration (spec X).
func RuntimeConflictMultiplier(signals []models.RuntimeSignal, events []models.RuntimeEvent) float64 {
	if len(signals) < 2 {
		return 1.0
	}
	var execW, netW int64
	for _, rs := range signals {
		c := int64(rs.Count)
		if c < 1 {
			c = 1
		}
		st := strings.ToUpper(strings.TrimSpace(rs.SignalType))
		cat := strings.ToLower(strings.TrimSpace(rs.Category))
		switch st {
		case "INTERACTIVE_SHELL_EXEC", "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "EBPF_EXEC_ACTIVITY",
			"TMP_BINARY_EXECUTION", "NAMESPACE_ESCAPE", "PROC_ROOT_PIVOT":
			execW += c
		case "NETWORK_QUEUE_ANOMALY", "EXTERNAL_EGRESS":
			netW += c
		default:
			if cat == "network" {
				netW += c
			} else if cat == "execution" || cat == "escape" {
				execW += c
			}
		}
	}
	if execW < 2 || netW < 2 {
		return 1.0
	}
	if CorroboratesK8sAPIAccess(events, signals) {
		return 1.0
	}
	return 0.92
}
