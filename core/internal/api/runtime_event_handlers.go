package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/rep"
)

type runtimeEventPayload struct {
	EventType     string `json:"event_type"`
	MitreTechnique string `json:"mitre_technique"`
	Signal        string `json:"signal"`
	Severity      string `json:"severity"`

	Pod struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		UID       string `json:"uid"`
	} `json:"pod"`

	PodUID string `json:"pod_uid"`
	PodUid string `json:"podUid"`

	Namespace  string `json:"namespace"`
	Syscall    string `json:"syscall"`
	Target     string `json:"target"`
	TargetPath string `json:"target_path"`
	Capability string `json:"capability"`
	Timestamp  int64  `json:"timestamp"`
}

type runtimeEventResponse struct {
	Processed int `json:"processed"`
}

// PostRuntimeEvents ingests runtime escape probe events from agents/sensors.
func PostRuntimeEvents(db *gorm.DB) gin.HandlerFunc {
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
			target := p.Target
			if target == "" {
				target = p.TargetPath
			}
			var ts *time.Time
			if p.Timestamp > 0 {
				t := time.Unix(p.Timestamp, 0).UTC()
				ts = &t
			}
			if podUID == "" || p.Syscall == "" {
				continue
			}

			log.Printf("[RuntimeEvent] Ingesting event: pod_uid=%s namespace=%s syscall=%s target=%s capability=%s",
				podUID, namespace, p.Syscall, target, p.Capability)
			
			result, err := rep.ProcessRuntimeEvent(c.Request.Context(), db, rep.RuntimeEventInput{
				PodUID:     podUID,
				Namespace:  namespace,
				Syscall:    p.Syscall,
				TargetPath: target,
				Capability: p.Capability,
				Timestamp:  ts,
			})
			if err != nil {
				log.Printf("[RuntimeEvent] ❌ Failed to process event for pod_uid=%s: %v", podUID, err)
				continue
			}
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
