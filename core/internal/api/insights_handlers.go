package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/worker"
)

// GetInsights returns all insights with optional filtering
func GetInsights(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var insights []models.Insight
		query := db.Model(&models.Insight{})
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

		// Default: only show active insights (unless status filter is explicitly set)
		statusParam := c.Query("status")
		if statusParam == "all" {
			// Show all insights (no status filter) - explicitly requested
			log.Printf("[GetInsights] Status=all: returning all insights")
		} else if statusParam != "" {
			// Filter by specific status
			query = query.Where("status = ?", statusParam)
			log.Printf("[GetInsights] Status=%s: filtering by status", statusParam)
		} else {
			// Default to active insights only when no status parameter provided
			// Use explicit Where clause to ensure it's applied
			query = query.Where("status = ?", "active")
			log.Printf("[GetInsights] No status param: defaulting to active (applying WHERE status = 'active')")
		}

		// Filter by type (support both old 'type' and new 'insight_type')
		if insightType := c.Query("type"); insightType != "" {
			query = query.Where("(type = ? OR insight_type = ?)", insightType, insightType)
		}

		// Filter by insight_type (new schema)
		if insightType := c.Query("insight_type"); insightType != "" {
			query = query.Where("insight_type = ?", insightType)
		}

		// Filter by severity
		if severity := c.Query("severity"); severity != "" {
			query = query.Where("severity = ?", severity)
		}

		// Filter by resource_uid (new schema)
		if resourceUID := c.Query("resource_uid"); resourceUID != "" {
			query = query.Where("resource_uid = ?", resourceUID)
		}

		// Filter by resource_type (new schema)
		if resourceType := c.Query("resource_type"); resourceType != "" {
			query = query.Where("resource_type = ?", resourceType)
		}

		// Filter by resource_namespace (new schema)
		if resourceNamespace := c.Query("resource_namespace"); resourceNamespace != "" {
			query = query.Where("resource_namespace = ?", resourceNamespace)
		}

		// Filter by resource_name (new schema)
		if resourceName := c.Query("resource_name"); resourceName != "" {
			query = query.Where("resource_name = ?", resourceName)
		}

		// Filter by sbom_id (old schema, still supported)
		if sbomID := c.Query("sbom_id"); sbomID != "" {
			query = query.Where("sbom_id = ?", sbomID)
		}

		// Filter by cluster (from affected resources - old schema, still supported)
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("affected_resources::text LIKE ?", "%"+clusterID+"%")
		}

		// Exclude Pod insights whose pod no longer exists (so list matches summary and Risk Center is consistent)
		query = query.Where("(resource_type != 'Pod' OR resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))")

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&insights).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"insights": insights,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

// insightWithResourceExists is used by GetInsight to add resourceExists when resource is Pod.
type insightWithResourceExists struct {
	models.Insight
	ResourceExists *bool `json:"resourceExists,omitempty"`
}

// GetInsight returns a specific insight by ID.
// When resource_type is Pod, adds resourceExists: true/false so UI can show "Resource no longer exists" for deleted pods.
func GetInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var insight models.Insight
		if err := db.First(&insight, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Insight not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resp := insightWithResourceExists{Insight: insight}
		if insight.ResourceType == "Pod" && insight.ResourceUID != "" {
			var podExists int64
			db.Model(&models.Pod{}).Where("uid = ? AND deleted_at IS NULL", insight.ResourceUID).Count(&podExists)
			exists := podExists > 0
			resp.ResourceExists = &exists
		}
		c.JSON(http.StatusOK, resp)
	}
}

// GetInsightsSummary returns summary statistics of insights.
// Query param clusterId: when set, counts are scoped to insights for Pods in that cluster (sync with global cluster selector).
// Query param sinceMinutes: when > 0, counts are limited to insights with detected_at >= now - sinceMinutes.
func GetInsightsSummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := strings.TrimSpace(c.Query("clusterId"))
		if clusterID != "" {
			clusterID = NormalizeClusterID(db, clusterID)
		}
		sinceMinutes, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "0"))
		var since time.Time
		if sinceMinutes > 0 {
			since = time.Now().Add(-time.Duration(sinceMinutes) * time.Minute)
		}
		detectedSinceClause := ""
		if sinceMinutes > 0 {
			detectedSinceClause = " AND i.detected_at >= ?"
		}
		detectedSinceClauseNoAlias := ""
		if sinceMinutes > 0 {
			detectedSinceClauseNoAlias = " AND detected_at >= ?"
		}

		var summary struct {
			Total    int64            `json:"total"`
			Critical int64            `json:"critical"`
			High     int64            `json:"high"`
			Medium   int64            `json:"medium"`
			Low      int64            `json:"low"`
			ByType   map[string]int64 `json:"byType"`
		}
		summary.ByType = make(map[string]int64)

		if clusterID != "" {
			// Scope to insights whose resource_uid matches a pod in this cluster (same join as GetDashboardStats).
			// Join: pods.uid = insights.resource_uid (no resource_type filter to avoid case/format mismatch).
			// Count all insight types so summary returns risk data when any risks exist for the cluster.
			joinCond := "INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL"
			// Diagnostic: why summary might be 0 — log pod count, global insight count, and join result
			var podCount, insightGlobal int64
			db.Raw("SELECT COUNT(DISTINCT uid) FROM pods WHERE cluster_id = ? AND deleted_at IS NULL", clusterID).Scan(&podCount)
			db.Model(&models.Insight{}).Where("deleted_at IS NULL AND (status = ? OR status IS NULL)", "active").Count(&insightGlobal)
			log.Printf("[InsightsSummary] clusterId=%q normalized; pods_in_cluster=%d, insights_global=%d", clusterID, podCount, insightGlobal)
			totalArgs := []interface{}{clusterID}
			if sinceMinutes > 0 {
				totalArgs = append(totalArgs, since)
			}
			db.Raw(`
				SELECT COUNT(*) FROM insights i
				`+joinCond+`
				WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)`+detectedSinceClause,
				totalArgs...).Scan(&summary.Total)
			log.Printf("[InsightsSummary] clusterId=%q join result total=%d", clusterID, summary.Total)
			if summary.Total == 0 && podCount > 0 && insightGlobal > 0 {
				var matchCount int64
				db.Raw(`
					SELECT COUNT(*) FROM insights i
					WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)
					AND i.resource_uid IN (SELECT uid FROM pods WHERE cluster_id = ? AND deleted_at IS NULL)`,
					clusterID).Scan(&matchCount)
				log.Printf("[InsightsSummary] clusterId=%q uid-match check: insights_with_resource_uid_in_cluster_pods=%d (if 0, resource_uid format may not match pods.uid)", clusterID, matchCount)
			}

			var severityCounts []struct {
				Severity string `gorm:"column:severity"`
				Count    int64  `gorm:"column:count"`
			}
			sevArgs := []interface{}{clusterID}
			if sinceMinutes > 0 {
				sevArgs = append(sevArgs, since)
			}
			db.Raw(`
				SELECT LOWER(i.severity) as severity, COUNT(*) as count 
				FROM insights i
				`+joinCond+`
				WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)`+detectedSinceClause+`
				GROUP BY LOWER(i.severity)`,
				sevArgs...).Scan(&severityCounts)
			for _, sc := range severityCounts {
				switch sc.Severity {
				case "critical":
					summary.Critical = sc.Count
				case "high":
					summary.High = sc.Count
				case "medium":
					summary.Medium = sc.Count
				case "low":
					summary.Low = sc.Count
				}
			}

			var typeCounts []struct {
				Type  string `gorm:"column:insight_type"`
				Count int64  `gorm:"column:count"`
			}
			typeArgs := []interface{}{clusterID}
			if sinceMinutes > 0 {
				typeArgs = append(typeArgs, since)
			}
			db.Raw(`
				SELECT i.insight_type, COUNT(*) as count 
				FROM insights i
				`+joinCond+`
				WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)`+detectedSinceClause+`
				GROUP BY i.insight_type`,
				typeArgs...).Scan(&typeCounts)
			for _, tc := range typeCounts {
				summary.ByType[tc.Type] = tc.Count
			}
		} else {
			// Global scope: only count insights for existing resources (Pod insights only when pod exists)
			podFilter := "(resource_type != 'Pod' OR resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))"
			query := db.Model(&models.Insight{}).Where("deleted_at IS NULL AND (status = ? OR status IS NULL)", "active").Where(podFilter)
			if sinceMinutes > 0 {
				query = query.Where("detected_at >= ?", since)
			}
			query.Count(&summary.Total)

			var severityCounts []struct {
				Severity string `gorm:"column:severity"`
				Count    int64  `gorm:"column:count"`
			}
			if sinceMinutes > 0 {
				db.Raw(`
					SELECT LOWER(severity) as severity, COUNT(*) as count 
					FROM insights 
					WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL) AND `+podFilter+detectedSinceClauseNoAlias+`
					GROUP BY LOWER(severity)`, since).Scan(&severityCounts)
			} else {
				db.Raw(`
					SELECT LOWER(severity) as severity, COUNT(*) as count 
					FROM insights 
					WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL) AND `+podFilter+`
					GROUP BY LOWER(severity)
				`).Scan(&severityCounts)
			}
			for _, sc := range severityCounts {
				switch sc.Severity {
				case "critical":
					summary.Critical = sc.Count
				case "high":
					summary.High = sc.Count
				case "medium":
					summary.Medium = sc.Count
				case "low":
					summary.Low = sc.Count
				}
			}

			var typeCounts []struct {
				Type  string `gorm:"column:insight_type"`
				Count int64  `gorm:"column:count"`
			}
			summaryQuery := db.Model(&models.Insight{}).
				Where("deleted_at IS NULL AND (status = ? OR status IS NULL) AND "+podFilter)
			if sinceMinutes > 0 {
				summaryQuery = summaryQuery.Where("detected_at >= ?", since)
			}
			summaryQuery.
				Select("insight_type, COUNT(*) as count").
				Group("insight_type").
				Scan(&typeCounts)
			for _, tc := range typeCounts {
				summary.ByType[tc.Type] = tc.Count
			}
		}

		c.JSON(http.StatusOK, summary)
	}
}

// TriggerRiskEvaluation manually triggers risk evaluation
// This uses the HistoricalRiskEvaluator for database-based evaluation
func TriggerRiskEvaluation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		evaluator := worker.NewHistoricalRiskEvaluator(db)
		if err := evaluator.EvaluateAllResources(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Risk evaluation triggered successfully",
		})
	}
}

// TriggerHistoricalRiskEvaluation triggers evaluation using the Risk Worker engine
// This processes historical data from database using the same engine as Risk Worker
func TriggerHistoricalRiskEvaluation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Step 1: Re-evaluate all resources (creates/updates insights for existing risks)
		evaluator := worker.NewHistoricalRiskEvaluator(db)
		if err := evaluator.EvaluateAllResources(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Step 2: Auto-resolve insights where risks no longer exist
		statusUpdater := worker.NewInsightStatusUpdater(db)
		if err := statusUpdater.UpdateStatusForResolvedRisks(ctx); err != nil {
			// Log error but don't fail the request - evaluation was successful
			log.Printf("[TriggerHistoricalRiskEvaluation] Error updating insight statuses: %v", err)
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Historical risk evaluation completed successfully",
		})
	}
}

// DeleteInsight deletes an insight by ID
func DeleteInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := db.Delete(&models.Insight{}, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Insight deleted successfully"})
	}
}

// AcknowledgeInsight acknowledges an insight (keeps status as 'active' but marks as acknowledged)
func AcknowledgeInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var insight models.Insight
		if err := db.First(&insight, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Insight not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Update insight (acknowledgment doesn't change status, just updates timestamp)
		now := time.Now()
		updates := map[string]interface{}{
			"updated_at": now,
		}

		if err := db.Model(&insight).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      "Insight acknowledged successfully",
			"acknowledged": true,
			"status":       insight.Status,
		})
	}
}

// ResolveInsight resolves an insight
func ResolveInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var request struct {
			Resolution string `json:"resolution"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var insight models.Insight
		if err := db.First(&insight, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Insight not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Update insight status to 'resolved'
		insight.Status = "resolved"
		insight.Recommendation = request.Resolution
		insight.UpdatedAt = time.Now()

		if err := db.Save(&insight).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "Insight resolved successfully",
			"resolved": true,
			"status":   insight.Status,
		})
	}
}

// DismissInsight dismisses an insight (marks as dismissed)
func DismissInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var request struct {
			Reason string `json:"reason"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			// Reason is optional, continue without it
		}

		var insight models.Insight
		if err := db.First(&insight, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Insight not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Update insight status to 'dismissed'
		insight.Status = "dismissed"
		insight.UpdatedAt = time.Now()

		if err := db.Save(&insight).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "Insight dismissed successfully",
			"dismissed": true,
			"status":    insight.Status,
		})
	}
}
