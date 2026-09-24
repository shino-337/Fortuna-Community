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
	errRuntimeSourceHealthLifecycleRequired = errors.New("runtime producer lifecycle state required")
	errRuntimeSourceHealthInactive          = errors.New("runtime producer is not active for this session")
	errRuntimeSourceHealthConflict          = errors.New("runtime source-health replay or ordering conflict")
)

func PostRuntimeSourceHealth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, scoped := middleware.AgentPrincipal(c)
		if !scoped || strings.TrimSpace(principal.ClusterID) == "" || strings.TrimSpace(principal.AgentID) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "scoped agent identity required", "code": "runtime_source_health_identity_required"})
			return
		}

		var req collection.RuntimeSourceHealthReport
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_source_health"})
			return
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		req.ObservedAt = req.ObservedAt.UTC().Truncate(time.Microsecond)
		req.ValidUntil = req.ValidUntil.UTC().Truncate(time.Microsecond)
		if err := req.Validate(now); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_source_health"})
			return
		}
		if req.Status == collection.RuntimeSourceHealthHealthy && !req.ValidUntil.After(now) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "healthy runtime source-health lease is already expired", "code": "invalid_runtime_source_health"})
			return
		}

		row := models.RuntimeSourceHealth{
			ClusterID:  principal.ClusterID,
			AgentID:    principal.AgentID,
			ProducerID: req.ProducerID,
			SessionID:  req.SessionID,
			SourceKind: req.SourceKind,
			Status:     req.Status,
			ProofKind:  req.ProofKind,
			ObservedAt: req.ObservedAt,
			ValidUntil: req.ValidUntil,
			ReceivedAt: now,
			Reason:     strings.TrimSpace(req.Reason),
		}

		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			var producer models.RuntimeProducerState
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", row.ClusterID, row.AgentID, row.ProducerID).
				First(&producer).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errRuntimeSourceHealthLifecycleRequired
				}
				return err
			}
			if producer.SessionID != row.SessionID || producer.SourceKind != row.SourceKind ||
				!producer.Enabled || !producer.LeaseFresh(now) ||
				producer.State == collection.RuntimeProducerDisabled ||
				producer.State == collection.RuntimeProducerStopped {
				return errRuntimeSourceHealthInactive
			}

			var prior models.RuntimeSourceHealth
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", row.ClusterID, row.AgentID, row.ProducerID).
				First(&prior).Error
			switch {
			case err == nil:
				if row.ObservedAt.Before(prior.ObservedAt) {
					return errRuntimeSourceHealthConflict
				}
				if row.ObservedAt.Equal(prior.ObservedAt) {
					if prior.SessionID != row.SessionID || prior.SourceKind != row.SourceKind ||
						prior.Status != row.Status || prior.ProofKind != row.ProofKind ||
						!prior.ValidUntil.Equal(row.ValidUntil) || prior.Reason != row.Reason {
						return errRuntimeSourceHealthConflict
					}
					return nil
				}
				if row.Status == collection.RuntimeSourceHealthHealthy {
					start := row.ObservedAt
					if prior.Status == collection.RuntimeSourceHealthHealthy &&
						prior.SessionID == row.SessionID &&
						prior.SourceKind == row.SourceKind &&
						prior.ProofKind == row.ProofKind &&
						!row.ObservedAt.After(prior.ValidUntil) {
						if prior.ContinuousSince != nil {
							start = *prior.ContinuousSince
						} else {
							start = prior.ObservedAt
						}
					}
					row.ContinuousSince = &start
				}
				if err := tx.Model(&prior).Updates(map[string]interface{}{
					"session_id": row.SessionID,
					"source_kind": row.SourceKind,
					"status": row.Status,
					"proof_kind": row.ProofKind,
					"observed_at": row.ObservedAt,
					"continuous_since": row.ContinuousSince,
					"valid_until": row.ValidUntil,
					"received_at": row.ReceivedAt,
					"reason": row.Reason,
				}).Error; err != nil {
					return err
				}
			case errors.Is(err, gorm.ErrRecordNotFound):
				if row.Status == collection.RuntimeSourceHealthHealthy {
					start := row.ObservedAt
					row.ContinuousSince = &start
				}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			default:
				return err
			}

			authoritative := row.Status == collection.RuntimeSourceHealthHealthy
			updates := map[string]interface{}{
				"authoritative": authoritative,
				"source_health_status": row.Status,
				"source_health_proof_kind": row.ProofKind,
				"source_health_observed_at": row.ObservedAt,
				"source_health_continuous_since": row.ContinuousSince,
				"source_health_valid_until": row.ValidUntil,
			}
			if !authoritative {
				updates["gap_reason"] = "source_health_failed"
				if producer.GapSince == nil || row.ObservedAt.Before(*producer.GapSince) {
					updates["gap_since"] = row.ObservedAt
				}
			}
			return tx.Model(&models.RuntimeProducerState{}).
				Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", row.ClusterID, row.AgentID, row.ProducerID).
				Updates(updates).Error
		})
		if err != nil {
			switch {
			case errors.Is(err, errRuntimeSourceHealthLifecycleRequired):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_lifecycle_required"})
			case errors.Is(err, errRuntimeSourceHealthInactive):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_producer_inactive"})
			case errors.Is(err, errRuntimeSourceHealthConflict):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_source_health_conflict"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime source-health persistence unavailable", "code": "runtime_source_health_persistence"})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"producerId": row.ProducerID,
			"sessionId": row.SessionID,
			"status": row.Status,
			"validUntil": row.ValidUntil,
		})
	}
}
