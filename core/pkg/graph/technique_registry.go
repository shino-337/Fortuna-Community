package graph

// ─── Technique Registry (Phase 3, Spec §3) ────────────────────────────────────
//
// Every capability transition in the attack chain MUST go through an explicit
// technique. There are no implicit derivations. This ensures determinism and
// auditability (spec §1, design goal 3).
//
// Reference: analogous to BloodHound edge types and K8sCout technique nodes.

// AttackTechnique describes one discrete, auditable attack step.
// A chain is valid only if consecutive capabilities are connected by a registered technique.
type AttackTechnique struct {
	// TechniqueID is the canonical identifier (maps to ATT&CK or Fortuna internal).
	TechniqueID string
	// Name is the human-readable name used in narrative steps.
	Name string
	// Requires is the set of capabilities the attacker must already have.
	Requires []string
	// Provides is the set of capabilities gained after successfully executing this technique.
	Provides []string
	// Cost is the attacker effort (additive; lower = easier).
	Cost float64
	// Realism is the base probability that this technique succeeds in a typical cluster.
	// Modified by ClusterHardeningHints at chain evaluation time.
	Realism float64
	// Preconditions are additional checks beyond capability matching.
	// Empty means the technique applies whenever Requires are satisfied.
	Preconditions []string
	// MitreTechniques is populated from technique_overlay.yaml via TechniqueByID (single semantic layer).
	MitreTechniques []MitreTechniqueRef `json:"mitre_techniques,omitempty"`
	// RiskWeight optional multiplier from overlay (default 1.0).
	RiskWeight float64 `json:"risk_weight,omitempty"`
	// ContextScopes execution-surface hints from overlay (pod | node | control_plane).
	ContextScopes []string `json:"context_scopes,omitempty"`
}

// techniqueRegistry is the single source of truth for Kubernetes attack techniques.
// Key = TechniqueID.
var techniqueRegistry = map[string]AttackTechnique{
	// ── Container escape techniques ───────────────────────────────────────────
	"ESCAPE_HOSTPATH": {
		TechniqueID: "ESCAPE_HOSTPATH",
		Name:        "HostPath volume node escape",
		Requires:    []string{"CONTAINER_ACCESS"},
		Provides:    []string{"NODE_SHELL_ACCESS"},
		Cost:        3,
		Realism:     0.90,
		Preconditions: []string{"pod_has_writable_hostpath"},
	},
	"ESCAPE_RUNTIME": {
		TechniqueID: "ESCAPE_RUNTIME",
		Name:        "Container runtime escape",
		Requires:    []string{"CONTAINER_ACCESS"},
		Provides:    []string{"NODE_SHELL_ACCESS"},
		Cost:        3,
		Realism:     0.85,
		Preconditions: []string{"runtime_escape_confirmed"},
	},
	"ESCAPE_PROC_ROOT": {
		TechniqueID: "ESCAPE_PROC_ROOT",
		Name:        "/proc/1/root filesystem pivot",
		Requires:    []string{"CONTAINER_ACCESS"},
		Provides:    []string{"NODE_SHELL_ACCESS"},
		Cost:        3,
		Realism:     0.80,
		Preconditions: []string{"hostpid_or_privileged"},
	},
	"ESCAPE_PRIVILEGED": {
		TechniqueID: "ESCAPE_PRIVILEGED",
		Name:        "Privileged container escape",
		Requires:    []string{"CONTAINER_ACCESS"},
		Provides:    []string{"CONTAINER_RUNTIME_ACCESS"},
		Cost:        2,
		Realism:     0.80,
		Preconditions: []string{"pod_is_privileged"},
	},
	"ESCAPE_HOSTPID": {
		TechniqueID: "ESCAPE_HOSTPID",
		Name:        "hostPID namespace pivot",
		Requires:    []string{"CONTAINER_ACCESS"},
		Provides:    []string{"CONTAINER_RUNTIME_ACCESS"},
		Cost:        2,
		Realism:     0.75,
		Preconditions: []string{"pod_has_hostpid"},
	},

	// ── Node-level harvesting techniques ─────────────────────────────────────
	"KUBELET_TOKEN_HARVEST": {
		TechniqueID: "KUBELET_TOKEN_HARVEST",
		Name:        "Harvest SA tokens via node filesystem",
		Requires:    []string{"NODE_SHELL_ACCESS"},
		Provides:    []string{"SA_TOKEN"},
		Cost:        3,
		Realism:     0.75,
		Preconditions: []string{"automount_enabled_on_target_pod"},
	},
	"RUNTIME_TOKEN_HARVEST": {
		TechniqueID: "RUNTIME_TOKEN_HARVEST",
		Name:        "Harvest SA token via container runtime exec",
		Requires:    []string{"CONTAINER_RUNTIME_ACCESS"},
		Provides:    []string{"SA_TOKEN"},
		Cost:        2,
		Realism:     0.60,
		Preconditions: []string{"target_pod_running_on_same_node"},
	},
	"KUBELET_API_PROBE": {
		TechniqueID: "KUBELET_API_PROBE",
		Name:        "Kubelet API credential enumeration",
		Requires:    []string{"NODE_SHELL_ACCESS"},
		Provides:    []string{"KUBELET_API_ACCESS"},
		Cost:        2,
		Realism:     0.70,
		Preconditions: []string{"kubelet_anon_auth_disabled_unknown"},
	},

	// ── RBAC / privilege escalation techniques ────────────────────────────────
	"RBAC_PRIV_ESC": {
		TechniqueID: "RBAC_PRIV_ESC",
		Name:        "Privilege escalation via RBAC role binding",
		Requires:    []string{"SA_TOKEN"},
		Provides:    []string{"ROLE"},
		Cost:        1,
		Realism:     0.95,
	},
	"CLUSTER_ADMIN_ESC": {
		TechniqueID: "CLUSTER_ADMIN_ESC",
		Name:        "Cluster-admin escalation via RBAC",
		Requires:    []string{"SA_TOKEN"},
		Provides:    []string{"CLUSTER_ADMIN"},
		Cost:        1,
		Realism:     0.95,
	},

	// ── Lateral movement techniques ───────────────────────────────────────────
	"LATERAL_NETWORK": {
		TechniqueID: "LATERAL_NETWORK",
		Name:        "Lateral movement via network reach",
		Requires:    []string{"NETWORK_ACCESS"},
		Provides:    []string{"CONTAINER_ACCESS"},
		Cost:        1,
		Realism:     0.80,
	},
	"SA_TOKEN_REUSE": {
		TechniqueID: "SA_TOKEN_REUSE",
		Name:        "Reuse existing service account token",
		Requires:    []string{"SA_TOKEN"},
		Provides:    []string{"SA_TOKEN"},
		Cost:        1,
		Realism:     0.90,
		Preconditions: []string{"same_sa_or_node_derived"},
	},
}

// TechniqueByID looks up a technique, returning (technique, found).
// MITRE refs, risk_weight, and context_scope are merged from technique_overlay.yaml when present.
func TechniqueByID(id string) (AttackTechnique, bool) {
	t, ok := techniqueRegistry[id]
	if !ok {
		return AttackTechnique{}, false
	}
	if refs := MitreRefsForTechnique(id); len(refs) > 0 {
		t.MitreTechniques = refs
	}
	if row, ok := overlayForTechnique(id); ok {
		rw := row.RiskWeight
		if rw <= 0 {
			rw = defaultOverlayRiskWeight()
		}
		t.RiskWeight = rw
		if len(row.ContextScope) > 0 {
			t.ContextScopes = append([]string(nil), row.ContextScope...)
		}
	}
	if t.RiskWeight <= 0 {
		t.RiskWeight = defaultOverlayRiskWeight()
		if t.RiskWeight <= 0 {
			t.RiskWeight = 1.0
		}
	}
	return t, true
}

// ── Capability-to-technique mapping ──────────────────────────────────────────
//
// capabilityToTechniques maps a (have, want) capability pair to the technique(s)
// that can bridge them. This replaces the implicit "if NODE_ACCESS add SA_TOKEN:*"
// derivation with an explicit lookup.
//
// Key: "FROM_CAP→TO_CAP"

var capabilityBridgeMap = map[string]string{
	"NODE_SHELL_ACCESS→SA_TOKEN":             "KUBELET_TOKEN_HARVEST",
	"NODE_SHELL_ACCESS→KUBELET_API_ACCESS":   "KUBELET_API_PROBE",
	"CONTAINER_RUNTIME_ACCESS→SA_TOKEN":      "RUNTIME_TOKEN_HARVEST",
	"CONTAINER_ACCESS→NODE_SHELL_ACCESS":     "ESCAPE_HOSTPATH",   // default; overridden by cap ID
	"CONTAINER_ACCESS→CONTAINER_RUNTIME_ACCESS": "ESCAPE_PRIVILEGED",
	"NETWORK_ACCESS→CONTAINER_ACCESS":        "LATERAL_NETWORK",
	"SA_TOKEN→ROLE":                          "RBAC_PRIV_ESC",
	"SA_TOKEN→CLUSTER_ADMIN":                 "CLUSTER_ADMIN_ESC",
	"SA_TOKEN→SA_TOKEN":                      "SA_TOKEN_REUSE",
}

// CapabilityToBridgeTechnique returns the technique that connects `from` to `to`,
// or empty string if no registered bridge exists.
// This enforces the "no implicit derivation" rule (spec §3.3).
func CapabilityToBridgeTechnique(from, to string) (AttackTechnique, bool) {
	key := from + "→" + to
	id, ok := capabilityBridgeMap[key]
	if !ok {
		return AttackTechnique{}, false
	}
	return TechniqueByID(id)
}

// ── Capability-to-technique from escape cap ID ────────────────────────────────
//
// EscapeCapToTechnique maps a PodCapability.CapabilityID to the technique it
// represents. Used during path building (relational_path_builder.go) to replace
// string-based edge type inference.

var escapeCapToTechniqueID = map[string]string{
	"ESC_HOSTPATH_NODE":   "ESCAPE_HOSTPATH",
	"ESC_RUNTIME_ACTIVE":  "ESCAPE_RUNTIME",
	"ESC_RUNTIME_PROC_ROOT": "ESCAPE_PROC_ROOT",
	"ESC_PRIV_POD":        "ESCAPE_PRIVILEGED",
	"ESC_HOSTPID_POD":     "ESCAPE_HOSTPID",
	"ESC_HOSTIPC_POD":     "ESCAPE_HOSTPID",  // same class
	"ESC_RUNTIME_PROBE":   "ESCAPE_RUNTIME",
}

// EscapeCapToTechnique returns the technique for a raw capability ID.
func EscapeCapToTechnique(capID string) (AttackTechnique, bool) {
	id, ok := escapeCapToTechniqueID[capID]
	if !ok {
		return AttackTechnique{}, false
	}
	return TechniqueByID(id)
}

// ── Chain type to technique sequence ─────────────────────────────────────────
//
// ChainTypeToTechniqueSequence maps a chain type to the ordered list of technique
// IDs that constitute the canonical attack path. Used by buildAttackSteps.

var chainTypeToTechniqueSequence = map[string][]string{
	"ESCAPE_TO_PRIV_ESC": {
		"ESCAPE_HOSTPATH",        // or ESCAPE_RUNTIME etc (overridden by actual cap)
		"KUBELET_TOKEN_HARVEST",
		"RBAC_PRIV_ESC",
	},
	"LATERAL_TO_PRIV_ESC": {
		"LATERAL_NETWORK",
		"KUBELET_TOKEN_HARVEST",
		"RBAC_PRIV_ESC",
	},
	"NETWORK_BRIDGE": {
		"LATERAL_NETWORK",
		"SA_TOKEN_REUSE",
		"RBAC_PRIV_ESC",
	},
	"SA_TOKEN_REUSE": {
		"SA_TOKEN_REUSE",
		"RBAC_PRIV_ESC",
	},
	"NODE_DOMINANCE": {
		"KUBELET_TOKEN_HARVEST",
		"RBAC_PRIV_ESC",
	},
	"PRIV_ESC_LADDER": {
		"RBAC_PRIV_ESC",
		"CLUSTER_ADMIN_ESC",
	},
}

// TechniqueSequenceForChain returns the ordered technique IDs for a chain type.
// Returns nil if no sequence is defined (caller should use generic fallback).
func TechniqueSequenceForChain(chainType string) []string {
	return chainTypeToTechniqueSequence[chainType]
}
