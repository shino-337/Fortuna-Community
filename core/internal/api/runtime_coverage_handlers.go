package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errRuntimeCoverageConflict = errors.New("runtime coverage replay or ordering conflict")
	errRuntimeLifecycleRequired = errors.New("runtime producer lifecycle state required")
	errRuntimeProducerInactive = errors.New("runtime producer is not active for this session")
)

// PostRuntimeCoverage accepts evidence only from a scoped Agent principal and a
// current producer lifecycle lease for the same Agent execution session.
func PostRuntimeCoverage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, scoped := middleware.AgentPrincipal(c)
		if !scoped || strings.TrimSpace(principal.ClusterID) == "" || strings.TrimSpace(principal.AgentID) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "scoped agent identity required", "code": "runtime_coverage_identity_required"})
			return
		}

		var req collection.RuntimeCoverage
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_coverage"})
			return
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		req.WindowStart = req.WindowStart.UTC().Truncate(time.Microsecond)
		req.WindowEnd = req.WindowEnd.UTC().Truncate(time.Microsecond)
		if err := req.Validate(now); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_coverage"})
			return
		}

		row := models.RuntimeCoverage{
			ClusterID: principal.ClusterID,
			AgentID: principal.AgentID,
			ProducerID: req.ProducerID,
			SessionID: req.SessionID,
			CoverageID: req.ID,
			SourceKind: req.SourceKind,
			Status: req.Status,
			WindowStart: req.WindowStart,
			WindowEnd: req.WindowEnd,
			ReceivedAt: now,
			Emitted: req.Emitted,
			Delivered: req.Delivered,
			Dropped: req.Dropped,
			Invalid: req.Invalid,
			Errors: req.Errors,
			Reason: req.Reason,
		}

		replay := false
		var producer models.RuntimeProducerState
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", row.ClusterID, row.AgentID, row.ProducerID).
				First(&producer).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errRuntimeLifecycleRequired
				}
				return err
			}
			if producer.SessionID != row.SessionID || producer.SourceKind != row.SourceKind ||
				row.WindowStart.Before(producer.SessionStartedAt) {
				return errRuntimeProducerInactive
			}
			if !producer.LeaseFresh(now) || !producer.Enabled ||
				producer.State == collection.RuntimeProducerDisabled ||
				producer.State == collection.RuntimeProducerStopped {
				return errRuntimeProducerInactive
			}
			// Non-authoritative producers may still report complete observation
			// windows for operational visibility. They must never establish
			// continuity or absence-eligible evidence.
			// Serialize first report and subsequent windows under the lifecycle
			// row lock. A new Agent session is an explicit continuity boundary.
			candidate := row
			if candidate.Status == "complete" && producer.Authoritative {
				start := candidate.WindowStart
				candidate.ContinuousSince = &start
			}
			insert := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "cluster_id"}, {Name: "agent_id"}, {Name: "producer_id"}},
				DoNothing: true,
			}).Create(&candidate)
			if insert.Error != nil {
				return insert.Error
			}
			if insert.RowsAffected == 1 {
				row = candidate
			} else {
				var prior models.RuntimeCoverage
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", row.ClusterID, row.AgentID, row.ProducerID).
					First(&prior).Error; err != nil {
					return err
				}

				if prior.CoverageID == row.CoverageID {
					if prior.SessionID != row.SessionID || prior.SourceKind != row.SourceKind || prior.Status != row.Status ||
						!prior.WindowStart.Equal(row.WindowStart) || !prior.WindowEnd.Equal(row.WindowEnd) ||
						prior.Emitted != row.Emitted || prior.Delivered != row.Delivered ||
						prior.Dropped != row.Dropped || prior.Invalid != row.Invalid ||
						prior.Errors != row.Errors || prior.Reason != row.Reason {
						return errRuntimeCoverageConflict
					}
					row = prior
					replay = true
				} else if prior.SessionID != row.SessionID {
					// Restart/new execution session: old receipt is retained only as
					// overwritten history and cannot extend continuity.
					row.ContinuousSince = nil
					if row.Status == "complete" && producer.Authoritative {
						start := row.WindowStart
						row.ContinuousSince = &start
					}
				} else {
					if prior.SourceKind != row.SourceKind ||
						!row.WindowEnd.After(prior.WindowEnd) ||
						row.WindowStart.Before(prior.WindowEnd) {
						return errRuntimeCoverageConflict
					}
					row.ContinuousSince = nil
					if row.Status == "complete" && producer.Authoritative {
						start := row.WindowStart
						if producer.State == collection.RuntimeProducerActive && prior.Status == "complete" && prior.ContinuousSince != nil && row.WindowStart.Equal(prior.WindowEnd) {
							start = *prior.ContinuousSince
						}
						row.ContinuousSince = &start
					}
				}

				if !replay {
					if err := tx.Model(&prior).Updates(map[string]interface{}{
						"session_id": row.SessionID,
						"coverage_id": row.CoverageID,
						"source_kind": row.SourceKind,
						"status": row.Status,
						"window_start": row.WindowStart,
						"window_end": row.WindowEnd,
						"received_at": row.ReceivedAt,
						"continuous_since": row.ContinuousSince,
						"emitted": row.Emitted,
						"delivered": row.Delivered,
						"dropped": row.Dropped,
						"invalid": row.Invalid,
						"errors": row.Errors,
						"reason": row.Reason,
					}).Error; err != nil {
						return err
					}
				}
			}

			if !replay {
				producer.LastCoverageID = row.CoverageID
				end := row.WindowEnd
				producer.LastCoverageEnd = &end
				if row.Status == "complete" && producer.Authoritative {
					producer.State = collection.RuntimeProducerActive
					producer.GapSince = nil
					producer.GapReason = ""
				} else if producer.Authoritative {
					producer.State = collection.RuntimeProducerDegraded
					if producer.GapSince == nil {
						gap := row.WindowStart
						producer.GapSince = &gap
					}
					producer.GapReason = "coverage_failed"
				} else {
					producer.State = collection.RuntimeProducerNonAuthoritative
					if producer.GapSince == nil {
						gap := row.WindowStart
						producer.GapSince = &gap
					}
					producer.GapReason = "non_authoritative"
				}
				if err := tx.Model(&models.RuntimeProducerState{}).
					Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", producer.ClusterID, producer.AgentID, producer.ProducerID).
					Updates(map[string]interface{}{
						"state": producer.State,
						"last_coverage_id": producer.LastCoverageID,
						"last_coverage_end": producer.LastCoverageEnd,
						"gap_since": producer.GapSince,
						"gap_reason": producer.GapReason,
					}).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			switch {
			case errors.Is(err, errRuntimeCoverageConflict):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_coverage_conflict"})
			case errors.Is(err, errRuntimeLifecycleRequired):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_lifecycle_required"})
			case errors.Is(err, errRuntimeProducerInactive):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_producer_inactive"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime coverage persistence unavailable", "code": "runtime_coverage_persistence"})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"replay": replay,
			"coverage": row,
			"producer": producer,
			"effectiveStatus": row.EffectiveStatus(&producer, time.Now().UTC()),
		})
	}
}
