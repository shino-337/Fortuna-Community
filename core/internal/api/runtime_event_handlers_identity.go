package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fortuna/core/internal/middleware"
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

func validRuntimeSourceRecordID(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) != 64 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}

// PostRuntimeEventsV2Scoped is the production v2 ingest path after ownership middleware.
func PostRuntimeEventsV2Scoped(db *gorm.DB) gin.HandlerFunc {
	rescoreMgr := riskengine.NewRuntimeAttackRescoreManager(db)
	return func(c *gin.Context) {
		clusterID, ok := trustedRuntimeCluster(c)
		if !ok {
			return
		}
		principal, scoped := middleware.AgentPrincipal(c)
		if scoped && principal.ClusterID != clusterID {
			c.JSON(http.StatusForbidden, gin.H{"error": "runtime agent cluster mismatch", "code": "runtime_agent_cluster_mismatch"})
			return
		}
		payloads, err := bindRuntimeV2Payloads(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Validate the replay identity contract for the complete processable
		// batch before any event can create effects. Invalid/no-op compatibility
		// payloads remain ignored as before.
		for _, p := range payloads {
			if strings.TrimSpace(p.Pod.UID) == "" || strings.TrimSpace(p.Syscall) == "" || p.Confidence <= 0 {
				continue
			}
			if !validRuntimeSourceRecordID(p.SourceRecordID) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "valid source_record_id is required", "code": "runtime_source_record_identity_required"})
				return
			}
		}

		processed := 0
		duplicates := 0
		now := time.Now().UTC()
		for _, p := range payloads {
			podUID := strings.TrimSpace(p.Pod.UID)
			if podUID == "" || strings.TrimSpace(p.Syscall) == "" || p.Confidence <= 0 {
				continue
			}
			sourceRecordID := strings.TrimSpace(p.SourceRecordID)
			agentID := ""
			if scoped {
				agentID = principal.AgentID
			} else {
				// Explicit legacy shared-token mode has no authenticated Agent
				// identity. Keep compatibility in a pod-local replay namespace
				// without pretending this is a trusted Agent principal.
				agentID = "legacy-pod:" + podUID
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
				AgentID: agentID,
				PodUID: podUID, PodName: strings.TrimSpace(p.Pod.Name), Namespace: strings.TrimSpace(p.Pod.Namespace),
				NodeName: strings.TrimSpace(p.Pod.Node), Syscall: strings.TrimSpace(p.Syscall), TargetPath: strings.TrimSpace(p.Target),
				Capability: capabilityName, Timestamp: observedAt, EventID: strings.TrimSpace(p.EventID),
				SourceRecordID: sourceRecordID,
				ObservedAt: observedAt, IngestedAt: ingestedAt, ResolutionState: strings.TrimSpace(p.ResolutionState),
				SourceKind: sourceKind, SourceSensorID: sourceSensorID, SourceRule: sourceRule,
				PayloadJSON: payloadJSON, PayloadHash: strings.TrimSpace(p.PayloadHash), Runtime: strings.TrimSpace(p.Runtime),
				EventType: strings.TrimSpace(p.EventType), Signal: strings.TrimSpace(p.Signal),
				MitreTechnique: strings.TrimSpace(p.MitreTechnique), Severity: strings.TrimSpace(p.Severity), Confidence: p.Confidence,
			})
			if err != nil {
				log.Printf("[RuntimeEventV2] scoped processing failed cluster=%s source_record_id=%s event_id=%s pod_uid=%s: %v", clusterID, sourceRecordID, p.EventID, podUID, err)
				if errors.Is(err, rep.ErrRuntimeSourceRecordConflict) {
					c.JSON(http.StatusConflict, gin.H{"error": "runtime source record identity conflict", "code": "runtime_source_record_conflict"})
					return
				}
				// Non-2xx is deliberate: Agent retains/retries the batch. Events
				// already committed earlier in this batch are safe to replay because
				// their physical source-record claims are idempotent.
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime event processing unavailable", "code": "runtime_event_processing_unavailable"})
				return
			}
			if result != nil && result.Duplicate {
				duplicates++
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
		c.JSON(http.StatusOK, runtimeEventResponse{Processed: processed, Duplicates: duplicates})
	}
}
