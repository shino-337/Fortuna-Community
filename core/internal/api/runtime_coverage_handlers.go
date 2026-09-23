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

var errRuntimeCoverageConflict = errors.New("runtime coverage replay or ordering conflict")

// PostRuntimeCoverage accepts evidence only from a scoped Agent principal. Coverage
// is producer-specific and never inferred from event silence.
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
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			// Close the concurrent first-report race without an unlocked
			// SELECT-then-INSERT sequence. One transaction creates the producer row;
			// concurrent contenders wait on the unique key, then arbitrate against
			// the committed row under FOR UPDATE.
			candidate := row
			if candidate.Status == "complete" {
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
				return nil
			}

			var prior models.RuntimeCoverage
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", row.ClusterID, row.AgentID, row.ProducerID).
				First(&prior).Error; err != nil {
				return err
			}

			if prior.CoverageID == row.CoverageID {
				if prior.SourceKind != row.SourceKind || prior.Status != row.Status ||
					!prior.WindowStart.Equal(row.WindowStart) || !prior.WindowEnd.Equal(row.WindowEnd) ||
					prior.Emitted != row.Emitted || prior.Delivered != row.Delivered ||
					prior.Dropped != row.Dropped || prior.Invalid != row.Invalid ||
					prior.Errors != row.Errors || prior.Reason != row.Reason {
					return errRuntimeCoverageConflict
				}
				row = prior
				replay = true
				return nil
			}

			// Producer identity is permanently bound to its source kind. New windows
			// must be monotonic and may be adjacent or gapped, never overlapping.
			if prior.SourceKind != row.SourceKind ||
				!row.WindowEnd.After(prior.WindowEnd) ||
				row.WindowStart.Before(prior.WindowEnd) {
				return errRuntimeCoverageConflict
			}

			row.ContinuousSince = nil
			if row.Status == "complete" {
				start := row.WindowStart
				if prior.Status == "complete" && prior.ContinuousSince != nil && row.WindowStart.Equal(prior.WindowEnd) {
					start = *prior.ContinuousSince
				}
				row.ContinuousSince = &start
			}

			return tx.Model(&prior).Updates(map[string]interface{}{
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
			}).Error
		})
		if err != nil {
			if errors.Is(err, errRuntimeCoverageConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "runtime_coverage_conflict"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime coverage persistence unavailable", "code": "runtime_coverage_persistence"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"replay": replay,
			"coverage": row,
			"effectiveStatus": row.EffectiveStatus(time.Now().UTC()),
		})
	}
}
