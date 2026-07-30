package rep

import "strings"

// RuntimeSignalMeta is the canonical metadata for a runtime signal type.
// P0.4 minimal: used by REP v2 compare path, later shared by API/UI.
type RuntimeSignalMeta struct {
	SignalType string
	Category   string
	Confidence float64
}

var runtimeSignalRegistry = map[string]RuntimeSignalMeta{
	"INTERACTIVE_SHELL_EXEC": {
		SignalType: "INTERACTIVE_SHELL_EXEC",
		Category:   "EXECUTION",
		Confidence: 0.80,
	},
	"TMP_BINARY_EXECUTION": {
		SignalType: "TMP_BINARY_EXECUTION",
		Category:   "EXECUTION",
		Confidence: 0.75,
	},
	"REMOTE_PAYLOAD_FETCH": {
		SignalType: "REMOTE_PAYLOAD_FETCH",
		Category:   "EXECUTION",
		Confidence: 0.70,
	},
	"EXTERNAL_EGRESS": {
		SignalType: "EXTERNAL_EGRESS",
		Category:   "NETWORK",
		Confidence: 0.70,
	},
	"SUSPICIOUS_EXEC_FROM_SNAPSHOT": {
		SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT",
		Category:   "EXECUTION",
		Confidence: 0.70,
	},
	"NETWORK_QUEUE_ANOMALY": {
		SignalType: "NETWORK_QUEUE_ANOMALY",
		Category:   "NETWORK",
		Confidence: 0.65,
	},
	"SERVICEACCOUNT_TOKEN_READ": {
		SignalType: "SERVICEACCOUNT_TOKEN_READ",
		Category:   "CREDENTIALS",
		Confidence: 0.80,
	},
	"HOST_PATH_ACCESS": {
		SignalType: "HOST_PATH_ACCESS",
		Category:   "FILESYSTEM",
		Confidence: 0.75,
	},
}

func lookupRuntimeSignalMeta(signalType string) (RuntimeSignalMeta, bool) {
	k := strings.ToUpper(strings.TrimSpace(signalType))
	v, ok := runtimeSignalRegistry[k]
	return v, ok
}
