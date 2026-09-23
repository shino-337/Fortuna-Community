package api

import (
	"encoding/json"
	"strings"
)

type runtimeEventResponse struct {
	Processed  int `json:"processed"`
	Duplicates int `json:"duplicates,omitempty"`
}

// runtimeEventV2Payload is canonical-ish DTO for POST /api/v2/runtime/events (P0.1 minimal).
type runtimeEventV2Payload struct {
	EventID         string `json:"event_id"`
	SourceRecordID  string `json:"source_record_id"`
	ObservedAt      string `json:"observed_at"` // RFC3339
	IngestedAt      string `json:"ingested_at"` // RFC3339
	ResolutionState string `json:"resolution_state"`

	// Flattened source fields (agent compatibility).
	// If nested `source` is missing, these will be used.
	SourceKindFlat     string `json:"source_kind"`
	SourceSensorIDFlat string `json:"source_sensor_id"`
	SourceRuleFlat     string `json:"source_rule"`

	Source struct {
		Kind     string `json:"kind"`
		SensorID string `json:"sensor_id"`
		Rule     string `json:"rule"`
	} `json:"source"`
	PayloadJSON json.RawMessage `json:"payload_json"`
	PayloadHash string          `json:"payload_hash"`

	Pod struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		UID       string `json:"uid"`
		Node      string `json:"node"`
	} `json:"pod"`

	Syscall    string `json:"syscall"`
	Target     string `json:"target"`
	Capability string `json:"capability"`

	// Compatibility with existing REP classification inputs
	EventType      string `json:"event_type"`
	Signal         string `json:"signal"`
	MitreTechnique string `json:"mitre_technique"`
	Severity       string `json:"severity"`
	Runtime        string `json:"runtime"`

	Confidence float64 `json:"confidence"`
}

func deriveCapabilityFromSignal(signal string) string {
	switch strings.ToUpper(strings.TrimSpace(signal)) {
	case "EBPF_EXEC_EVENT":
		return "EBPF_EXEC_TRACE"
	case "EBPF_CONNECT_EVENT":
		return "EBPF_CONNECT_TRACE"
	case "EBPF_ATTACH_EVENT":
		return "EBPF_ATTACH"
	default:
		return ""
	}
}
