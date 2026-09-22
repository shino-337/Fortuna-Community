package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/rep"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errRuntimeEventReplayConflict = errors.New("runtime event replay content conflict")

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

type preparedRuntimeEvent struct {
	eventID string
	digest  string
	input   rep.RuntimeEventInput
	meta    riskengine.RuntimeEventMeta
	id      resourceidentity.Identity
}

func prepareRuntimeV2Batch(clusterID string, payloads []runtimeEventV2Payload, now time.Time) ([]preparedRuntimeEvent, error) {
	if len(payloads) == 0 {
		return nil, fmt.Errorf("runtime event batch is empty")
	}
	seen := make(map[string]struct{}, len(payloads))
	prepared := make([]preparedRuntimeEvent, 0, len(payloads))
	for i, p := range payloads {
		eventID := strings.TrimSpace(p.EventID)
		podUID := strings.TrimSpace(p.Pod.UID)
		syscall := strings.TrimSpace(p.Syscall)
		if eventID == "" || len(eventID) > 64 {
			return nil, fmt.Errorf("runtime event %d requires event_id", i)
		}
		if _, exists := seen[eventID]; exists {
			return nil, fmt.Errorf("runtime event %d duplicates event_id %q within batch", i, eventID)
		}
		seen[eventID] = struct{}{}
		if podUID == "" || syscall == "" {
			return nil, fmt.Errorf("runtime event %d requires pod uid and syscall", i)
		}
		if p.Confidence <= 0 || p.Confidence > 1 {
			return nil, fmt.Errorf("runtime event %d confidence must be within (0,1]", i)
		}
		id, err := resourceidentity.New(clusterID, podUID)
		if err != nil {
			return nil, fmt.Errorf("runtime event %d invalid identity: %w", i, err)
		}

		observedAt := now
		if raw := strings.TrimSpace(p.ObservedAt); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return nil, fmt.Errorf("runtime event %d invalid observed_at: %w", i, err)
			}
			observedAt = parsed.UTC()
		}
		ingestedAt := now
		if raw := strings.TrimSpace(p.IngestedAt); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return nil, fmt.Errorf("runtime event %d invalid ingested_at: %w", i, err)
			}
			ingestedAt = parsed.UTC()
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

		digestPayload := p
		// ingested_at is a transport/server receipt timestamp, not immutable
		// source-event identity. Retries may legitimately arrive later.
		digestPayload.IngestedAt = ""
		canonical, err := json.Marshal(digestPayload)
		if err != nil {
			return nil, fmt.Errorf("runtime event %d cannot canonicalize: %w", i, err)
		}
		sum := sha256.Sum256(canonical)
		digest := hex.EncodeToString(sum[:])

		observedCopy, ingestedCopy := observedAt, ingestedAt
		input := rep.RuntimeEventInput{
			PodUID: podUID, PodName: strings.TrimSpace(p.Pod.Name), Namespace: strings.TrimSpace(p.Pod.Namespace),
			NodeName: strings.TrimSpace(p.Pod.Node), Syscall: syscall, TargetPath: strings.TrimSpace(p.Target),
			Capability: capabilityName, Timestamp: &observedCopy, EventID: eventID,
			ObservedAt: &observedCopy, IngestedAt: &ingestedCopy, ResolutionState: strings.TrimSpace(p.ResolutionState),
			SourceKind: sourceKind, SourceSensorID: sourceSensorID, SourceRule: sourceRule,
			PayloadJSON: payloadJSON, PayloadHash: strings.TrimSpace(p.PayloadHash), Runtime: strings.TrimSpace(p.Runtime),
			EventType: strings.TrimSpace(p.EventType), Signal: strings.TrimSpace(p.Signal),
			MitreTechnique: strings.TrimSpace(p.MitreTechnique), Severity: strings.TrimSpace(p.Severity), Confidence: p.Confidence,
		}
		prepared = append(prepared, preparedRuntimeEvent{
			eventID: eventID, digest: digest, input: input, id: id,
			meta: riskengine.RuntimeEventMeta{
				ClusterID: clusterID, PodUID: podUID, Runtime: strings.TrimSpace(p.Runtime),
				SourceKind: sourceKind, SourceRule: sourceRule, ResolutionState: strings.TrimSpace(p.ResolutionState),
				Syscall: syscall, Severity: strings.TrimSpace(p.Severity), ObservedAt: &observedCopy,
			},
		})
	}
	return prepared, nil
}

// PostRuntimeEventsV2Scoped is the production v2 ingest path after ownership
// middleware. HTTP 200 is a whole-batch acknowledgement: every event has either
// committed its base runtime row or is an exact idempotent replay.
func PostRuntimeEventsV2Scoped(db *gorm.DB) gin.HandlerFunc {
	rescoreMgr := riskengine.NewRuntimeAttackRescoreManager(db)
	return func(c *gin.Context) {
		clusterID, ok := trustedRuntimeCluster(c)
		if !ok {
			return
		}
		payloads, err := bindRuntimeV2Payloads(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_payload"})
			return
		}
		prepared, err := prepareRuntimeV2Batch(clusterID, payloads, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_event"})
			return
		}

		accepted, processed, replayed := 0, 0, 0
		rescore := make([]riskengine.RuntimeEventMeta, 0, len(prepared))
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			for _, p := range prepared {
				claim := models.RuntimeEventIngestClaim{
					ClusterID: clusterID, EventID: p.eventID, PayloadSHA256: p.digest,
					AcceptedAt: time.Now().UTC(),
				}
				insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
				if insert.Error != nil {
					return fmt.Errorf("claim runtime event %s: %w", p.eventID, insert.Error)
				}
				if insert.RowsAffected == 0 {
					var prior models.RuntimeEventIngestClaim
					if err := tx.Where("cluster_id = ? AND event_id = ?", clusterID, p.eventID).First(&prior).Error; err != nil {
						return fmt.Errorf("load runtime event replay claim %s: %w", p.eventID, err)
					}
					if prior.PayloadSHA256 != p.digest {
						return fmt.Errorf("%w: event_id=%s", errRuntimeEventReplayConflict, p.eventID)
					}
					accepted++
					replayed++
					continue
				}

				result, err := rep.ProcessRuntimeEventForIdentity(c.Request.Context(), tx, p.id, p.input)
				if err != nil {
					return fmt.Errorf("persist runtime event %s: %w", p.eventID, err)
				}
				accepted++
				if result != nil {
					processed++
				}
				rescore = append(rescore, p.meta)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, errRuntimeEventReplayConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_event_replay_conflict"})
				return
			}
			log.Printf("[RuntimeEventV2] batch persistence failed cluster=%s: %v", clusterID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime event batch persistence unavailable", "code": "runtime_event_persistence_failed"})
			return
		}

		// Derived rescore scheduling is deliberately post-commit. A rolled-back
		// event must never escape the transaction through an in-memory timer.
		for _, meta := range rescore {
			rescoreMgr.Notify(meta)
		}
		c.JSON(http.StatusOK, runtimeEventResponse{Accepted: accepted, Processed: processed, Replayed: replayed})
	}
}
