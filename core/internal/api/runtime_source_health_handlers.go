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
	errRuntimeSourceHealthConflict = errors.New("runtime source health replay or ordering conflict")
	errRuntimeSourceHealthInactive = errors.New("runtime source health producer is not active for this session")
)

func runtimeSourceHealthReceiptMatches(receipt models.RuntimeSourceHealthReceipt, row models.RuntimeSourceHealthReceipt) bool {
	return receipt.ClusterID == row.ClusterID &&
		receipt.AgentID == row.AgentID &&
		receipt.ProducerID == row.ProducerID &&
		receipt.SessionID == row.SessionID &&
		receipt.HealthID == row.HealthID &&
		receipt.SourceKind == row.SourceKind &&
		receipt.ProbeKind == row.ProbeKind &&
		receipt.SourceInstanceID == row.SourceInstanceID &&
		receipt.Status == row.Status &&
		receipt.ObservedAt.Equal(row.ObservedAt) &&
		receipt.Reason == row.Reason
}

// PostRuntimeSourceHealth accepts an approved upstream-health observation from
// the exact scoped Agent producer/session. Source health is independent of event
// emptiness and reader activity; it does not renew the producer lifecycle lease.
func PostRuntimeSourceHealth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, scoped := middleware.AgentPrincipal(c)
		if !scoped || strings.TrimSpace(principal.ClusterID) == "" || strings.TrimSpace(principal.AgentID) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "scoped agent identity required", "code": "runtime_source_health_identity_required"})
			return
		}

		var req collection.RuntimeSourceHealth
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_source_health"})
			return
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		req.ObservedAt = req.ObservedAt.UTC().Truncate(time.Microsecond)
		if err := req.Validate(now); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_runtime_source_health"})
			return
		}

		receipt := models.RuntimeSourceHealthReceipt{
			ClusterID:        principal.ClusterID,
			AgentID:          principal.AgentID,
			ProducerID:       req.ProducerID,
			SessionID:        req.SessionID,
			HealthID:         req.ID,
			SourceKind:       req.SourceKind,
			ProbeKind:        req.ProbeKind,
			SourceInstanceID: req.SourceInstanceID,
			Status:           req.Status,
			ObservedAt:       req.ObservedAt,
			ReceivedAt:       now,
			Reason:           req.Reason,
		}

		replay := false
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			var producer models.RuntimeProducerState
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", principal.ClusterID, principal.AgentID, req.ProducerID).
				First(&producer).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errRuntimeLifecycleRequired
				}
				return err
			}
			if producer.SessionID != req.SessionID || producer.SourceKind != req.SourceKind ||
				req.ObservedAt.Before(producer.SessionStartedAt) ||
				!producer.LeaseFresh(now) || !producer.Enabled ||
				producer.State == collection.RuntimeProducerDisabled ||
				producer.State == collection.RuntimeProducerStopped {
				return errRuntimeSourceHealthInactive
			}

			var historical models.RuntimeSourceHealthReceipt
			historyErr := tx.Where(
				"cluster_id = ? AND agent_id = ? AND producer_id = ? AND session_id = ? AND health_id = ?",
				receipt.ClusterID, receipt.AgentID, receipt.ProducerID, receipt.SessionID, receipt.HealthID,
			).First(&historical).Error
			if historyErr == nil {
				if !runtimeSourceHealthReceiptMatches(historical, receipt) {
					return errRuntimeSourceHealthConflict
				}
				replay = true
				return nil
			}
			if !errors.Is(historyErr, gorm.ErrRecordNotFound) {
				return historyErr
			}

			if producer.SourceHealthObservedAt != nil && !req.ObservedAt.After(*producer.SourceHealthObservedAt) {
				return errRuntimeSourceHealthConflict
			}

			breakContinuity := false
			gapReason := ""
			var healthySince *time.Time
			authoritative := false

			if req.Status == collection.RuntimeSourceHealthHealthy {
				authoritative = true
				canExtend := producer.Authoritative &&
					producer.SourceHealthStatus == collection.RuntimeSourceHealthHealthy &&
					producer.SourceHealthObservedAt != nil &&
					producer.SourceHealthHealthySince != nil &&
					producer.SourceHealthProbeKind == req.ProbeKind &&
					producer.SourceInstanceID == req.SourceInstanceID &&
					req.ObservedAt.Sub(*producer.SourceHealthObservedAt) <= collection.RuntimeSourceHealthMaxGap
				if canExtend {
					start := *producer.SourceHealthHealthySince
					healthySince = &start
				} else {
					start := req.ObservedAt
					healthySince = &start
					if producer.SourceHealthObservedAt != nil {
						breakContinuity = true
						if producer.SourceInstanceID != "" && producer.SourceInstanceID != req.SourceInstanceID {
							gapReason = "source_restart"
						} else {
							gapReason = "source_health_gap"
						}
					}
				}
			} else {
				breakContinuity = true
				gapReason = "source_health_failed"
			}

			gapSince := producer.GapSince
			if breakContinuity {
				gap := req.ObservedAt
				if producer.SourceHealthObservedAt != nil && !producer.SourceHealthObservedAt.After(req.ObservedAt) {
					gap = *producer.SourceHealthObservedAt
				}
				if gapSince == nil || gap.Before(*gapSince) {
					gapSince = &gap
				}
				// Old complete coverage cannot regain authority after an upstream
				// failure/restart merely because a later health report is healthy.
				if err := tx.Model(&models.RuntimeCoverage{}).
					Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", producer.ClusterID, producer.AgentID, producer.ProducerID).
					Update("continuous_since", nil).Error; err != nil {
					return err
				}
			}

			if err := tx.Model(&models.RuntimeProducerState{}).
				Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", producer.ClusterID, producer.AgentID, producer.ProducerID).
				Updates(map[string]interface{}{
					"authoritative": authoritative,
					"source_health_id": req.ID,
					"source_health_status": req.Status,
					"source_health_probe_kind": req.ProbeKind,
					"source_instance_id": req.SourceInstanceID,
					"source_health_observed_at": req.ObservedAt,
					"source_health_healthy_since": healthySince,
					"source_health_reason": req.Reason,
					"gap_since": gapSince,
					"gap_reason": func() string {
						if gapReason != "" {
							return gapReason
						}
						return producer.GapReason
					}(),
				}).Error; err != nil {
				return err
			}
			return tx.Create(&receipt).Error
		})
		if err != nil {
			switch {
			case errors.Is(err, errRuntimeLifecycleRequired):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_lifecycle_required"})
			case errors.Is(err, errRuntimeSourceHealthInactive):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_source_health_inactive"})
			case errors.Is(err, errRuntimeSourceHealthConflict):
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_source_health_conflict"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime source health persistence unavailable", "code": "runtime_source_health_persistence"})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "replay": replay, "healthId": req.ID})
	}
}
