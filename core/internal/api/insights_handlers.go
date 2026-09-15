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
func getInsightsSummaryData(db *gorm.DB, filter RiskFilter, sinceMinutes int) (InsightsSummaryResult, error) {
	summary := InsightsSummaryResult{ByType: make(map[string]int64)}
	base := func() *gorm.DB {
		q := db.Table("insights i").Where("i.deleted_at IS NULL AND (i.status IN ? OR i.status IS NULL)", []string{"active", "acknowledged"})
		q = q.Where("(i.resource_type != 'Pod' OR i.resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))")
		q = scopedAggregateQuery(db, q, filter, "i.resource_uid")
		if sinceMinutes > 0 {
			q = q.Where("i.detected_at >= ?", time.Now().Add(-time.Duration(sinceMinutes)*time.Minute))
		}
		return q
	}
	var rows []struct {
		Severity    string
		InsightType string
		Count       int64
	}
	if err := base().Select("LOWER(i.severity) AS severity, i.insight_type, COUNT(*) AS count").Group("LOWER(i.severity), i.insight_type").Scan(&rows).Error; err != nil {
		return summary, err
	}
	for _, r := range rows {
		summary.Total += r.Count
		summary.ByType[r.InsightType] += r.Count
		switch r.Severity {
		case "critical":
			summary.Critical += r.Count
		case "high":
			summary.High += r.Count
		case "medium":
			summary.Medium += r.Count
		case "low":
			summary.Low += r.Count
		}
	}
	var levels []struct {
		Level string
		Count int64
	}
	band := "CASE WHEN pref.total_score >= 70 THEN 'critical' WHEN pref.total_score >= 40 THEN 'high' WHEN pref.total_score >= 20 THEN 'medium' ELSE 'low' END"
	if err := base().Joins("INNER JOIN " + preferredRiskScoreSubquerySQL + " AS pref ON pref.resource_uid=i.resource_uid").Select(band + " AS level, COUNT(*) AS count").Group(band).Scan(&levels).Error; err != nil {
		return summary, err
	}
	if len(levels) > 0 {
		summary.RiskLevelCounts = &RiskLevelCounts{}
		for _, r := range levels {
			switch r.Level {
			case "critical":
				summary.RiskLevelCounts.Critical += r.Count
			case "high":
				summary.RiskLevelCounts.High += r.Count
			case "medium":
				summary.RiskLevelCounts.Medium += r.Count
			case "low":
				summary.RiskLevelCounts.Low += r.Count
			}
		}
	}
	return summary, nil
}

// GetInsightsSummary returns summary statistics of insights.
func GetInsightsSummary(db *gorm.DB) gin.HandlerFunc { return insightsSummaryHandler(db, false, false) }
func GetInsightsSummaryCached(db *gorm.DB) gin.HandlerFunc {
	return insightsSummaryHandler(db, true, false)
}
func GetInsightsSummaryGlobalCached(db *gorm.DB) gin.HandlerFunc {
	return insightsSummaryHandler(db, true, true)
}

func insightsSummaryHandler(db *gorm.DB, cached, global bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		filter, ok := aggregateScope(db, c)
		if !ok {
			return
		}
		if global {
			filter.ClusterID = ""
			filter.ScopedClusterIDs, _ = middleware.ScopedClusterIDs(c)
		}
		since, _ := strconv.Atoi(c.DefaultQuery("sinceMinutes", "0"))
		key := BuildInsightsSummaryCacheKey(filter.ClusterID, since)
		if global {
			key = BuildInsightsSummaryGlobalCacheKey(since)
		}
		key = authorizationCacheKey(c, key)
		if cached && defaultRisksCache != nil {
			if b, hit := defaultRisksCache.Get(key); hit {
				c.Data(http.StatusOK, "application/json", b)
				return
			}
		}
		result, err := getInsightsSummaryData(db, filter, since)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load insight summary"})
			return
		}
		if cached && defaultRisksCache != nil {
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
		filter, ok := aggregateScope(db, c)
		if !ok {
			return
		}
		var rows []row
		query := db.Table("insights i").Joins(joinCond).Where(whereBase)
		query = scopedAggregateQuery(db, query, filter, "i.resource_uid")
		if sinceMinutes > 0 {
			query = query.Where("i.detected_at >= ?", since)
		}
		if err := query.Select(`p.cluster_id, COUNT(*) AS total,
          SUM(CASE WHEN LOWER(i.severity)='critical' THEN 1 ELSE 0 END) AS critical,
          SUM(CASE WHEN LOWER(i.severity)='high' THEN 1 ELSE 0 END) AS high,
          SUM(CASE WHEN LOWER(i.severity)='medium' THEN 1 ELSE 0 END) AS medium,
          SUM(CASE WHEN LOWER(i.severity)='low' THEN 1 ELSE 0 END) AS low`).Group("p.cluster_id").Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load cluster summary"})
			return
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
		if !requireGlobalRiskEvaluation(db, c) {
			return
		}
		ctx := c.Request.Context()
		evaluator := worker.NewHistoricalRiskEvaluator(db)
		if err := evaluator.EvaluateAllResources(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "risk evaluation failed; some resources may already have been updated"})
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
		if !requireGlobalRiskEvaluation(db, c) {
			return
		}
		ctx := c.Request.Context()

		// Step 1: Re-evaluate all resources (creates/updates insights for existing risks)
		evaluator := worker.NewHistoricalRiskEvaluator(db)
		if err := evaluator.EvaluateAllResources(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "risk evaluation failed; some resources may already have been updated"})
			return
		}

		// Step 2: Auto-resolve insights where risks no longer exist
		statusUpdater := worker.NewInsightStatusUpdater(db)
		if err := statusUpdater.UpdateStatusForResolvedRisks(ctx); err != nil {
			// Evaluation succeeded, but reconciliation is incomplete.
			log.Printf("[TriggerHistoricalRiskEvaluation] Error updating insight statuses: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "evaluation completed but status reconciliation failed"})
			return
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
		needed := map[string]authorization.Permission{
			"acknowledge": authorization.PermissionFindingsAck,
			"resolve":     authorization.PermissionFindingsResolve,
			"dismiss":     authorization.PermissionFindingsDismiss,
		}[body.Action]
		if !authorization.HasPermission(middleware.GrantedPermissions(c), needed) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "required_permission": string(needed)})
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
		if len(uniqueIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least one non-empty insight ID is required"})
			return
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
