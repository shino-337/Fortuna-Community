package api

import (
	"strings"

	"gorm.io/gorm"
)

// Sort keys accepted by GET /risk/insights?sort=. They match the columns of the dashboard findings table.
const (
	insightsSortScore     = "score"     // preferred risk_scores.total_score (findings without a score sort last)
	insightsSortSeverity  = "severity"  // by rank: critical > high > medium > low > info
	insightsSortDetected  = "detected"  // detected_at (first seen); default
	insightsSortUpdated   = "updated"   // updated_at (last seen / last change)
	insightsSortTitle     = "title"     // finding title
	insightsSortType      = "type"      // insight_type
	insightsSortResource  = "resource"  // resource_name
	insightsSortNamespace = "namespace" // resource_namespace
	insightsSortStatus    = "status"    // workflow status
)

// insightsListSortAliases maps accepted spellings to the canonical sort key.
var insightsListSortAliases = map[string]string{
	"score":       insightsSortScore,
	"total_score": insightsSortScore,
	"severity":    insightsSortSeverity,
	"detected":    insightsSortDetected,
	"detected_at": insightsSortDetected,
	"created":     insightsSortDetected,
	"first_seen":  insightsSortDetected,
	"updated":     insightsSortUpdated,
	"updated_at":  insightsSortUpdated,
	"last_seen":   insightsSortUpdated,
	"title":       insightsSortTitle,
	"type":        insightsSortType,
	"resource":    insightsSortResource,
	"namespace":   insightsSortNamespace,
	"status":      insightsSortStatus,
}

// insightsSeverityRankSQL orders severity by rank, not alphabetically.
const insightsSeverityRankSQL = `CASE LOWER(insights.severity) WHEN 'critical' THEN 5 WHEN 'high' THEN 4 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 WHEN 'info' THEN 1 ELSE 0 END`

// normalizeInsightsListSort validates sort/order against the allowlist. Like the other list params
// (finalLevel, scoreBin), unknown values are ignored: an unknown sort returns ("", "") and the
// default ordering applies; an unknown order falls back to the column's natural direction.
func normalizeInsightsListSort(sort, order string) (string, string) {
	key, ok := insightsListSortAliases[strings.ToLower(strings.TrimSpace(sort))]
	if !ok {
		return "", ""
	}
	switch o := strings.ToLower(strings.TrimSpace(order)); o {
	case "asc", "desc":
		return key, o
	}
	switch key {
	case insightsSortTitle, insightsSortType, insightsSortResource, insightsSortNamespace, insightsSortStatus:
		return key, "asc"
	default:
		return key, "desc"
	}
}

// applyInsightsListOrder orders the instance list. Every ordering ends with insights.id in the same
// direction so pages are stable when the sort column has ties.
func applyInsightsListOrder(query *gorm.DB, sortKey, order string) *gorm.DB {
	if sortKey == "" {
		return query.Order("insights.detected_at DESC, insights.id DESC")
	}
	dir := "DESC"
	if order == "asc" {
		dir = "ASC"
	}
	tie := "insights.id " + dir
	switch sortKey {
	case insightsSortScore:
		return query.
			Joins(`LEFT JOIN ` + preferredRiskScoreSubquerySQL + ` AS sort_pref ON sort_pref.cluster_id = insights.cluster_id AND sort_pref.resource_uid = insights.resource_uid`).
			Order("(sort_pref.total_score IS NULL) ASC, sort_pref.total_score " + dir + ", " + tie)
	case insightsSortSeverity:
		return query.Order(insightsSeverityRankSQL + " " + dir + ", " + tie)
	case insightsSortUpdated:
		return query.Order("insights.updated_at " + dir + ", " + tie)
	case insightsSortTitle:
		return query.Order("LOWER(insights.title) " + dir + ", " + tie)
	case insightsSortType:
		return query.Order("insights.insight_type " + dir + ", " + tie)
	case insightsSortResource:
		return query.Order("LOWER(insights.resource_name) " + dir + ", " + tie)
	case insightsSortNamespace:
		return query.Order("LOWER(insights.resource_namespace) " + dir + ", " + tie)
	case insightsSortStatus:
		return query.Order("insights.status " + dir + ", " + tie)
	default: // detected
		return query.Order("insights.detected_at " + dir + ", " + tie)
	}
}

// insightsGroupListOrder returns the ORDER BY for view=group. Columns without a group-level meaning
// (resource, namespace, status) keep the default group ordering. The tiebreak is the group's sample id.
func insightsGroupListOrder(sortKey, order string) string {
	const def = `(MAX(pref.total_score) IS NULL) ASC, MAX(pref.total_score) DESC, COUNT(*) DESC, MAX(insights.detected_at) DESC, MIN(insights.id) ASC`
	dir := "DESC"
	if order == "asc" {
		dir = "ASC"
	}
	tie := ", MIN(insights.id) " + dir
	switch sortKey {
	case insightsSortScore:
		return `(MAX(pref.total_score) IS NULL) ASC, MAX(pref.total_score) ` + dir + tie
	case insightsSortSeverity:
		return `MAX(` + insightsSeverityRankSQL + `) ` + dir + tie
	case insightsSortDetected:
		return `MAX(insights.detected_at) ` + dir + tie
	case insightsSortUpdated:
		return `MAX(insights.updated_at) ` + dir + tie
	case insightsSortTitle:
		return `LOWER(MAX(insights.title)) ` + dir + tie
	case insightsSortType:
		return `insights.insight_type ` + dir + tie
	default:
		return def
	}
}
