package graph

// Canonical graph node types.
const (
	NodeTypePod            = "pod"
	NodeTypeServiceAccount = "service_account"
	NodeTypeRoleBinding    = "role_binding"
	NodeTypeClusterBinding = "cluster_role_binding"
	NodeTypeRole           = "role"
	NodeTypeClusterRole    = "cluster_role"
	NodeTypeCapability     = "capability"
	NodeTypeAttackStep     = "attack_step"
	NodeTypeNode           = "node"
	NodeTypeExternal       = "external"
)

// Canonical graph edge types.
// NOTE: only these types should be emitted into attack-path graph reasoning.
const (
	EdgeTypeNetworkReach       = "NETWORK_REACH"
	EdgeTypeNetworkReachSoft   = "NETWORK_REACH_SOFT"
	EdgeTypeServiceAccount     = "SERVICE_ACCOUNT_ACCESS"
	EdgeTypeHostAccess         = "HOST_ACCESS"
	EdgeTypeContainerEscape    = "CONTAINER_ESCAPE"
	EdgeTypeCVEExploit         = "CVE_EXPLOIT"
	EdgeTypeLateralMove        = "LATERAL_MOVE"
	EdgeTypeRbacBinding        = "RBAC_BINDING"
	EdgeTypeGrantsRole         = "GRANTS_ROLE"
	EdgeTypeHasAttackStep      = "HAS_ATTACK_STEP"
	EdgeTypeCanStealCredential = "CAN_STEAL_CREDENTIALS"
)

// AttackPath represents an attack path from a source to a target
type AttackPath struct {
	// PathID is stable within one bundle response (p0, p1, …) and matches AttackChain.Paths entries.
	PathID      string     `json:"path_id,omitempty"`
	Nodes       []PathNode `json:"nodes"`
	Edges       []PathEdge `json:"edges"`
	TotalRisk   float64    `json:"total_risk"`
	Difficulty  float64    `json:"difficulty"` // 0-1, how hard to execute
	Impact      float64    `json:"impact"`     // 0-1, potential damage
	Length      int        `json:"length"`
	Description string     `json:"description"`
	// EnrichedFromPCE is true when capability/attack-step edges were added (Phase 1.2)
	EnrichedFromPCE bool `json:"enrichedFromPce"`
	// Explainability provides data-driven reasoning shown in UI.
	Explainability *PathExplainability `json:"explainability,omitempty"`
}

type PathExplainability struct {
	Class       string                 `json:"class"`
	Strength    float64                `json:"strength"`
	Feasibility PathFeasibility        `json:"feasibility"`
	Scoring     PathScoringBreakdown   `json:"scoring"`
	RiskInject  PathRiskInjection      `json:"risk_injection"`
	Evidence    PathExplainabilityData `json:"evidence"`
}

type PathFeasibility struct {
	NetworkDecision string  `json:"network_decision"` // allow|soft_allow|deny|n/a
	Reason          string  `json:"reason"`           // observed_egress|new_pod_fallback|deny_cache|not_required
	Confidence      float64 `json:"confidence"`       // 0..1
}

type PathScoringBreakdown struct {
	WeakestLink      float64 `json:"weakest_link"`
	LengthPenalty    float64 `json:"length_penalty"`
	UncertainPenalty float64 `json:"uncertain_penalty"`
	FinalStrength    float64 `json:"final_strength"`
}

type PathRiskInjection struct {
	E                   float64 `json:"E"`
	R                   float64 `json:"R"`
	I                   float64 `json:"I"`
	Scale               float64 `json:"scale"`
	EPoints             float64 `json:"e_points"`
	RPoints             float64 `json:"r_points"`
	IPoints             float64 `json:"i_points"`
	EstimatedScoreDelta float64 `json:"estimated_score_delta"`
}

type PathExplainabilityData struct {
	Capabilities []string `json:"capabilities"`
	AttackSteps  []string `json:"attack_steps"`
}

// PathNode represents a node in an attack path
type PathNode struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"` // Pod, ServiceAccount, Role, etc.
	Properties map[string]interface{} `json:"properties"`
}

// PathEdge represents an edge in an attack path
type PathEdge struct {
	Type       string                 `json:"type"` // USES_SERVICE_ACCOUNT, BINDS_TO, etc.
	Source     string                 `json:"source"`
	Target     string                 `json:"target"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Permission represents permissions for a ServiceAccount (graph-based)
type Permission struct {
	RoleName  string       `json:"role_name"`
	Rules     []PolicyRule `json:"rules"`
	Namespace string       `json:"namespace"`
	RoleType  string       `json:"role_type"` // Role or ClusterRole
}

// PolicyRule represents a Kubernetes policy rule
type PolicyRule struct {
	Verbs         []string `json:"verbs"`
	Resources     []string `json:"resources"`
	APIGroups     []string `json:"api_groups"`
	ResourceNames []string `json:"resource_names,omitempty"`
}

// RiskyPod represents a pod with privilege escalation risk
type RiskyPod struct {
	UID                string `json:"uid"`
	Name               string `json:"name"`
	Namespace          string `json:"namespace"`
	ServiceAccountName string `json:"service_account_name"`
	RoleName           string `json:"role_name"`
	// RiskScore is the peak attack-path strength mapped to 0–10 (not the unified V3 0–100 score).
	RiskScore    float64  `json:"risk_score"`
	RiskReason   string   `json:"risk_reason"`
	UnifiedScore *float64 `json:"unified_score,omitempty"` // latest v3 total_score when present
}

// AttackObjective summarizes correlated paths into operator-facing priorities.
type AttackObjective struct {
	Objective    string   `json:"objective"`
	FinalTarget  string   `json:"final_target,omitempty"`
	Priority     string   `json:"priority"` // high|medium|low
	ChainCount   int      `json:"chain_count,omitempty"`
	PathCount    int      `json:"path_count"`
	MaxStrength  float64  `json:"max_strength"`
	InvolvedPods int      `json:"involved_pods"`
	PodUIDs      []string `json:"pod_uids,omitempty"`
	Confidence   string   `json:"confidence"` // high|medium|low
}

// ─── Phase 3 types (Spec §2) ──────────────────────────────────────────────────

// ChainValidity describes whether a chain passed hard/soft gate validation.
// Severity: "" = valid, "SOFT" = penalty applied, "HARD" = invalid (score floored).
type ChainValidity struct {
	IsValid  bool   `json:"is_valid"`
	Severity string `json:"severity,omitempty"` // "" | "SOFT" | "HARD"
	Reason   string `json:"reason,omitempty"`
}

// AttackStep is a single, technique-bound action in an attack chain.
// One step = one registered technique (spec §2.2). No free-text steps.
type AttackStep struct {
	TechniqueID  string   `json:"technique_id"`
	Name         string   `json:"name"`
	InputCaps    []string `json:"input_caps"`
	OutputCaps   []string `json:"output_caps"`
	Realism      float64  `json:"realism"`
	Cost         float64  `json:"cost"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	// MitreTechniques ATT&CK references for this step (from technique overlay / registry).
	MitreTechniques []MitreTechniqueRef `json:"mitre_techniques,omitempty"`
	// RuntimeObserved is true when any MITRE ID on this step matched runtime_events for chain pods (7d window).
	RuntimeObserved bool `json:"runtime_observed,omitempty"`
	// ContextScopes from overlay (pod | node | control_plane) — optional execution-surface hints.
	ContextScopes []string `json:"context_scopes,omitempty"`
	// RuntimeGroundingScore = events matching this step's MITRE ÷ runtime events considered for chain (set when pool non-empty).
	RuntimeGroundingScore *float64 `json:"runtime_grounding_score,omitempty"`
}

// Assumption represents an inferred condition that the chain depends on
// but cannot be directly confirmed from cluster data (spec §2.3).
// Every assumption must have a Source and Confidence to remain auditable.
type Assumption struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	// Source: "inferred" (from graph reachability) | "derived" (from config)
	// | "heuristic" (from industry knowledge)
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

// Evidence is a traceable fact from the real cluster state (spec §2.4).
// Inspired by K8sCout: everything traces back to a real resource.
type Evidence struct {
	// Type: "FACT" (confirmed config) | "CONFIG" (policy setting) | "RELATION" (binding/mount)
	Type string `json:"type"`
	// SourceRef points to the real Kubernetes resource (e.g. "pod/kube-proxy", "role/cluster-admin").
	SourceRef string `json:"source_ref"`
	Detail    string `json:"detail"`
}

// AttackStory is the human-readable narrative (spec §5.3).
// Evidence-first ordering enforced by the UI (spec §5.4).
type AttackStory struct {
	Headline    string   `json:"headline"`
	Narrative   string   `json:"narrative"`
	ImpactText  string   `json:"impact_text"`
	ExploitText string   `json:"exploit_text"`
	Steps       []string `json:"steps"`
	// EvidenceSummary is a short human-readable list of key evidence items.
	// Displayed before story text (spec §5.4 order: Evidence → Assumptions → Story → Impact → Steps).
	EvidenceSummary []string `json:"evidence_summary,omitempty"`
	// AssumptionSummary is a short list of key assumptions displayed between evidence and story.
	AssumptionSummary []string `json:"assumption_summary,omitempty"`
}

// AttackChain represents a complete, auditable multi-step attack scenario.
type AttackChain struct {
	ChainID string `json:"chain_id"`
	// ID mirrors ChainID for API consumers that expect a stable "id" field (resource risk projection spec).
	ID            string   `json:"id,omitempty"`
	SourceID      string   `json:"source_id,omitempty"` // entry workload (typically pod UID)
	TargetID      string   `json:"target_id,omitempty"` // graph terminal node id for the chain
	Paths         []string `json:"paths"`
	Objective     string   `json:"objective"`
	ChainStrength float64  `json:"chain_strength"`
	Confidence    string   `json:"confidence"`     // "high"|"medium"|"low" (band)
	ConfidenceRaw float64  `json:"confidence_raw"` // 0–1 for UI breakdown display
	FinalTarget   string   `json:"final_target"`
	Explanation   string   `json:"explanation"`
	Type          string   `json:"type"`
	// Impact is a discrete band for UI: LOW | MEDIUM | HIGH | CRITICAL (derived from ImpactScope).
	Impact string `json:"impact,omitempty"`

	// InvolvedResources lists deduped graph node IDs on the composed paths (pods, identities, roles, etc.).
	InvolvedResources []string `json:"involved_resources,omitempty"`

	// Phase 2 scoring fields
	Realism float64 `json:"realism"`
	// RealismPreCapabilityValidation is extractRealism output before capability-validation multipliers (for re-apply after runtime gap closure).
	RealismPreCapabilityValidation float64       `json:"-"`
	ExploitCost                    int           `json:"exploit_cost"`
	BlastRadius                    float64       `json:"blast_radius"`
	Validity                       ChainValidity `json:"validity"`
	Story                          AttackStory   `json:"story"`
	Primary                        bool          `json:"primary"`

	// Phase 3 — technique-bound steps + provenance (spec §2.1–2.4)
	Steps       []AttackStep `json:"steps"`
	Assumptions []Assumption `json:"assumptions"`
	Evidence    []Evidence   `json:"evidence"`

	// ImpactScope classifies the blast radius type: "NODE" | "NAMESPACE" | "CLUSTER"
	ImpactScope string `json:"impact_scope"`
	// VariantSpread = log(1 + len(VariantNodes)) — used in primary score formula (spec §4.4).
	VariantSpread float64 `json:"variant_spread"`

	// VariantNodes: distinct intermediate nodes from semantic dedup (FIX J).
	VariantNodes []string `json:"variant_nodes,omitempty"`

	// MitreSummary correlates path-derived ATT&CK IDs with runtime observations (semantic unification layer).
	MitreSummary *AttackChainMitreSummary `json:"mitre_summary,omitempty"`

	// MitreCoverage lists observed | inferred | not_covered per path MITRE id (minimal gap engine).
	MitreCoverage []MitreCoverageItem `json:"mitre_coverage,omitempty"`

	// CapabilityValidation is soft-mode fuzzy validation of requires/provides across steps (Phase 2 guardrails).
	CapabilityValidation *CapabilityValidationResult `json:"capability_validation,omitempty"`
}

// CapabilityValidationResult is advisory-only in production (never drops chains).
type CapabilityValidationResult struct {
	Confidence float64  `json:"confidence"` // matched_requires / max(1,total_requires)
	Gaps       []string `json:"gaps,omitempty"`
	SoftMode   bool     `json:"soft_mode"`
}
