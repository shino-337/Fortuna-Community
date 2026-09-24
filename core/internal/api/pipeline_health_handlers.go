package api

import (
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

// PipelineHealthResponse is the response for GET /api/v1/monitoring/pipeline-health.
// It provides visibility into each layer of the Unified Risk Pipeline.
type PipelineHealthResponse struct {
	Layer1 PipelineLayer1 `json:"layer1"`
	Layer2 PipelineLayer2 `json:"layer2"`
	Layer3 PipelineLayer3 `json:"layer3"`
	Layer4 PipelineLayer4 `json:"layer4"`
}

// PipelineLayer1 — Fact Discovery (PCE + Risk Engine)
type PipelineLayer1 struct {
	LastPceEval        *time.Time `json:"lastPceEval"`
	LastRiskEngineEval *time.Time `json:"lastRiskEngineEval"`
	InsightCount       int64      `json:"insightCount"`
	FreshnessMinutes   int64      `json:"freshnessMinutes"`
	Status             string     `json:"status"` // healthy | degraded | stale | unknown
}

// PipelineLayer2 — Runtime Enrichment (CSC state promotion + AttackStepInference)
type PipelineLayer2 struct {
	LastStateChange      *time.Time `json:"lastStateChange"`
	ActivePromotionRules int64      `json:"activePromotionRules"`
	ExploitedCapCount    int64      `json:"exploitedCapCount"`
	FreshnessMinutes     int64      `json:"freshnessMinutes"`
	Status               string     `json:"status"` // healthy | degraded | stale | unknown
}

// PipelineLayer3 — Path Analysis (Attack Path Builder)
type PipelineLayer3 struct {
	LastPathComputation *time.Time `json:"lastPathComputation"`
	TotalPaths          int64      `json:"totalPaths"`
	CriticalPaths       int64      `json:"criticalPaths"`
	FreshnessMinutes    int64      `json:"freshnessMinutes"`
	Status              string     `json:"status"` // healthy | degraded | stale | unknown
}

// PipelineLayer4 — Unified Scoring
type PipelineLayer4 struct {
	LastScoreCalc    *time.Time `json:"lastScoreCalc"`
	ResourcesScored  int64      `json:"resourcesScored"`
	AvgScore         float64    `json:"avgScore"`
	V3Resources      int64      `json:"v3Resources"`
	FreshnessMinutes int64      `json:"freshnessMinutes"`
	Status           string     `json:"status"` // healthy | degraded | stale | unknown
}

type freshnessPolicy struct {
	HealthyMinutes  int64
	DegradedMinutes int64
}

func getEnvInt64(key string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(os.Getenv(key), 10, 64)
	if err != nil {
		return fallback
	}
	if parsed <= 0 {
		return fallback
	}
	return parsed
}

func getPolicyByLayer(layer string) freshnessPolicy {
	switch layer {
	case "layer1":
		return freshnessPolicy{
			HealthyMinutes:  getEnvInt64("PIPELINE_HEALTH_LAYER1_HEALTHY_MINUTES", 30),
			DegradedMinutes: getEnvInt64("PIPELINE_HEALTH_LAYER1_DEGRADED_MINUTES", 180),
		}
	case "layer2":
		return freshnessPolicy{
			HealthyMinutes:  getEnvInt64("PIPELINE_HEALTH_LAYER2_HEALTHY_MINUTES", 30),
			DegradedMinutes: getEnvInt64("PIPELINE_HEALTH_LAYER2_DEGRADED_MINUTES", 180),
		}
	case "layer3":
		return freshnessPolicy{
			HealthyMinutes:  getEnvInt64("PIPELINE_HEALTH_LAYER3_HEALTHY_MINUTES", 30),
			DegradedMinutes: getEnvInt64("PIPELINE_HEALTH_LAYER3_DEGRADED_MINUTES", 180),
		}
	default:
		return freshnessPolicy{
			HealthyMinutes:  getEnvInt64("PIPELINE_HEALTH_LAYER4_HEALTHY_MINUTES", 30),
			DegradedMinutes: getEnvInt64("PIPELINE_HEALTH_LAYER4_DEGRADED_MINUTES", 180),
		}
	}
}

func calcFreshness(last *time.Time, p freshnessPolicy) (int64, string) {
	if last == nil {
		return -1, "unknown"
	}
	minutes := int64(time.Since(*last).Minutes())
	if minutes <= p.HealthyMinutes {
		return minutes, "healthy"
	}
	if minutes <= p.DegradedMinutes {
		return minutes, "degraded"
	}
	return minutes, "stale"
}

func latestQueryTime(q *gorm.DB, column string) (*time.Time, error) {
	var values []time.Time
	if err := q.
		Where(column + " IS NOT NULL").
		Order(column + " DESC").
		Limit(1).
		Pluck(column, &values).Error; err != nil {
		return nil, err
	}
	if len(values) == 0 || values[0].IsZero() {
		return nil, nil
	}
	t := values[0]
	return &t, nil
}

func pipelineHealthClusterFilter(db *gorm.DB, c *gin.Context) ([]string, bool) {
	if clusterID := strings.TrimSpace(c.Query("cluster_id")); clusterID != "" {
		clusterID = NormalizeClusterID(db, clusterID)
		if !middleware.ClusterAllowed(c, clusterID) {
			middleware.AbortClusterScopeDenied(db, c, clusterID)
			return nil, false
		}
		return []string{clusterID}, true
	}
	if scoped, restricted := middleware.ScopedClusterIDs(c); restricted {
		out := make([]string, 0, len(scoped))
		for _, id := range scoped {
			if id = strings.TrimSpace(id); id != "" {
				out = append(out, NormalizeClusterID(db, id))
			}
		}
		return out, true
	}
	return nil, true
}

// GetPipelineHealth returns the current health and activity of each pipeline layer.
// GET /api/v1/monitoring/pipeline-health
func GetPipelineHealth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := db.WithContext(c.Request.Context())
		resp := PipelineHealthResponse{}
		clusterIDs, ok := pipelineHealthClusterFilter(db, c)
		if !ok {
			return
		}
		scoped := func(q *gorm.DB, column string) *gorm.DB {
			if len(clusterIDs) > 0 {
				q = q.Where(column+" IN ?", clusterIDs)
			}
			return q
		}
		fail := func(err error, code, message string) bool {
			if err == nil {
				return false
			}
			respondDataUnavailable(c, code, message)
			return true
		}

		// --- Layer 1 ---
		lastCapUpdate, err := latestQueryTime(scoped(db.Model(&models.PodCapability{}), "cluster_id"), "updated_at")
		if fail(err, "pipeline_health_layer1_capabilities_unavailable", "Pipeline capability freshness could not be loaded") {
			return
		}
		resp.Layer1.LastPceEval = lastCapUpdate

		lastInsightUpdate, err := latestQueryTime(scoped(db.Model(&models.Insight{}).
			Where("insight_type = 'rbac' AND deleted_at IS NULL"), "cluster_id"), "updated_at")
		if fail(err, "pipeline_health_layer1_insights_unavailable", "Pipeline insight freshness could not be loaded") {
			return
		}
		resp.Layer1.LastRiskEngineEval = lastInsightUpdate
		if fail(scoped(db.Model(&models.Insight{}).
			Where("status = 'active' AND deleted_at IS NULL"), "cluster_id").
			Count(&resp.Layer1.InsightCount).Error,
			"pipeline_health_layer1_counts_unavailable", "Pipeline insight counts could not be loaded") {
			return
		}
		resp.Layer1.FreshnessMinutes, resp.Layer1.Status = calcFreshness(resp.Layer1.LastPceEval, getPolicyByLayer("layer1"))

		// --- Layer 2 ---
		lastStateChange, err := latestQueryTime(scoped(db.Model(&models.PodCapability{}).
			Where("state != 'detected'"), "cluster_id"), "updated_at")
		if fail(err, "pipeline_health_layer2_state_unavailable", "Pipeline capability state could not be loaded") {
			return
		}
		resp.Layer2.LastStateChange = lastStateChange
		if db.Migrator().HasTable("promotion_rules") {
			if fail(db.Model(&models.PromotionRule{}).Count(&resp.Layer2.ActivePromotionRules).Error,
				"pipeline_health_promotion_rules_unavailable", "Promotion rule metrics could not be loaded") {
				return
			}
		}
		if fail(scoped(db.Model(&models.PodCapability{}).
			Where("state IN ('exploited', 'chained')"), "cluster_id").
			Count(&resp.Layer2.ExploitedCapCount).Error,
			"pipeline_health_layer2_counts_unavailable", "Pipeline exploited capability counts could not be loaded") {
			return
		}
		resp.Layer2.FreshnessMinutes, resp.Layer2.Status = calcFreshness(resp.Layer2.LastStateChange, getPolicyByLayer("layer2"))

		// --- Layer 3 ---
		if db.Migrator().HasTable("attack_paths") {
			activePaths := func() *gorm.DB {
				q := db.Model(&models.AttackPath{}).
					Joins("INNER JOIN pods ON pods.cluster_id = attack_paths.cluster_id AND pods.uid = attack_paths.pod_uid AND pods.deleted_at IS NULL")
				if len(clusterIDs) > 0 {
					q = q.Where("attack_paths.cluster_id IN ?", clusterIDs)
				}
				return q
			}
			lastPath, err := latestQueryTime(activePaths(), "attack_paths.updated_at")
			if fail(err, "pipeline_health_layer3_freshness_unavailable", "Attack path freshness could not be loaded") {
				return
			}
			resp.Layer3.LastPathComputation = lastPath
			if fail(activePaths().Where("attack_paths.total_risk >= 0.7").Count(&resp.Layer3.TotalPaths).Error,
				"pipeline_health_layer3_counts_unavailable", "Attack path counts could not be loaded") {
				return
			}
			if fail(activePaths().Where("attack_paths.total_risk >= 9.0").Count(&resp.Layer3.CriticalPaths).Error,
				"pipeline_health_layer3_critical_unavailable", "Critical attack path counts could not be loaded") {
				return
			}
		}
		resp.Layer3.FreshnessMinutes, resp.Layer3.Status = calcFreshness(resp.Layer3.LastPathComputation, getPolicyByLayer("layer3"))

		// --- Layer 4 ---
		lastScore, err := latestQueryTime(scoped(db.Model(&models.RiskScore{}).
			Where("deleted_at IS NULL"), "cluster_id"), "calculated_at")
		if fail(err, "pipeline_health_layer4_freshness_unavailable", "Risk score freshness could not be loaded") {
			return
		}
		resp.Layer4.LastScoreCalc = lastScore
		if fail(scoped(db.Model(&models.RiskScore{}).Where("deleted_at IS NULL"), "cluster_id").
			Count(&resp.Layer4.ResourcesScored).Error,
			"pipeline_health_layer4_counts_unavailable", "Risk score counts could not be loaded") {
			return
		}
		var avgScore sql.NullFloat64
		if fail(scoped(db.Model(&models.RiskScore{}).Where("deleted_at IS NULL"), "cluster_id").
			Select("AVG(total_score)").Scan(&avgScore).Error,
			"pipeline_health_layer4_average_unavailable", "Average risk score could not be loaded") {
			return
		}
		if avgScore.Valid {
			resp.Layer4.AvgScore = avgScore.Float64
		}
		if fail(scoped(db.Model(&models.RiskScore{}).
			Where("scorer_version = 'v3' AND deleted_at IS NULL"), "cluster_id").
			Count(&resp.Layer4.V3Resources).Error,
			"pipeline_health_layer4_v3_unavailable", "V3 risk score counts could not be loaded") {
			return
		}
		resp.Layer4.FreshnessMinutes, resp.Layer4.Status = calcFreshness(resp.Layer4.LastScoreCalc, getPolicyByLayer("layer4"))

		c.JSON(http.StatusOK, gin.H{"dataStatus": "available", "data": resp})
	}
}
