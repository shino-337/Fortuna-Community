package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/rep"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func trustedRuntimeCluster(c *gin.Context) (string, bool) {
	clusterID, ok := resourceidentity.ClusterIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "trusted runtime cluster is required", "code": "runtime_cluster_context_missing"})
		return "", false
	}
	return clusterID, true
}

func bindRuntimeV2Payloads(c *gin.Context) ([]runtimeEventV2Payload, error) {
	body, err := c.GetRawData()
	if err != nil {
		return nil, err
	}
	var batch []runtimeEventV2Payload
	if err := json.Unmarshal(body, &batch); err == nil {
		return batch, nil
	}
	var single runtimeEventV2Payload
	if err := json.Unmarshal(body, &single); err != nil {
		return nil, err
	}
	return []runtimeEventV2Payload{single}, nil
}

// PostRuntimeEventsV2Scoped is the production v2 ingest path after ownership middleware.
func PostRuntimeEventsV2Scoped(db *gorm.DB) gin.HandlerFunc {
	rescoreMgr := riskengine.NewRuntimeAttackRescoreManager(db)
	return func(c *gin.Context) {
		clusterID, ok := trustedRuntimeCluster(c)
		if !ok {
			return
		}
		payloads, err := bindRuntimeV2Payloads(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		processed := 0
		now := time.Now().UTC()
		for _, p := range payloads {
			podUID := strings.TrimSpace(p.Pod.UID)
			if podUID == "" || strings.TrimSpace(p.Syscall) == "" || p.Confidence <= 0 {
				continue
			}
			id, err := resourceidentity.New(clusterID, podUID)
			if err != nil {
				continue
			}
			observedAt := &now
			if raw := strings.TrimSpace(p.ObservedAt); raw != "" {
				if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
					t := parsed.UTC()
					observedAt = &t
				}
			}
			ingestedAt := &now
			if raw := strings.TrimSpace(p.IngestedAt); raw != "" {
				if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
					t := parsed.UTC()
					ingestedAt = &t
				}
			}
			capabilityName := strings.TrimSpace(p.Capability)
			if capabilityName == "" {
				capabilityName = deriveCapabilityFromSignal(p.Signal)
			}
			payloadJSON := "{}"
			if len(p.PayloadJSON) > 0 {
				payloadJSON = string(p.PayloadJSON)
			}
			sourceKind := strings.TrimSpace(p.Source.Kind)
			sourceSensorID := strings.TrimSpace(p.Source.SensorID)
			sourceRule := strings.TrimSpace(p.Source.Rule)
			if sourceKind == "" {
				sourceKind = strings.TrimSpace(p.SourceKindFlat)
			}
			if sourceSensorID == "" {
				sourceSensorID = strings.TrimSpace(p.SourceSensorIDFlat)
			}
			if sourceRule == "" {
				sourceRule = strings.TrimSpace(p.SourceRuleFlat)
			}
			result, err := rep.ProcessRuntimeEventForIdentity(c.Request.Context(), db, id, rep.RuntimeEventInput{
				PodUID: podUID, PodName: strings.TrimSpace(p.Pod.Name), Namespace: strings.TrimSpace(p.Pod.Namespace),
				NodeName: strings.TrimSpace(p.Pod.Node), Syscall: strings.TrimSpace(p.Syscall), TargetPath: strings.TrimSpace(p.Target),
				Capability: capabilityName, Timestamp: observedAt, EventID: strings.TrimSpace(p.EventID),
				ObservedAt: observedAt, IngestedAt: ingestedAt, ResolutionState: strings.TrimSpace(p.ResolutionState),
				SourceKind: sourceKind, SourceSensorID: sourceSensorID, SourceRule: sourceRule,
				PayloadJSON: payloadJSON, PayloadHash: strings.TrimSpace(p.PayloadHash), Runtime: strings.TrimSpace(p.Runtime),
				EventType: strings.TrimSpace(p.EventType), Signal: strings.TrimSpace(p.Signal),
				MitreTechnique: strings.TrimSpace(p.MitreTechnique), Severity: strings.TrimSpace(p.Severity), Confidence: p.Confidence,
			})
			if err != nil {
				log.Printf("[RuntimeEventV2] scoped processing failed cluster=%s event_id=%s pod_uid=%s: %v", clusterID, p.EventID, podUID, err)
				continue
			}
			rescoreMgr.Notify(riskengine.RuntimeEventMeta{
				ClusterID: clusterID, PodUID: podUID, Runtime: strings.TrimSpace(p.Runtime),
				SourceKind: sourceKind, SourceRule: sourceRule, ResolutionState: strings.TrimSpace(p.ResolutionState),
				Syscall: strings.TrimSpace(p.Syscall), Severity: strings.TrimSpace(p.Severity), ObservedAt: observedAt,
			})
			if result != nil {
				processed++
			}
		}
		c.JSON(http.StatusOK, runtimeEventResponse{Processed: processed})
	}
}
