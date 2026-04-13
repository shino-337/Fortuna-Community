package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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
	LastPceEval       *time.Time `json:"lastPceEval"`
	LastRiskEngineEval *time.Time `json:"lastRiskEngineEval"`
	InsightCount      int64      `json:"insightCount"`
}

// PipelineLayer2 — Runtime Enrichment (CSC state promotion + AttackStepInference)
type PipelineLayer2 struct {
	LastStateChange       *time.Time `json:"lastStateChange"`
	ActivePromotionRules  int64      `json:"activePromotionRules"`
	ExploitedCapCount     int64      `json:"exploitedCapCount"`
}

// PipelineLayer3 — Path Analysis (Attack Path Builder)
type PipelineLayer3 struct {
	LastPathComputation *time.Time `json:"lastPathComputation"`
	TotalPaths          int64      `json:"totalPaths"`
	CriticalPaths       int64      `json:"criticalPaths"`
}

// PipelineLayer4 — Unified Scoring
type PipelineLayer4 struct {
	LastScoreCalc   *time.Time `json:"lastScoreCalc"`
	ResourcesScored int64      `json:"resourcesScored"`
	AvgScore        float64    `json:"avgScore"`
	V3Resources     int64      `json:"v3Resources"`
}

// GetPipelineHealth returns the current health and activity of each pipeline layer.
// GET /api/v1/monitoring/pipeline-health
func GetPipelineHealth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp := PipelineHealthResponse{}

		// --- Layer 1 ---
		var lastCapUpdate, lastInsightUpdate time.Time
		db.Model(&models.PodCapability{}).Select("MAX(updated_at)").Scan(&lastCapUpdate)
		if !lastCapUpdate.IsZero() {
			resp.Layer1.LastPceEval = &lastCapUpdate
		}

		db.Model(&models.Insight{}).
			Where("insight_type = 'rbac' AND deleted_at IS NULL").
			Select("MAX(updated_at)").Scan(&lastInsightUpdate)
		if !lastInsightUpdate.IsZero() {
			resp.Layer1.LastRiskEngineEval = &lastInsightUpdate
		}
		db.Model(&models.Insight{}).
			Where("status = 'active' AND deleted_at IS NULL").
			Count(&resp.Layer1.InsightCount)

		// --- Layer 2 ---
		var lastStateChange time.Time
		db.Model(&models.PodCapability{}).
			Where("state != 'detected'").
			Select("MAX(updated_at)").Scan(&lastStateChange)
		if !lastStateChange.IsZero() {
			resp.Layer2.LastStateChange = &lastStateChange
		}
		if db.Migrator().HasTable("promotion_rules") {
			db.Model(&models.PromotionRule{}).Count(&resp.Layer2.ActivePromotionRules)
		}
		db.Model(&models.PodCapability{}).
			Where("state IN ('exploited', 'chained')").
			Count(&resp.Layer2.ExploitedCapCount)

		// --- Layer 3 ---
		if db.Migrator().HasTable("attack_paths") {
			var lastPath time.Time
			db.Model(&models.AttackPath{}).Select("MAX(updated_at)").Scan(&lastPath)
			if !lastPath.IsZero() {
				resp.Layer3.LastPathComputation = &lastPath
			}
			db.Model(&models.AttackPath{}).Count(&resp.Layer3.TotalPaths)
			db.Model(&models.AttackPath{}).Where("total_risk >= 9.0").Count(&resp.Layer3.CriticalPaths)
		}

		// --- Layer 4 ---
		var lastScore time.Time
		db.Model(&models.RiskScore{}).
			Where("deleted_at IS NULL").
			Select("MAX(calculated_at)").Scan(&lastScore)
		if !lastScore.IsZero() {
			resp.Layer4.LastScoreCalc = &lastScore
		}
		db.Model(&models.RiskScore{}).
			Where("deleted_at IS NULL").
			Count(&resp.Layer4.ResourcesScored)
		var avgScore float64
		db.Model(&models.RiskScore{}).
			Where("deleted_at IS NULL").
			Select("AVG(total_score)").Scan(&avgScore)
		resp.Layer4.AvgScore = avgScore
		db.Model(&models.RiskScore{}).
			Where("scorer_version = 'v3' AND deleted_at IS NULL").
			Count(&resp.Layer4.V3Resources)

		c.JSON(http.StatusOK, gin.H{"data": resp})
	}
}
