package risk

import "strings"

// Canonical logical capability IDs (spec III). Physical scoring/CKDB rows may use
// ESC_* / ID_TOKEN_POD; LogicalID documents the spec name where it differs.
type SignalCapabilityBinding struct {
	CapabilityID     string // persisted / merged into pod capability scoring ID
	LogicalID        string // spec label for explainability (optional)
	Scope            string // pod | container | node | cluster
	BaseState        string // confirmed | exploited — gated by confidence + corroboration
	StrongClaim      bool   // if true, corroboration not required for BaseState exploited
	ExploitConfFloor float64 // min signal confidence to allow exploited (0 = use global threshold)
}

// RuntimeSignalToCapability returns the binding for a runtime_signal.type, or ok=false.
func RuntimeSignalToCapability(signalType string) (SignalCapabilityBinding, bool) {
	st := strings.ToUpper(strings.TrimSpace(signalType))
	b, ok := runtimeSignalCapabilityRegistry[st]
	if !ok {
		return SignalCapabilityBinding{}, false
	}
	return b, true
}

// DefaultRuntimeUnknownCapability is the mandatory fallback: every signal maps to a capability.
var DefaultRuntimeUnknownCapability = SignalCapabilityBinding{
	CapabilityID:     "ESC_RUNTIME_PROBE",
	LogicalID:        "RUNTIME_ACTIVITY",
	Scope:            "pod",
	BaseState:        "confirmed",
	StrongClaim:      false,
	ExploitConfFloor: 0,
}

// registry: spec table III + platform IDs (single idempotent capability per signal class).
var runtimeSignalCapabilityRegistry = map[string]SignalCapabilityBinding{
	// Interactive / execution → INTERACTIVE_EXEC (logical) → ESC_RUNTIME_ACTIVE
	"INTERACTIVE_SHELL_EXEC": {
		CapabilityID: "ESC_RUNTIME_ACTIVE", LogicalID: "INTERACTIVE_EXEC",
		Scope: "pod", BaseState: "exploited", StrongClaim: false, ExploitConfFloor: 0,
	},
	"SUSPICIOUS_EXEC_FROM_SNAPSHOT": {
		CapabilityID: "ESC_RUNTIME_ACTIVE", LogicalID: "INTERACTIVE_EXEC",
		Scope: "pod", BaseState: "exploited", StrongClaim: false, ExploitConfFloor: 0,
	},
	"EBPF_EXEC_ACTIVITY": {
		CapabilityID: "ESC_RUNTIME_ACTIVE", LogicalID: "INTERACTIVE_EXEC",
		Scope: "pod", BaseState: "exploited", StrongClaim: false, ExploitConfFloor: 0,
	},
	"TMP_BINARY_EXECUTION": {
		CapabilityID: "ESC_RUNTIME_ACTIVE", LogicalID: "INTERACTIVE_EXEC",
		Scope: "pod", BaseState: "confirmed", StrongClaim: false, ExploitConfFloor: 0,
	},
	"REMOTE_PAYLOAD_FETCH": {
		CapabilityID: "ESC_RUNTIME_PROBE", LogicalID: "RUNTIME_FETCH",
		Scope: "pod", BaseState: "confirmed", StrongClaim: false, ExploitConfFloor: 0,
	},
	// Network anomaly → K8S_API_ACCESS / SA_TOKEN narrative uses ID_TOKEN_POD when corroborated
	"NETWORK_QUEUE_ANOMALY": {
		CapabilityID: "ID_TOKEN_POD", LogicalID: "K8S_API_ACCESS",
		Scope: "pod", BaseState: "confirmed", StrongClaim: false, ExploitConfFloor: 0,
	},
	"EXTERNAL_EGRESS": {
		CapabilityID: "ESC_RUNTIME_PROBE", LogicalID: "NETWORK_EGRESS",
		Scope: "pod", BaseState: "confirmed", StrongClaim: false, ExploitConfFloor: 0,
	},
	// Token / creds
	"SERVICEACCOUNT_TOKEN_READ": {
		CapabilityID: "ID_TOKEN_POD", LogicalID: "SA_TOKEN",
		Scope: "pod", BaseState: "exploited", StrongClaim: true, ExploitConfFloor: 0,
	},
	// Host / priv
	"HOST_PATH_ACCESS": {
		CapabilityID: "ESC_HOSTPATH_NODE", LogicalID: "NODE_ACCESS",
		Scope: "node", BaseState: "exploited", StrongClaim: false, ExploitConfFloor: 0,
	},
	"CAPABILITY_MISUSE": {
		CapabilityID: "ESC_PRIV_POD", LogicalID: "PRIVILEGED_WORKLOAD",
		Scope: "pod", BaseState: "confirmed", StrongClaim: false, ExploitConfFloor: 0,
	},
	"PROC_ROOT_PIVOT": {
		CapabilityID: "ESC_RUNTIME_PROC_ROOT", LogicalID: "NODE_ACCESS",
		Scope: "node", BaseState: "exploited", StrongClaim: false, ExploitConfFloor: 0,
	},
	"FS_ESCAPE_ATTEMPT": {
		CapabilityID: "ESC_HOSTPATH_NODE", LogicalID: "NODE_ACCESS",
		Scope: "node", BaseState: "exploited", StrongClaim: false, ExploitConfFloor: 0,
	},
	"NAMESPACE_ESCAPE": {
		CapabilityID: "ESC_RUNTIME_ACTIVE", LogicalID: "NAMESPACE_ESCAPE",
		Scope: "pod", BaseState: "exploited", StrongClaim: true, ExploitConfFloor: 0.5,
	},
}

// ResolveSignalCapability returns a binding for every signal type (spec: EVERY signal maps).
func ResolveSignalCapability(signalType string) SignalCapabilityBinding {
	if b, ok := RuntimeSignalToCapability(signalType); ok {
		return b
	}
	return DefaultRuntimeUnknownCapability
}
