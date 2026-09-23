package api

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errRuntimeProducerConflict = errors.New("runtime producer lifecycle conflict")

func runtimeProducerManifestState(decl collection.RuntimeProducerDeclaration, agentState string, gap time.Time, reason string) models.RuntimeProducerState {
	state := collection.RuntimeProducerStarting
	if agentState == collection.RuntimeAgentStopping {
		state = collection.RuntimeProducerStopped
		reason = "agent_stopping"
	} else if !decl.Enabled {
		state = collection.RuntimeProducerDisabled
		reason = "disabled"
	} else if !decl.Authoritative {
		state = collection.RuntimeProducerNonAuthoritative
		reason = "non_authoritative"
	}
	g := gap
	return models.RuntimeProducerState{
		ProducerID: decl.ProducerID,
		SourceKind: decl.SourceKind,
		Enabled: decl.Enabled,
		Authoritative: decl.Authoritative,
		State: state,
		GapSince: &g,
		GapReason: reason,
	}
}

// PostRuntimeProducerManifest persists the complete runtime producer lifecycle for
// one scoped Agent execution session. Missing producers are not inferred disabled:
// the manifest must explicitly declare the bounded producer registry.
func PostRuntimeProducerManifest(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, scoped := middleware.AgentPrincipal(c)
		if !scoped || strings.TrimSpace(principal.ClusterID) == "" || strings.TrimSpace(principal.AgentID) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "scoped agent identity required", "code": "runtime_producer_identity_required"})
			return
		}

		var req collection.RuntimeProducerManifest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_producer_manifest"})
			return
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		req.SessionStartedAt = req.SessionStartedAt.UTC().Truncate(time.Microsecond)
		req.ReportedAt = req.ReportedAt.UTC().Truncate(time.Microsecond)
		if err := req.Validate(now); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_producer_manifest"})
			return
		}
		sort.Slice(req.Producers, func(i, j int) bool { return req.Producers[i].ProducerID < req.Producers[j].ProducerID })

		states := make([]models.RuntimeProducerState, 0, len(req.Producers))
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			for _, decl := range req.Producers {
				base := runtimeProducerManifestState(decl, req.AgentState, req.SessionStartedAt, "startup")
				base.ClusterID = principal.ClusterID
				base.AgentID = principal.AgentID
				base.SessionID = req.SessionID
				base.SessionStartedAt = req.SessionStartedAt
				base.LastManifestAt = req.ReportedAt
				base.LastHeartbeatAt = now

				insert := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "cluster_id"}, {Name: "agent_id"}, {Name: "producer_id"}},
					DoNothing: true,
				}).Create(&base)
				if insert.Error != nil {
					return insert.Error
				}
				if insert.RowsAffected == 1 {
					states = append(states, base)
					continue
				}

				var prior models.RuntimeProducerState
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", principal.ClusterID, principal.AgentID, decl.ProducerID).
					First(&prior).Error; err != nil {
					return err
				}
				if prior.SourceKind != decl.SourceKind {
					return errRuntimeProducerConflict
				}

				leaseExpired := prior.LastHeartbeatAt.IsZero() || now.Before(prior.LastHeartbeatAt) ||
					now.Sub(prior.LastHeartbeatAt) > collection.RuntimeProducerLeaseMaxAge
				producerSilent := prior.State == collection.RuntimeProducerActive &&
					(prior.LastCoverageEnd == nil || prior.LastCoverageEnd.IsZero() || now.Before(*prior.LastCoverageEnd) ||
						now.Sub(*prior.LastCoverageEnd) > collection.RuntimeProducerLeaseMaxAge)

				next := prior
				next.Enabled = decl.Enabled
				next.Authoritative = decl.Authoritative
				next.LastManifestAt = req.ReportedAt
				next.LastHeartbeatAt = now

				if prior.SessionID != req.SessionID {
					if !req.SessionStartedAt.After(prior.SessionStartedAt) {
						return errRuntimeProducerConflict
					}
					reset := runtimeProducerManifestState(decl, req.AgentState, req.SessionStartedAt, "agent_restart")
					next.SessionID = req.SessionID
					next.SessionStartedAt = req.SessionStartedAt
					next.State = reset.State
					// A restart invalidates continuity from the last accepted
					// producer observation, not merely from the new process start.
					// This preserves the full possible loss interval when an
					// in-memory queue disappeared with the previous process.
					if prior.LastCoverageEnd != nil && !prior.LastCoverageEnd.IsZero() && prior.LastCoverageEnd.Before(req.SessionStartedAt) {
						gap := *prior.LastCoverageEnd
						next.GapSince = &gap
					} else {
						next.GapSince = reset.GapSince
					}
					next.GapReason = reset.GapReason
					next.LastCoverageID = ""
					next.LastCoverageEnd = nil
				} else {
					if req.ReportedAt.Before(prior.LastManifestAt) {
						return errRuntimeProducerConflict
					}
					switch {
					case req.AgentState == collection.RuntimeAgentStopping:
						next.State = collection.RuntimeProducerStopped
						if prior.State != collection.RuntimeProducerStopped || prior.GapSince == nil {
							gap := req.ReportedAt
							next.GapSince = &gap
						}
						next.GapReason = "agent_stopping"
					case !decl.Enabled:
						if prior.Enabled || prior.State != collection.RuntimeProducerDisabled {
							gap := req.ReportedAt
							next.GapSince = &gap
						}
						next.State = collection.RuntimeProducerDisabled
						next.GapReason = "disabled"
					case !decl.Authoritative:
						if prior.Authoritative || prior.State != collection.RuntimeProducerNonAuthoritative {
							gap := req.ReportedAt
							next.GapSince = &gap
						}
						next.State = collection.RuntimeProducerNonAuthoritative
						next.GapReason = "non_authoritative"
					case !prior.Enabled || !prior.Authoritative || prior.State == collection.RuntimeProducerDisabled || prior.State == collection.RuntimeProducerStopped || prior.State == collection.RuntimeProducerNonAuthoritative:
						next.State = collection.RuntimeProducerStarting
						gap := req.ReportedAt
						next.GapSince = &gap
						next.GapReason = "enabled"
					case leaseExpired:
						next.State = collection.RuntimeProducerStarting
						gap := prior.LastHeartbeatAt.Add(collection.RuntimeProducerLeaseMaxAge)
						if prior.LastHeartbeatAt.IsZero() || gap.After(now) {
							gap = now
						}
						next.GapSince = &gap
						next.GapReason = "lifecycle_lease_expired"
					case producerSilent:
						next.State = collection.RuntimeProducerStarting
						gap := prior.LastCoverageEnd.Add(collection.RuntimeProducerLeaseMaxAge)
						if gap.After(now) {
							gap = now
						}
						next.GapSince = &gap
						next.GapReason = "producer_silent"
					}
				}

				if err := tx.Model(&prior).Updates(map[string]interface{}{
					"session_id": next.SessionID,
					"session_started_at": next.SessionStartedAt,
					"enabled": next.Enabled,
					"authoritative": next.Authoritative,
					"state": next.State,
					"last_manifest_at": next.LastManifestAt,
					"last_heartbeat_at": next.LastHeartbeatAt,
					"last_coverage_id": next.LastCoverageID,
					"last_coverage_end": next.LastCoverageEnd,
					"gap_since": next.GapSince,
					"gap_reason": next.GapReason,
				}).Error; err != nil {
					return err
				}
				states = append(states, next)
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, errRuntimeProducerConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_producer_conflict"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime producer lifecycle persistence unavailable", "code": "runtime_producer_persistence"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "sessionId": req.SessionID, "states": states})
	}
}
