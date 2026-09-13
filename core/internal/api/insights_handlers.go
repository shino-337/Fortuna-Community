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

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/explainability"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
	"github.com/fortuna/core/pkg/securityaudit"
	"github.com/fortuna/core/pkg/worker"
)

type InsightEvidenceRefs struct {
	EventIDs      []string `json:"eventIds,omitempty"`
	FactIDs       []string `json:"factIds,omitempty"`
	SignalTypes   []string `json:"signalTypes,omitempty"`
	IncidentTypes []string `json:"incidentTypes,omitempty"`
	CapabilityIDs []string `json:"capabilityIds,omitempty"`
	RuleIDs       []string `json:"ruleIds,omitempty"`
}

type insightWithExplainability struct {
	models.Insight
	EvidenceRefs     InsightEvidenceRefs        `json:"evidence_refs"`
	ExplanationChain []explainability.ChainStep `json:"explanation_chain,omitempty"`
}

func resourceUIDClusterID(db *gorm.DB, resourceUID string) (string, error) {
	resourceUID = strings.TrimSpace(resourceUID)
	if db == nil || resourceUID == "" {
		return "", nil
	}
	var clusterID string
	err := db.Model(&models.Pod{}).
		Select("cluster_id").
		Unscoped().
		Where("uid = ?", resourceUID).
		Limit(1).
		Scan(&clusterID).Error
	return strings.TrimSpace(clusterID), err
}

func requireResourceUIDClusterScope(db *gorm.DB, c *gin.Context, resourceUID string) bool {
	if _, restricted := middleware.ScopedClusterIDs(c); !restricted {
		return true
	}
	clusterID, err := resourceUIDClusterID(db, resourceUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return false
	}
	if clusterID == "" {
		middleware.AbortClusterScopeDenied(db, c, "unknown")
		return false
	}
	if middleware.ClusterAllowed(c, clusterID) {
		return true
	}
	middleware.AbortClusterScopeDenied(db, c, clusterID)
	return false
}

func requireInsightClusterScope(db *gorm.DB, c *gin.Context, insight models.Insight) bool {
	return requireResourceUIDClusterScope(db, c, insight.ResourceUID)
}

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

func appendInsightGovernanceEvent(db *gorm.DB, c *gin.Context, action, insightID string, beforeStatus, afterStatus string, details any) {
	sev := securityaudit.ClassifyResultSeverity(action, "success")
	ev := securityaudit.FromRequest(
		c,
		authorization.ToStrings(middleware.GrantedPermissions(c)),
		c.GetString(middleware.CtxJWTSessionID),
		action,
		"finding",
		insightID,
		"success",
		sev,
		"jwt",
		map[string]any{"status": beforeStatus},
		map[string]any{"status": afterStatus},
		details,
		nil,
	)
	securityaudit.Append(db, &ev)
}

func scheduleUnifiedScoreRecalculation(db *gorm.DB, resourceUID string) {
	resourceUID = strings.TrimSpace(resourceUID)
	if resourceUID == "" {
		return
	}
	risk.NewUnifiedScorerV3(db).ScheduleUnifiedScoreCalculation(resourceUID)
}

// insightWithResourceExists is used by GetInsight to add resourceExists when resource is Pod.
type insightWithResourceExists struct {
	models.Insight
	ResourceExists    *bool                             `json:"resourceExists,omitempty"`
	EvidenceRefs      InsightEvidenceRefs               `json:"evidence_refs"`
	ExplanationChain  []explainability.ChainStep        `json:"explanation_chain,omitempty"`
	EvidenceChainRefs []explainability.EvidenceChainRef `json:"evidence_chain_refs,omitempty"`
	EnrichedRefs      *explainability.EnrichedRefs      `json:"enriched_refs,omitempty"`
	SeverityHint      string                            `json:"severity_hint,omitempty"`
	FinalScore        *float64                          `json:"final_score,omitempty"`
	FinalLevel        string                            `json:"final_level,omitempty"`
	Breakdown         []RiskBreakdownItem               `json:"breakdown,omitempty"`
}

func preferredScoreForResource(db *gorm.DB, resourceUID string) *models.RiskScore {
	resourceUID = strings.TrimSpace(resourceUID)
	if resourceUID == "" {
		return nil
	}
	var rs models.RiskScore
	if err := db.Model(&models.RiskScore{}).
		Where("resource_uid = ? AND deleted_at IS NULL AND LOWER(TRIM(COALESCE(scorer_version, ''))) = ?", resourceUID, "v3").
		Order("calculated_at DESC, id DESC").
		First(&rs).Error; err != nil {
		return nil
	}
	return &rs
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
		if !requireInsightClusterScope(db, c, insight) {
			return
		}
		refs := buildInsightEvidenceRefs(insight)
		chain := buildExplanationChain(refs)
		resp := insightWithResourceExists{
			Insight:           insight,
			EvidenceRefs:      refs,
			ExplanationChain:  chain,
			EvidenceChainRefs: explainability.FlattenChainToEvidenceRefs(chain),
			SeverityHint:      insight.Severity,
		}
		if s := preferredScoreForResource(db, insight.ResourceUID); s != nil {
			resp.FinalScore = &s.TotalScore
			resp.FinalLevel = risk.DeriveFinalLevelFromScore(s.TotalScore)
			resp.Breakdown = risk.ParseBreakdownFromFactorsJSON(s.Factors)
		}
		if c.Query("enrich") == "1" && insight.ResourceType == "Pod" && strings.TrimSpace(insight.ResourceUID) != "" {
			if er, err := explainability.BuildEnrichedRefs(c.Request.Context(), db, insight.ResourceUID, refs.FactIDs); err == nil && er != nil {
				resp.EnrichedRefs = er
			}
		}
		if insight.ResourceType == "Pod" && insight.ResourceUID != "" {
			var podExists int64
			db.Model(&models.Pod{}).Where("uid = ? AND deleted_at IS NULL", insight.ResourceUID).Count(&podExists)
			exists := podExists > 0
			resp.ResourceExists = &exists
		}
		createInsightAuditLog(db, c, "view", id, "{}")
		viewerJSON(c, http.StatusOK, resp)
	}
}

func buildInsightEvidenceRefs(in models.Insight) InsightEvidenceRefs {
	refs := InsightEvidenceRefs{}
	seen := map[string]map[string]struct{}{
		"event":      {},
		"fact":       {},
		"signal":     {},
		"incident":   {},
		"capability": {},
		"rule":       {},
	}
	add := func(kind, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[kind][v]; ok {
			return
		}
		seen[kind][v] = struct{}{}
		switch kind {
		case "event":
			refs.EventIDs = append(refs.EventIDs, v)
		case "fact":
			refs.FactIDs = append(refs.FactIDs, v)
		case "signal":
			refs.SignalTypes = append(refs.SignalTypes, v)
		case "incident":
			refs.IncidentTypes = append(refs.IncidentTypes, v)
		case "capability":
			refs.CapabilityIDs = append(refs.CapabilityIDs, v)
		case "rule":
			refs.RuleIDs = append(refs.RuleIDs, v)
		}
	}
	extractTo := func(m map[string]interface{}, kind string, keys ...string) {
		for _, key := range keys {
			if v, ok := m[key]; ok {
				switch t := v.(type) {
				case string:
					add(kind, t)
				case float64:
					add(kind, strconv.FormatInt(int64(t), 10))
				case []interface{}:
					for _, it := range t {
						switch tv := it.(type) {
						case string:
							add(kind, tv)
						case float64:
							add(kind, strconv.FormatInt(int64(tv), 10))
						}
					}
				}
			}
		}
	}

	// Parse Evidence (if available) and map known keys.
	if strings.TrimSpace(in.Evidence) != "" {
		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(in.Evidence), &ev); err == nil {
			if ids, ok := ev["evidence_fact_ids"].([]interface{}); ok {
				for _, it := range ids {
					if s, ok := it.(string); ok {
						add("fact", s)
					}
				}
			}
			if v, ok := ev["signal_type"].(string); ok {
				add("signal", v)
			}
			if v, ok := ev["incident_type"].(string); ok {
				add("incident", v)
			}
			if v, ok := ev["capability_id"].(string); ok {
				add("capability", v)
			}
			if v, ok := ev["capabilityId"].(string); ok {
				add("capability", v)
			}
			if v, ok := ev["event_id"]; ok {
				switch t := v.(type) {
				case string:
					add("event", t)
				case float64:
					add("event", strconv.FormatInt(int64(t), 10))
				}
			}
			// Generic fallbacks for explicit keys
			extractTo(ev, "event", "event", "eventId", "event_id", "eventIds", "event_ids")
			extractTo(ev, "fact", "fact", "factId", "fact_id", "factIds", "fact_ids", "evidenceFactIds", "evidence_fact_ids")
			extractTo(ev, "signal", "signal", "signalType", "signal_type", "signalTypes", "signal_types")
			extractTo(ev, "incident", "incident", "incidentType", "incident_type", "incidentTypes", "incident_types")
			extractTo(ev, "capability", "capability", "capabilityId", "capability_id", "capabilityIds", "capability_ids")
			extractTo(ev, "rule", "rule", "ruleId", "rule_id", "ruleIds", "rule_ids")
		}
	}

	// Parse ViolatedRules (if available).
	if strings.TrimSpace(in.ViolatedRules) != "" {
		var raw interface{}
		if err := json.Unmarshal([]byte(in.ViolatedRules), &raw); err == nil {
			switch v := raw.(type) {
			case []interface{}:
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						if rid, ok := m["ruleId"].(string); ok {
							add("rule", rid)
						}
						if rid, ok := m["rule_id"].(string); ok {
							add("rule", rid)
						}
						if rid, ok := m["id"].(string); ok {
							add("rule", rid)
						}
					}
				}
			case map[string]interface{}:
				if rid, ok := v["ruleId"].(string); ok {
					add("rule", rid)
				}
				if rid, ok := v["rule_id"].(string); ok {
					add("rule", rid)
				}
				if rid, ok := v["id"].(string); ok {
					add("rule", rid)
				}
			}
		}
	}
	return refs
}

func buildExplanationChain(refs InsightEvidenceRefs) []explainability.ChainStep {
	return explainability.BuildOrderedChain(
		refs.EventIDs,
		refs.FactIDs,
		refs.SignalTypes,
		refs.IncidentTypes,
		refs.CapabilityIDs,
		refs.RuleIDs,
	)
}

// InsightContextResponse is returned by GET /risk/insights/:id/context.
// It provides cross-resource context for a single insight: pods, cluster, and related risk rules.
type InsightContextResponse struct {
	Insight models.Insight    `json:"insight"`
	Pods    []models.Pod      `json:"pods"`
	Cluster *models.Cluster   `json:"cluster,omitempty"`
	Rules   []models.RiskRule `json:"rules"`
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
		if !requireInsightClusterScope(db, c, insight) {
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

		viewerJSON(c, http.StatusOK, resp)
	}
}

// RiskLevelCounts holds insight counts grouped by their pod's V3 unified final level (ADR bands).
type RiskLevelCounts struct {
	Critical int64 `json:"critical"`
	High     int64 `json:"high"`
	Medium   int64 `json:"medium"`
	Low      int64 `json:"low"`
}

// InsightsSummaryResult is the response shape of GET /risk/insights/summary (for caching).
type InsightsSummaryResult struct {
	Total           int64            `json:"total"`
	Critical        int64            `json:"critical"`
	High            int64            `json:"high"`
	Medium          int64            `json:"medium"`
	Low             int64            `json:"low"`
	ByType          map[string]int64 `json:"byType"`
	RiskLevelCounts *RiskLevelCounts `json:"riskLevelCounts,omitempty"`
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
		db.Model(&models.Insight{}).Where("deleted_at IS NULL AND (status IN ? OR status IS NULL)", []string{"active", "acknowledged"}).Count(&insightGlobal)
		log.Printf("[InsightsSummary] clusterId=%q normalized; pods_in_cluster=%d, insights_global=%d", clusterID, podCount, insightGlobal)
		totalArgs := []interface{}{clusterID}
		if sinceMinutes > 0 {
			totalArgs = append(totalArgs, since)
		}
		db.Raw(`
				SELECT COUNT(*) FROM insights i
				`+joinCond+`
				WHERE i.deleted_at IS NULL AND (i.status IN ('active', 'acknowledged') OR i.status IS NULL)`+detectedSinceClause,
			totalArgs...).Scan(&summary.Total)
		log.Printf("[InsightsSummary] clusterId=%q join result total=%d", clusterID, summary.Total)
		if summary.Total == 0 && podCount > 0 && insightGlobal > 0 {
			var matchCount int64
			db.Raw(`
					SELECT COUNT(*) FROM insights i
					WHERE i.deleted_at IS NULL AND (i.status IN ('active', 'acknowledged') OR i.status IS NULL)
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
				WHERE i.deleted_at IS NULL AND (i.status IN ('active', 'acknowledged') OR i.status IS NULL)`+detectedSinceClause+`
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
				WHERE i.deleted_at IS NULL AND (i.status IN ('active', 'acknowledged') OR i.status IS NULL)`+detectedSinceClause+`
				GROUP BY i.insight_type`,
			typeArgs...).Scan(&typeCounts)
		for _, tc := range typeCounts {
			summary.ByType[tc.Type] = tc.Count
		}
	} else {
		// Global scope: only count insights for existing resources (Pod insights only when pod exists)
		podFilter := "(resource_type != 'Pod' OR resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))"
		query := db.Model(&models.Insight{}).Where("deleted_at IS NULL AND (status IN ? OR status IS NULL)", []string{"active", "acknowledged"}).Where(podFilter)
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
					WHERE deleted_at IS NULL AND (status IN ('active', 'acknowledged') OR status IS NULL) AND `+podFilter+detectedSinceClauseNoAlias+`
					GROUP BY LOWER(severity)`, since).Scan(&severityCounts)
		} else {
			db.Raw(`
					SELECT LOWER(severity) as severity, COUNT(*) as count 
					FROM insights 
					WHERE deleted_at IS NULL AND (status IN ('active', 'acknowledged') OR status IS NULL) AND ` + podFilter + `
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
			Where("deleted_at IS NULL AND (status IN ('active', 'acknowledged') OR status IS NULL) AND " + podFilter)
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

	// V3 unified risk level counts: group insights by their pod's preferred V3 risk score band
	var rlcRows []struct {
		Level string `gorm:"column:risk_level"`
		Count int64  `gorm:"column:count"`
	}
	rlcSQL := `SELECT
		CASE
			WHEN pref.total_score >= 70 THEN 'critical'
			WHEN pref.total_score >= 40 THEN 'high'
			WHEN pref.total_score >= 20 THEN 'medium'
			ELSE 'low'
		END AS risk_level,
		COUNT(*) AS count
	FROM insights i
	INNER JOIN ` + preferredRiskScoreSubquerySQL + ` AS pref ON pref.resource_uid = i.resource_uid
	WHERE i.deleted_at IS NULL AND (i.status IN ('active', 'acknowledged') OR i.status IS NULL)`
	rlcArgs := []interface{}{}
	if clusterID != "" {
		rlcSQL += " AND i.resource_uid IN (SELECT uid FROM pods WHERE cluster_id = ? AND deleted_at IS NULL)"
		rlcArgs = append(rlcArgs, clusterID)
	} else {
		rlcSQL += " AND (i.resource_type != 'Pod' OR i.resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))"
	}
	if sinceMinutes > 0 {
		rlcSQL += " AND i.detected_at >= ?"
		rlcArgs = append(rlcArgs, since)
	}
	rlcSQL += " GROUP BY risk_level"
	if err := db.Raw(rlcSQL, rlcArgs...).Scan(&rlcRows).Error; err == nil && len(rlcRows) > 0 {
		rlc := &RiskLevelCounts{}
		for _, r := range rlcRows {
			switch r.Level {
			case "critical":
				rlc.Critical = r.Count
			case "high":
				rlc.High = r.Count
			case "medium":
				rlc.Medium = r.Count
			case "low":
				rlc.Low = r.Count
			}
		}
		summary.RiskLevelCounts = rlc
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

// GetInsightsSummaryGlobalCached returns global (all-clusters) summary; same shape as GET /risk/insights/summary without clusterId. Cached (TTL 60s).
// GET /risk/insights/summary/global?sinceMinutes=0
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

// InsightsSummaryByClusterItem is one row for GET /risk/insights/summary/by-cluster.
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
		whereBase := "i.deleted_at IS NULL AND (i.status IN ('active', 'acknowledged') OR i.status IS NULL)"
		type row struct {
			ClusterID string `gorm:"column:cluster_id"`
			Total     int64  `gorm:"column:total"`
			Critical  int64  `gorm:"column:critical"`
			High      int64  `gorm:"column:high"`
			Medium    int64  `gorm:"column:medium"`
			Low       int64  `gorm:"column:low"`
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
				` + joinCond + `
				WHERE ` + whereBase + `
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
		var insight models.Insight
		if err := db.First(&insight, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Insight not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !requireInsightClusterScope(db, c, insight) {
			return
		}
		if err := db.Delete(&insight).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Insight deleted successfully"})
	}
}

// AcknowledgeInsight persists the review state without resolving the risk.
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
		if !requireInsightClusterScope(db, c, insight) {
			return
		}

		if insight.Status == "resolved" || insight.Status == "dismissed" {
			c.JSON(http.StatusConflict, gin.H{"error": "closed findings cannot be acknowledged"})
			return
		}
		prevStatus := insight.Status
		// Persist acknowledgement.
		now := time.Now()
		updates := map[string]interface{}{
			"updated_at": now,
			"status":     "acknowledged",
		}

		if err := db.Model(&insight).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		insight.Status = "acknowledged"
		scheduleUnifiedScoreRecalculation(db, insight.ResourceUID)

		createInsightAuditLog(db, c, "acknowledge", id, "{}")
		appendInsightGovernanceEvent(db, c, securityaudit.ActionFindingsAcknowledge, id, prevStatus, insight.Status, map[string]any{"updatedAt": now.UTC().Format(time.RFC3339)})
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
		if !requireInsightClusterScope(db, c, insight) {
			return
		}

		prevStatus := insight.Status
		// Update insight status to 'resolved'
		insight.Status = "resolved"
		// Resolution notes are preserved in the audit trail; keep the original recommendation.
		resolvedAt := time.Now()
		insight.ResolvedAt = &resolvedAt
		insight.UpdatedAt = time.Now()

		if err := db.Save(&insight).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		scheduleUnifiedScoreRecalculation(db, insight.ResourceUID)

		details := "{}"
		if request.Resolution != "" {
			if b, err := json.Marshal(map[string]string{"resolution": request.Resolution}); err == nil {
				details = string(b)
			}
		}
		createInsightAuditLog(db, c, "resolve", id, details)
		appendInsightGovernanceEvent(db, c, securityaudit.ActionFindingsResolve, id, prevStatus, insight.Status, map[string]any{"resolution": request.Resolution})
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
		if !requireInsightClusterScope(db, c, insight) {
			return
		}

		prevStatus := insight.Status
		// Update insight status to 'dismissed'
		insight.Status = "dismissed"
		insight.UpdatedAt = time.Now()

		if err := db.Save(&insight).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		scheduleUnifiedScoreRecalculation(db, insight.ResourceUID)

		details := "{}"
		if request.Reason != "" {
			if b, err := json.Marshal(map[string]string{"reason": request.Reason}); err == nil {
				details = string(b)
			}
		}
		createInsightAuditLog(db, c, "dismiss", id, details)
		appendInsightGovernanceEvent(db, c, securityaudit.ActionFindingsDismiss, id, prevStatus, insight.Status, map[string]any{"reason": request.Reason})
		c.JSON(http.StatusOK, gin.H{
			"message":   "Insight dismissed successfully",
			"dismissed": true,
			"status":    insight.Status,
		})
	}
}

const maxBulkInsightIDs = 500

// BulkInsightsAction validates scope for the whole selection before any write.
// Storage failures are reported per item; successful items are audited and rescored.
// POST /risk/insights/bulk body: { "action": "acknowledge"|"resolve"|"dismiss", "insight_ids": ["id1","id2"], "resolution"?: "", "reason"?: "" }
func BulkInsightsAction(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Action     string   `json:"action" binding:"required"`
			InsightIDs []string `json:"insight_ids" binding:"required"`
			Resolution string   `json:"resolution"`
			Reason     string   `json:"reason"`
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
		// Validate every selected resource before changing any of them. This avoids
		// a late scope denial after earlier records have already been written.
		selected := make(map[string]models.Insight)
		uniqueIDs := make([]string, 0, len(body.InsightIDs))
		seen := make(map[string]bool)
		for _, raw := range body.InsightIDs {
			id := strings.TrimSpace(raw)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
			var item models.Insight
			if err := db.First(&item, "id = ?", id).Error; err != nil {
				if err != gorm.ErrRecordNotFound {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "could not validate selection"})
					return
				}
				continue
			}
			if !requireInsightClusterScope(db, c, item) {
				return
			}
			if body.Action == "acknowledge" && (item.Status == "resolved" || item.Status == "dismissed") {
				c.JSON(http.StatusConflict, gin.H{"error": "closed findings cannot be acknowledged"})
				return
			}
			selected[id] = item
		}
		body.InsightIDs = uniqueIDs
		var successCount, failedCount int
		var errors []map[string]interface{}
		affectedResourceUIDs := make(map[string]struct{})

		for _, id := range body.InsightIDs {
			insight, found := selected[id]
			if !found {
				failedCount++
				errors = append(errors, map[string]interface{}{"id": id, "error": "not found"})
				continue
			}
			switch body.Action {
			case "acknowledge":
				if err := db.Model(&insight).Updates(map[string]interface{}{"status": "acknowledged", "updated_at": time.Now()}).Error; err != nil {
					failedCount++
					errors = append(errors, map[string]interface{}{"id": id, "error": err.Error()})
					continue
				}
				createInsightAuditLog(db, c, "acknowledge", id, "{}")
				if uid := strings.TrimSpace(insight.ResourceUID); uid != "" {
					affectedResourceUIDs[uid] = struct{}{}
				}
				successCount++
			case "resolve":
				insight.Status = "resolved"
				resolvedAt := time.Now()
				insight.ResolvedAt = &resolvedAt
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
				if uid := strings.TrimSpace(insight.ResourceUID); uid != "" {
					affectedResourceUIDs[uid] = struct{}{}
				}
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
				if uid := strings.TrimSpace(insight.ResourceUID); uid != "" {
					affectedResourceUIDs[uid] = struct{}{}
				}
				successCount++
			}
		}

		// Recalculate once per affected resource to keep bulk actions efficient
		// while preserving consistent scoring behavior across all bulk actions.
		for uid := range affectedResourceUIDs {
			scheduleUnifiedScoreRecalculation(db, uid)
		}

		resp := gin.H{
			"success_count": successCount,
			"failed_count":  failedCount,
			"action":        body.Action,
		}
		if len(errors) > 0 {
			resp["errors"] = errors
		}
		if successCount > 0 {
			ev := securityaudit.FromRequest(
				c,
				authorization.ToStrings(middleware.GrantedPermissions(c)),
				c.GetString(middleware.CtxJWTSessionID),
				"findings_bulk",
				"findings",
				"bulk",
				"success",
				"medium",
				"jwt",
				nil,
				map[string]any{"action": body.Action, "success_count": successCount, "failed_count": failedCount},
				map[string]any{"insight_ids": body.InsightIDs},
				nil,
			)
			securityaudit.Append(db, &ev)
		}
		c.JSON(http.StatusOK, resp)
	}
}
