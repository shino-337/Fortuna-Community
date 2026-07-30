// Package risk — canonical scoring contracts (spec I–IV, XIV).
//
// These types are the stable logical model. Persistence uses github.com/fortuna/core/pkg/models
// (GORM); adapters project DB rows ↔ contracts for scoring and explainability.
//
// Do not rename fields casually; UI and APIs may depend on JSON tags.

package risk

import "time"

// RuntimeEventContract is the normalized runtime event shape (target spec).
// models.RuntimeEvent is the store; map via AdaptRuntimeEventContract (requires persisted confidence > 0).
type RuntimeEventContract struct {
	Timestamp  time.Time      `json:"timestamp"`
	SourceRule string         `json:"sourceRule,omitempty"`
	SignalType string         `json:"signalType,omitempty"`
	Severity   string         `json:"severity,omitempty"`
	Confidence float64        `json:"confidence"`
	Target     string         `json:"target,omitempty"`
	Context    RuntimeContext `json:"context"`
	Evidence   map[string]any `json:"evidence,omitempty"`
}

// RuntimeContext scopes an event to the cluster hierarchy.
type RuntimeContext struct {
	ClusterID   string `json:"clusterId,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Pod         string `json:"pod,omitempty"`
	PodUID      string `json:"podUid,omitempty"`
	Node        string `json:"node,omitempty"`
	Container   string `json:"container,omitempty"`
	ContainerID string `json:"containerId,omitempty"`
}

// CapabilityContract is the canonical capability belief (spec VI).
type CapabilityContract struct {
	ID         string  `json:"id"`
	Scope      string  `json:"scope"` // container | pod | node | cluster
	State      string  `json:"state"` // observed | confirmed | exploited (maps to DB detected/confirmed/exploited)
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"` // static | runtime | inferred
}

// AttackStepContract describes a step in reasoning (spec VII); persisted paths use JSON nodes.
type AttackStepContract struct {
	TechniqueID           string   `json:"techniqueId,omitempty"`
	Requires              []string `json:"requires,omitempty"`
	Provides              []string `json:"provides,omitempty"`
	MitreTechniques       []string `json:"mitreTechniques,omitempty"`
	ContextScopes         []string `json:"contextScopes,omitempty"`
	RuntimeGroundingScore *float64 `json:"runtimeGroundingScore,omitempty"`
}

// CapabilityValidationContract summarizes chain validity (spec VII, XI).
type CapabilityValidationContract struct {
	Confidence float64  `json:"confidence"`
	Gaps       []string `json:"gaps,omitempty"`
}

// AttackChainContract is the logical chain container (spec III–VII).
type AttackChainContract struct {
	Steps                []AttackStepContract         `json:"steps,omitempty"`
	CapabilityValidation CapabilityValidationContract `json:"capabilityValidation"`
	MitreSummary         map[string]any               `json:"mitreSummary,omitempty"`
	RuntimeSummary       map[string]any               `json:"runtimeSummary,omitempty"`
}

// CorroborationResult is the truth layer output (spec IV).
type CorroborationResult struct {
	IsValid    bool    `json:"isValid"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}
