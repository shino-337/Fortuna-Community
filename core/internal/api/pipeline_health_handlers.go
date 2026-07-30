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

func timeFromNull(nt sql.NullTime) *time.Time {
	if !nt.Valid || nt.Time.IsZero() {
		return nil
	}
	t := nt.Time
	return &t
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
		resp := PipelineHealthResponse{}
		clusterIDs, ok := pipelineHealthClusterFilter(db, c)
		if !ok {
			return
		}

		// --- Layer 1 ---
		var lastCapUpdate, lastInsightUpdate sql.NullTime
		db.Model(&models.PodCapability{}).Select("MAX(updated_at)").Scan(&lastCapUpdate)
		resp.Layer1.LastPceEval = timeFromNull(lastCapUpdate)

		db.Model(&models.Insight{}).
			Where("insight_type = 'rbac' AND deleted_at IS NULL").
			Select("MAX(updated_at)").Scan(&lastInsightUpdate)
		resp.Layer1.LastRiskEngineEval = timeFromNull(lastInsightUpdate)
		db.Model(&models.Insight{}).
			Where("status = 'active' AND deleted_at IS NULL").
			Count(&resp.Layer1.InsightCount)
		resp.Layer1.FreshnessMinutes, resp.Layer1.Status = calcFreshness(resp.Layer1.LastPceEval, getPolicyByLayer("layer1"))

		// --- Layer 2 ---
		var lastStateChange sql.NullTime
		db.Model(&models.PodCapability{}).
			Where("state != 'detected'").
			Select("MAX(updated_at)").Scan(&lastStateChange)
		resp.Layer2.LastStateChange = timeFromNull(lastStateChange)
		if db.Migrator().HasTable("promotion_rules") {
			db.Model(&models.PromotionRule{}).Count(&resp.Layer2.ActivePromotionRules)
		}
		db.Model(&models.PodCapability{}).
			Where("state IN ('exploited', 'chained')").
			Count(&resp.Layer2.ExploitedCapCount)
		resp.Layer2.FreshnessMinutes, resp.Layer2.Status = calcFreshness(resp.Layer2.LastStateChange, getPolicyByLayer("layer2"))

		// --- Layer 3 ---
		if db.Migrator().HasTable("attack_paths") {
			var lastPath sql.NullTime
			activePaths := func() *gorm.DB {
				q := db.Model(&models.AttackPath{}).
					Joins("INNER JOIN pods ON pods.uid = attack_paths.pod_uid AND pods.deleted_at IS NULL")
				if len(clusterIDs) > 0 {
					q = q.Where("pods.cluster_id IN ?", clusterIDs)
				}
				return q
			}
			activePaths().Select("MAX(attack_paths.updated_at)").Scan(&lastPath)
			resp.Layer3.LastPathComputation = timeFromNull(lastPath)
			activePaths().Where("attack_paths.total_risk >= 0.7").Count(&resp.Layer3.TotalPaths)
			activePaths().Where("attack_paths.total_risk >= 9.0").Count(&resp.Layer3.CriticalPaths)
		}
		resp.Layer3.FreshnessMinutes, resp.Layer3.Status = calcFreshness(resp.Layer3.LastPathComputation, getPolicyByLayer("layer3"))

		// --- Layer 4 ---
		var lastScore sql.NullTime
		db.Model(&models.RiskScore{}).
			Where("deleted_at IS NULL").
			Select("MAX(calculated_at)").Scan(&lastScore)
		resp.Layer4.LastScoreCalc = timeFromNull(lastScore)
		db.Model(&models.RiskScore{}).
			Where("deleted_at IS NULL").
			Count(&resp.Layer4.ResourcesScored)
		var avgScore sql.NullFloat64
		db.Model(&models.RiskScore{}).
			Where("deleted_at IS NULL").
			Select("AVG(total_score)").Scan(&avgScore)
		if avgScore.Valid {
			resp.Layer4.AvgScore = avgScore.Float64
		}
		db.Model(&models.RiskScore{}).
			Where("scorer_version = 'v3' AND deleted_at IS NULL").
			Count(&resp.Layer4.V3Resources)
		resp.Layer4.FreshnessMinutes, resp.Layer4.Status = calcFreshness(resp.Layer4.LastScoreCalc, getPolicyByLayer("layer4"))

		c.JSON(http.StatusOK, gin.H{"data": resp})
	}
}
