package api

import (
	"encoding/json"
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

// createInsightAuditLog writes an audit log entry for a Risk Center insight action (acknowledge, resolve, dismiss).
// userID and username are read from context (set by auth middleware); if missing, 0 and "system" are used.
func createInsightAuditLog(db *gorm.DB, c *gin.Context, action, insightID, details string) {
	var userID uint
	var username string
	if v, ok := c.Get("userID"); ok {
		if u, ok := v.(uint); ok {
			userID = u
		}
	}
	if v, ok := c.Get("username"); ok {
		if s, ok := v.(string); ok {
			username = s
		}
	}
	if username == "" {
		username = "system"
	}
	entry := models.AuditLog{
		ClusterID:  "",
		UserID:     userID,
		Action:     action,
		Resource:   "insight",
		ResourceID: insightID,
		Details:    details,
		User:       username,
		IP:         c.ClientIP(),
	}
	if err := db.Create(&entry).Error; err != nil {
		log.Printf("[Insights] Failed to write audit log: action=%s insight=%s: %v", action, insightID, err)
	}
}

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
		createInsightAuditLog(db, c, "view", id, "{}")
		c.JSON(http.StatusOK, resp)
	}
}

// InsightContextResponse is returned by GET /risk/insights/:id/context.
// It provides cross-resource context for a single insight: pods, cluster, and related risk rules.
type InsightContextResponse struct {
	Insight models.Insight      `json:"insight"`
	Pods    []models.Pod        `json:"pods"`
	Cluster *models.Cluster     `json:"cluster,omitempty"`
	Rules   []models.RiskRule   `json:"rules"`
}

// GetInsightContext returns cross-resource context for a specific insight:
// - The insight itself
// - Related pods (when resource_type = Pod)
// - Cluster (derived from first related pod)
// - Related risk rules (resolved from violated_rules JSON when present)
func GetInsightContext(db *gorm.DB) gin.HandlerFunc {
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

		resp := InsightContextResponse{
			Insight: insight,
			Pods:    []models.Pod{},
			Rules:   []models.RiskRule{},
		}

		// Related pods and cluster (only for Pod-scoped insights)
		if strings.EqualFold(insight.ResourceType, "Pod") && insight.ResourceUID != "" {
			var pods []models.Pod
			if err := db.Where("uid = ? AND deleted_at IS NULL", insight.ResourceUID).Find(&pods).Error; err == nil {
				resp.Pods = pods
				if len(pods) > 0 && pods[0].ClusterID != "" {
					var cluster models.Cluster
					if err := db.Where("id = ? AND deleted_at IS NULL", pods[0].ClusterID).First(&cluster).Error; err == nil {
						resp.Cluster = &cluster
					}
				}
			}
		}

		// Related risk rules from violated_rules JSON (when present)
		if strings.TrimSpace(insight.ViolatedRules) != "" {
			var raw interface{}
			if err := json.Unmarshal([]byte(insight.ViolatedRules), &raw); err == nil {
				ruleIDs := make(map[string]struct{})
				switch v := raw.(type) {
				case []interface{}:
					for _, item := range v {
						if m, ok := item.(map[string]interface{}); ok {
							if rid, ok := m["ruleId"].(string); ok && strings.TrimSpace(rid) != "" {
								ruleIDs[strings.TrimSpace(rid)] = struct{}{}
							}
						}
					}
				case map[string]interface{}:
					if rid, ok := v["ruleId"].(string); ok && strings.TrimSpace(rid) != "" {
						ruleIDs[strings.TrimSpace(rid)] = struct{}{}
					}
				}
				if len(ruleIDs) > 0 {
					ids := make([]string, 0, len(ruleIDs))
					for rid := range ruleIDs {
						ids = append(ids, rid)
					}
					var rules []models.RiskRule
					if err := db.Where("rule_id IN ?", ids).Find(&rules).Error; err == nil {
						resp.Rules = rules
					}
				}
			}
		}

		c.JSON(http.StatusOK, resp)
	}
}

// InsightsSummaryResult is the response shape of GET /insights/summary (for caching).
type InsightsSummaryResult struct {
	Total    int64            `json:"total"`
	Critical int64            `json:"critical"`
	High     int64            `json:"high"`
	Medium   int64            `json:"medium"`
	Low      int64            `json:"low"`
	ByType   map[string]int64 `json:"byType"`
}

// getInsightsSummaryData returns summary counts (for caching).
func getInsightsSummaryData(db *gorm.DB, clusterID string, sinceMinutes int) InsightsSummaryResult {
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
	var summary InsightsSummaryResult
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
		return summary
}

// GetInsightsSummary returns summary statistics of insights.
func GetInsightsSummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := strings.TrimSpace(c.Query("clusterId"))
		if clusterID != "" {
			clusterID = NormalizeClusterID(db, clusterID)
		}
		sinceMinutes, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "0"))
		result := getInsightsSummaryData(db, clusterID, sinceMinutes)
		c.JSON(http.StatusOK, result)
	}
}

// GetInsightsSummaryCached uses defaultRisksCache when set (TTL 60s).
func GetInsightsSummaryCached(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := strings.TrimSpace(c.Query("clusterId"))
		if clusterID != "" {
			clusterID = NormalizeClusterID(db, clusterID)
		}
		sinceMinutes, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "0"))
		key := BuildInsightsSummaryCacheKey(clusterID, sinceMinutes)
		if defaultRisksCache != nil {
			if b, ok := defaultRisksCache.Get(key); ok {
				c.Data(http.StatusOK, "application/json", b)
				return
			}
		}
		result := getInsightsSummaryData(db, clusterID, sinceMinutes)
		if defaultRisksCache != nil {
			if b, err := json.Marshal(result); err == nil {
				defaultRisksCache.Set(key, b, risksCacheTTL)
			}
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetInsightsSummaryGlobalCached returns global (all-clusters) summary; same shape as GET /insights/summary without clusterId. Cached (TTL 60s).
// GET /insights/summary/global?sinceMinutes=0
func GetInsightsSummaryGlobalCached(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sinceMinutes, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "0"))
		key := BuildInsightsSummaryGlobalCacheKey(sinceMinutes)
		if defaultRisksCache != nil {
			if b, ok := defaultRisksCache.Get(key); ok {
				c.Data(http.StatusOK, "application/json", b)
				return
			}
		}
		result := getInsightsSummaryData(db, "", sinceMinutes)
		if defaultRisksCache != nil {
			if b, err := json.Marshal(result); err == nil {
				defaultRisksCache.Set(key, b, risksCacheTTL)
			}
		}
		c.JSON(http.StatusOK, result)
	}
}

// InsightsSummaryByClusterItem is one row for GET /insights/summary/by-cluster.
type InsightsSummaryByClusterItem struct {
	ClusterID   string `json:"clusterId"`
	ClusterName string `json:"clusterName,omitempty"`
	Total       int64  `json:"total"`
	Critical    int64  `json:"critical"`
	High        int64  `json:"high"`
	Medium      int64  `json:"medium"`
	Low         int64  `json:"low"`
}

// GetInsightsSummaryByCluster returns summary counts grouped by cluster (for global / multi-cluster view).
// Query param sinceMinutes: when > 0, only insights with detected_at >= now - sinceMinutes.
func GetInsightsSummaryByCluster(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sinceMinutes, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "0"))
		var since time.Time
		if sinceMinutes > 0 {
			since = time.Now().Add(-time.Duration(sinceMinutes) * time.Minute)
		}
		detectedClause := ""
		if sinceMinutes > 0 {
			detectedClause = " AND i.detected_at >= ?"
		}
		joinCond := "INNER JOIN pods p ON p.uid = i.resource_uid AND p.deleted_at IS NULL"
		whereBase := "i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)"
		type row struct {
			ClusterID   string `gorm:"column:cluster_id"`
			Total       int64  `gorm:"column:total"`
			Critical    int64  `gorm:"column:critical"`
			High        int64  `gorm:"column:high"`
			Medium      int64  `gorm:"column:medium"`
			Low         int64  `gorm:"column:low"`
		}
		var rows []row
		if sinceMinutes > 0 {
			db.Raw(`
				SELECT p.cluster_id,
					COUNT(*)::bigint AS total,
					COUNT(*) FILTER (WHERE LOWER(i.severity) = 'critical')::bigint AS critical,
					COUNT(*) FILTER (WHERE LOWER(i.severity) = 'high')::bigint AS high,
					COUNT(*) FILTER (WHERE LOWER(i.severity) = 'medium')::bigint AS medium,
					COUNT(*) FILTER (WHERE LOWER(i.severity) = 'low')::bigint AS low
				FROM insights i
				`+joinCond+`
				WHERE `+whereBase+detectedClause+`
				GROUP BY p.cluster_id`,
				since).Scan(&rows)
		} else {
			db.Raw(`
				SELECT p.cluster_id,
					COUNT(*)::bigint AS total,
					COUNT(*) FILTER (WHERE LOWER(i.severity) = 'critical')::bigint AS critical,
					COUNT(*) FILTER (WHERE LOWER(i.severity) = 'high')::bigint AS high,
					COUNT(*) FILTER (WHERE LOWER(i.severity) = 'medium')::bigint AS medium,
					COUNT(*) FILTER (WHERE LOWER(i.severity) = 'low')::bigint AS low
				FROM insights i
				`+joinCond+`
				WHERE `+whereBase+`
				GROUP BY p.cluster_id`).Scan(&rows)
		}
		clusterNames := make(map[string]string)
		if len(rows) > 0 {
			var ids []string
			for _, r := range rows {
				ids = append(ids, r.ClusterID)
			}
			var clusters []struct {
				ID   string `gorm:"column:id"`
				Name string `gorm:"column:name"`
			}
			db.Table("clusters").Where("id IN ?", ids).Select("id, name").Scan(&clusters)
			for _, cl := range clusters {
				clusterNames[cl.ID] = cl.Name
			}
		}
		out := make([]InsightsSummaryByClusterItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, InsightsSummaryByClusterItem{
				ClusterID:   r.ClusterID,
				ClusterName: clusterNames[r.ClusterID],
				Total:       r.Total,
				Critical:    r.Critical,
				High:        r.High,
				Medium:      r.Medium,
				Low:         r.Low,
			})
		}
		c.JSON(http.StatusOK, gin.H{"byCluster": out})
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

		createInsightAuditLog(db, c, "acknowledge", id, "{}")
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

		details := "{}"
		if request.Resolution != "" {
			if b, err := json.Marshal(map[string]string{"resolution": request.Resolution}); err == nil {
				details = string(b)
			}
		}
		createInsightAuditLog(db, c, "resolve", id, details)
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

		details := "{}"
		if request.Reason != "" {
			if b, err := json.Marshal(map[string]string{"reason": request.Reason}); err == nil {
				details = string(b)
			}
		}
		createInsightAuditLog(db, c, "dismiss", id, details)
		c.JSON(http.StatusOK, gin.H{
			"message":   "Insight dismissed successfully",
			"dismissed": true,
			"status":    insight.Status,
		})
	}
}

const maxBulkInsightIDs = 500

// BulkInsightsAction runs acknowledge, resolve, or dismiss on multiple insights in a transaction.
// POST /risk/insights/bulk body: { "action": "acknowledge"|"resolve"|"dismiss", "insight_ids": ["id1","id2"], "resolution"?: "", "reason"?: "" }
func BulkInsightsAction(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Action      string   `json:"action" binding:"required"`
			InsightIDs  []string `json:"insight_ids" binding:"required"`
			Resolution  string   `json:"resolution"`
			Reason      string   `json:"reason"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: action and insight_ids required"})
			return
		}
		switch body.Action {
		case "acknowledge", "resolve", "dismiss":
			// ok
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "action must be acknowledge, resolve, or dismiss"})
			return
		}
		if len(body.InsightIDs) > maxBulkInsightIDs {
			c.JSON(http.StatusBadRequest, gin.H{"error": "insight_ids exceeds max " + strconv.Itoa(maxBulkInsightIDs)})
			return
		}
		var successCount, failedCount int
		var errors []map[string]interface{}

		for _, id := range body.InsightIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			var insight models.Insight
			if err := db.First(&insight, "id = ?", id).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					failedCount++
					errors = append(errors, map[string]interface{}{"id": id, "error": "not found"})
				} else {
					failedCount++
					errors = append(errors, map[string]interface{}{"id": id, "error": err.Error()})
				}
				continue
			}
			switch body.Action {
			case "acknowledge":
				if err := db.Model(&insight).Update("updated_at", time.Now()).Error; err != nil {
					failedCount++
					errors = append(errors, map[string]interface{}{"id": id, "error": err.Error()})
					continue
				}
				createInsightAuditLog(db, c, "acknowledge", id, "{}")
				successCount++
			case "resolve":
				insight.Status = "resolved"
				insight.Recommendation = body.Resolution
				insight.UpdatedAt = time.Now()
				if err := db.Save(&insight).Error; err != nil {
					failedCount++
					errors = append(errors, map[string]interface{}{"id": id, "error": err.Error()})
					continue
				}
				details := "{}"
				if body.Resolution != "" {
					if b, err := json.Marshal(map[string]string{"resolution": body.Resolution}); err == nil {
						details = string(b)
					}
				}
				createInsightAuditLog(db, c, "resolve", id, details)
				successCount++
			case "dismiss":
				insight.Status = "dismissed"
				insight.UpdatedAt = time.Now()
				if err := db.Save(&insight).Error; err != nil {
					failedCount++
					errors = append(errors, map[string]interface{}{"id": id, "error": err.Error()})
					continue
				}
				details := "{}"
				if body.Reason != "" {
					if b, err := json.Marshal(map[string]string{"reason": body.Reason}); err == nil {
						details = string(b)
					}
				}
				createInsightAuditLog(db, c, "dismiss", id, details)
				successCount++
			}
		}

		resp := gin.H{
			"success_count": successCount,
			"failed_count":  failedCount,
			"action":        body.Action,
		}
		if len(errors) > 0 {
			resp["errors"] = errors
		}
		c.JSON(http.StatusOK, resp)
	}
}
