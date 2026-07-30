package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/rep"
	"github.com/fortuna/core/pkg/riskengine"
)

type runtimeEventPayload struct {
	EventType      string `json:"event_type"`
	MitreTechnique string `json:"mitre_technique"`
	Signal         string `json:"signal"`
	Severity       string `json:"severity"`

	Pod struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		UID       string `json:"uid"`
		Node      string `json:"node"`
	} `json:"pod"`

	PodUID string `json:"pod_uid"`
	PodUid string `json:"podUid"`

	Namespace  string `json:"namespace"`
	Syscall    string `json:"syscall"`
	Target     string `json:"target"`
	TargetPath string `json:"target_path"`
	Capability string `json:"capability"`
	Timestamp  int64  `json:"timestamp"`
	Runtime    string `json:"runtime"`

	// Confidence in (0,1] — required for ingest (sensors must supply explicit confidence).
	Confidence float64 `json:"confidence"`
}

type runtimeEventResponse struct {
	Processed int `json:"processed"`
}

// runtimeEventV2Payload is canonical-ish DTO for POST /api/v2/runtime/events (P0.1 minimal).
type runtimeEventV2Payload struct {
	EventID         string `json:"event_id"`
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

// PostRuntimeEvents ingests runtime escape probe events from agents/sensors.
func PostRuntimeEvents(db *gorm.DB) gin.HandlerFunc {
	rescoreMgr := riskengine.NewRuntimeAttackRescoreManager(db)
	return func(c *gin.Context) {
		var payloads []runtimeEventPayload
		if err := c.ShouldBindJSON(&payloads); err != nil {
			var single runtimeEventPayload
			if err2 := c.ShouldBindJSON(&single); err2 != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err2.Error()})
				return
			}
			payloads = []runtimeEventPayload{single}
		}

		processed := 0
		for _, p := range payloads {
			podUID := p.Pod.UID
			if podUID == "" {
				if p.PodUID != "" {
					podUID = p.PodUID
				} else {
					podUID = p.PodUid
				}
			}
			namespace := p.Pod.Namespace
			if namespace == "" {
				namespace = p.Namespace
			}
			podName := strings.TrimSpace(p.Pod.Name)
			nodeName := strings.TrimSpace(p.Pod.Node)
			runtimeSource := strings.TrimSpace(p.Runtime)
			target := p.Target
			if target == "" {
				target = p.TargetPath
			}
			capability := strings.TrimSpace(p.Capability)
			if capability == "" {
				capability = deriveCapabilityFromSignal(p.Signal)
			}
			var ts *time.Time
			if p.Timestamp > 0 {
				t := time.Unix(p.Timestamp, 0).UTC()
				ts = &t
			}
			if podUID == "" || p.Syscall == "" {
				continue
			}
			if p.Confidence <= 0 {
				log.Printf("[RuntimeEvent] skip event: confidence required (>0) pod_uid=%s", podUID)
				continue
			}

			log.Printf("[RuntimeEvent] Ingesting event: pod_uid=%s namespace=%s syscall=%s target=%s capability=%s",
				podUID, namespace, p.Syscall, target, p.Capability)

			result, err := rep.ProcessRuntimeEvent(c.Request.Context(), db, rep.RuntimeEventInput{
				PodUID:         podUID,
				PodName:        podName,
				Namespace:      namespace,
				NodeName:       nodeName,
				Syscall:        p.Syscall,
				TargetPath:     target,
				Capability:     capability,
				Timestamp:      ts,
				Runtime:        runtimeSource,
				EventType:      strings.TrimSpace(p.EventType),
				Signal:         strings.TrimSpace(p.Signal),
				MitreTechnique: strings.TrimSpace(p.MitreTechnique),
				Severity:       strings.TrimSpace(p.Severity),
				Confidence:     p.Confidence,
			})
			if err != nil {
				log.Printf("[RuntimeEvent] ❌ Failed to process event for pod_uid=%s: %v", podUID, err)
				continue
			}
			rescoreMgr.Notify(riskengine.RuntimeEventMeta{
				PodUID:     podUID,
				Runtime:    runtimeSource,
				SourceKind: runtimeSource,
				SourceRule: strings.TrimSpace(p.Signal),
				Syscall:    strings.TrimSpace(p.Syscall),
				Severity:   strings.TrimSpace(p.Severity),
				ObservedAt: ts,
			})
			if result != nil {
				log.Printf("[RuntimeEvent] ✅ Processed: pod_uid=%s signal=%s mitre=%s score=%d capability=%s severity=%s",
					podUID, result.Signal, result.Mitre, result.BaseScore, result.CapabilityID, result.Severity)
				processed++
			} else {
				log.Printf("[RuntimeEvent] ⚠️  No signal matched for pod_uid=%s syscall=%s target=%s", podUID, p.Syscall, target)
			}
		}

		c.JSON(http.StatusOK, runtimeEventResponse{Processed: processed})
	}
}

// PostRuntimeEventsV2 ingests canonical runtime events (P0.1 minimal).
// It is unauthenticated like v1 to support daemonset sensors.
func PostRuntimeEventsV2(db *gorm.DB) gin.HandlerFunc {
	rescoreMgr := riskengine.NewRuntimeAttackRescoreManager(db)
	return func(c *gin.Context) {
		var payloads []runtimeEventV2Payload
		if err := c.ShouldBindJSON(&payloads); err != nil {
			var single runtimeEventV2Payload
			if err2 := c.ShouldBindJSON(&single); err2 != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err2.Error()})
				return
			}
			payloads = []runtimeEventV2Payload{single}
		}

		processed := 0
		now := time.Now().UTC()

		for _, p := range payloads {
			podUID := strings.TrimSpace(p.Pod.UID)
			if podUID == "" || strings.TrimSpace(p.Syscall) == "" {
				continue
			}
			if p.Confidence <= 0 {
				log.Printf("[RuntimeEventV2] skip event: confidence required (>0) pod_uid=%s", podUID)
				continue
			}
			namespace := strings.TrimSpace(p.Pod.Namespace)

			var observedAt *time.Time
			if strings.TrimSpace(p.ObservedAt) != "" {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(p.ObservedAt)); err == nil {
					tt := t.UTC()
					observedAt = &tt
				}
			}
			if observedAt == nil {
				observedAt = &now
			}

			var ingestedAt *time.Time
			if strings.TrimSpace(p.IngestedAt) != "" {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(p.IngestedAt)); err == nil {
					tt := t.UTC()
					ingestedAt = &tt
				}
			}
			if ingestedAt == nil {
				ingestedAt = &now
			}

			target := strings.TrimSpace(p.Target)
			capability := strings.TrimSpace(p.Capability)
			if capability == "" {
				capability = deriveCapabilityFromSignal(p.Signal)
			}

			payloadJSON := "{}"
			if len(p.PayloadJSON) > 0 {
				payloadJSON = string(p.PayloadJSON)
			}

			sourceKind := strings.TrimSpace(p.Source.Kind)
			sourceSensorID := strings.TrimSpace(p.Source.SensorID)
			sourceRule := strings.TrimSpace(p.Source.Rule)
			// Fallback to flattened keys for compatibility with existing agent payload.
			if sourceKind == "" {
				sourceKind = strings.TrimSpace(p.SourceKindFlat)
			}
			if sourceSensorID == "" {
				sourceSensorID = strings.TrimSpace(p.SourceSensorIDFlat)
			}
			if sourceRule == "" {
				sourceRule = strings.TrimSpace(p.SourceRuleFlat)
			}

			result, err := rep.ProcessRuntimeEvent(c.Request.Context(), db, rep.RuntimeEventInput{
				PodUID:     podUID,
				PodName:    strings.TrimSpace(p.Pod.Name),
				Namespace:  namespace,
				NodeName:   strings.TrimSpace(p.Pod.Node),
				Syscall:    strings.TrimSpace(p.Syscall),
				TargetPath: target,
				Capability: capability,
				Timestamp:  observedAt,

				EventID:         strings.TrimSpace(p.EventID),
				ObservedAt:      observedAt,
				IngestedAt:      ingestedAt,
				ResolutionState: strings.TrimSpace(p.ResolutionState),
				SourceKind:      sourceKind,
				SourceSensorID:  sourceSensorID,
				SourceRule:      sourceRule,
				PayloadJSON:     payloadJSON,
				PayloadHash:     strings.TrimSpace(p.PayloadHash),

				Runtime:        strings.TrimSpace(p.Runtime),
				EventType:      strings.TrimSpace(p.EventType),
				Signal:         strings.TrimSpace(p.Signal),
				MitreTechnique: strings.TrimSpace(p.MitreTechnique),
				Severity:       strings.TrimSpace(p.Severity),
				Confidence:     p.Confidence,
			})
			if err != nil {
				log.Printf("[RuntimeEventV2] ❌ Failed to process event_id=%s pod_uid=%s: %v", p.EventID, podUID, err)
				continue
			}
			rescoreMgr.Notify(riskengine.RuntimeEventMeta{
				PodUID:          podUID,
				Runtime:         strings.TrimSpace(p.Runtime),
				SourceKind:      sourceKind,
				SourceRule:      sourceRule,
				Syscall:         strings.TrimSpace(p.Syscall),
				Severity:        strings.TrimSpace(p.Severity),
				ResolutionState: strings.TrimSpace(p.ResolutionState),
				ObservedAt:      observedAt,
			})
			if result != nil {
				processed++
			}
		}

		c.JSON(http.StatusOK, runtimeEventResponse{Processed: processed})
	}
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
