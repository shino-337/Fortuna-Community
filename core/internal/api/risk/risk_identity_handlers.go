package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	riskpkg "github.com/fortuna/core/pkg/risk"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func resolvedRiskPodIdentity(c *gin.Context) (resourceidentity.Identity, bool) {
	uid := strings.TrimSpace(c.Param("uid"))
	if uid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource uid is required"})
		return resourceidentity.Identity{}, false
	}
	clusterID, ok := middleware.ResolvedPodClusterID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolved pod cluster is required"})
		return resourceidentity.Identity{}, false
	}
	id, err := resourceidentity.New(clusterID, uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return resourceidentity.Identity{}, false
	}
	return id, true
}

// GetRiskScoreForPodIdentity returns only the V3 score for the Pod identity
// resolved by RequirePodUIDClusterScope. Query parameters cannot override the
// authorization identity selected by middleware.
func GetRiskScoreForPodIdentity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := resolvedRiskPodIdentity(c)
		if !ok {
			return
		}
		var score models.RiskScore
		tx := db.WithContext(c.Request.Context()).
			Where("cluster_id = ? AND resource_type = ? AND resource_uid = ? AND LOWER(TRIM(COALESCE(scorer_version, ''))) = ? AND deleted_at IS NULL", id.ClusterID, "pod", id.ResourceUID, "v3").
			Order("calculated_at DESC, id DESC").
			Limit(1).
			Find(&score)
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": tx.Error.Error()})
			return
		}
		if tx.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Risk score not found"})
			return
		}

		raw, err := json.Marshal(score)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(raw, &payload); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		payload["final_level"] = riskpkg.DeriveFinalLevelFromScore(score.TotalScore)
		if sev := strings.TrimSpace(score.HighestSeverity); sev != "" {
			payload["severity_hint"] = strings.ToLower(sev)
		}
		if bd := riskpkg.ParseBreakdownFromFactorsJSON(score.Factors); len(bd) > 0 {
			payload["breakdown"] = bd
			payload["drivers"] = bd
		}
		payload["final_score"] = score.TotalScore
		payload["risk"] = gin.H{"score": score.TotalScore, "level": payload["final_level"]}
		payload["meta"] = gin.H{"last_updated_at": score.CalculatedAt.UTC().Format(time.RFC3339)}
		delete(payload, "priorityLevel")
		delete(payload, "priority_level")
		if includeLegacyRiskRootFields() {
			payload["legacy"] = gin.H{"highest_severity": score.HighestSeverity, "priority_level": score.PriorityLevel}
		}
		c.JSON(http.StatusOK, payload)
	}
}

// CalculateRiskScoreForPodIdentity calculates and saves one canonical Pod V3
// score without allowing a UID-only lookup inside the scorer.
func CalculateRiskScoreForPodIdentity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("mode", "v3"))); mode != "" && mode != "v3" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "only mode=v3 is supported"})
			return
		}
		id, ok := resolvedRiskPodIdentity(c)
		if !ok {
			return
		}
		scorer := riskpkg.NewUnifiedScorerV3(db)
		score, err := scorer.CalculateScoreV3ForIdentity(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := scorer.SaveScoreV3ForIdentity(c.Request.Context(), id, score); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"mode": "v3", "v3": score})
	}
}

type riskPodIdentityRow struct {
	ClusterID   string `gorm:"column:cluster_id"`
	ResourceUID string `gorm:"column:resource_uid"`
}

func (s analyticsScope) syncPodIdentities(db *gorm.DB) ([]resourceidentity.Identity, error) {
	query := s.apply(db.Model(&models.Insight{}), "cluster_id").
		Where("LOWER(TRIM(resource_type)) = ?", "pod").
		Where("status IN ?", []string{"active", "acknowledged"}).
		Where("TRIM(cluster_id) != '' AND TRIM(resource_uid) != ''").
		Select("cluster_id, resource_uid").
		Distinct().
		Order("cluster_id, resource_uid")
	var rows []riskPodIdentityRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]resourceidentity.Identity, 0, len(rows))
	for _, row := range rows {
		id, err := resourceidentity.New(row.ClusterID, row.ResourceUID)
		if err != nil {
			return nil, fmt.Errorf("invalid persisted pod identity %q/%q: %w", row.ClusterID, row.ResourceUID, err)
		}
		out = append(out, id)
	}
	return out, nil
}

func syncSelectedRiskScoreIdentities(ctx context.Context, db *gorm.DB, ids []resourceidentity.Identity) {
	scorer := riskpkg.NewUnifiedScorerV3(db)
	for _, id := range ids {
		select {
		case <-ctx.Done():
			return
		default:
		}
		score, err := scorer.CalculateScoreV3ForIdentity(ctx, id)
		if err != nil {
			continue
		}
		_ = scorer.SaveScoreV3ForIdentity(ctx, id, score)
	}
}

// SyncRiskScoresByIdentity recalculates only cluster-qualified Pod resources.
func SyncRiskScoresByIdentity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, authorized := resolveAnalyticsScope(db, c)
		if !authorized {
			return
		}
		if mode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("mode", "v3"))); mode != "" && mode != "v3" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "only mode=v3 is supported"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		ids, err := scope.syncPodIdentities(db.WithContext(ctx))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to select resources for sync"})
			return
		}
		if len(ids) == 0 {
			c.JSON(http.StatusOK, gin.H{"message": "No Pod resources with active insights to sync", "resources": 0})
			return
		}
		go func(selected []resourceidentity.Identity) {
			bg, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			syncSelectedRiskScoreIdentities(bg, db, selected)
		}(append([]resourceidentity.Identity(nil), ids...))
		c.JSON(http.StatusAccepted, gin.H{"message": "Risk score sync started (mode=v3)", "resources": len(ids), "mode": "v3"})
	}
}
