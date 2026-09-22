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

// PostRuntimeCoverage records a producer window only when it is bound to a scoped
// Agent principal. Event silence alone never creates coverage.
func PostRuntimeCoverage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, scoped := middleware.AgentPrincipal(c)
		if !scoped || strings.TrimSpace(principal.ClusterID) == "" || strings.TrimSpace(principal.AgentID) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "scoped agent identity required", "code": "runtime_coverage_identity_required"})
			return
		}
		var req collection.RuntimeCoverage
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		now := time.Now().UTC()
		req.WindowStart = req.WindowStart.UTC().Truncate(time.Microsecond)
		req.WindowEnd = req.WindowEnd.UTC().Truncate(time.Microsecond)
		if err := req.Validate(now); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		row := models.RuntimeCoverage{
			ClusterID: principal.ClusterID, AgentID: principal.AgentID,
			ProducerID: req.ProducerID, CoverageID: req.ID, SourceKind: req.SourceKind,
			Status: req.Status, WindowStart: req.WindowStart, WindowEnd: req.WindowEnd,
			ReceivedAt: now.Truncate(time.Microsecond), Emitted: req.Emitted,
			Delivered: req.Delivered, Dropped: req.Dropped, Invalid: req.Invalid,
			Reason: req.Reason,
		}
		replay := false
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			var prior models.RuntimeCoverage
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", row.ClusterID, row.AgentID, row.ProducerID).
				First(&prior).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Create(&row).Error
			}
			if err != nil {
				return err
			}
			if prior.CoverageID == row.CoverageID {
				if prior.SourceKind != row.SourceKind || prior.Status != row.Status ||
					!prior.WindowStart.Equal(row.WindowStart) || !prior.WindowEnd.Equal(row.WindowEnd) ||
					prior.Emitted != row.Emitted || prior.Delivered != row.Delivered ||
					prior.Dropped != row.Dropped || prior.Invalid != row.Invalid || prior.Reason != row.Reason {
					return errRuntimeCoverageConflict
				}
				row = prior
				replay = true
				return nil
			}
			if !row.WindowEnd.After(prior.WindowEnd) {
				return errRuntimeCoverageConflict
			}
			return tx.Model(&prior).Updates(map[string]interface{}{
				"coverage_id": row.CoverageID, "source_kind": row.SourceKind, "status": row.Status,
				"window_start": row.WindowStart, "window_end": row.WindowEnd, "received_at": row.ReceivedAt,
				"emitted": row.Emitted, "delivered": row.Delivered, "dropped": row.Dropped,
				"invalid": row.Invalid, "reason": row.Reason,
			}).Error
		})
		if err != nil {
			if errors.Is(err, errRuntimeCoverageConflict) {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime coverage persistence unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true, "replay": replay, "coverage": row,
			"effectiveStatus": row.EffectiveStatus(time.Now().UTC()),
		})
	}
}
