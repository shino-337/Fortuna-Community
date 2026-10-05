package api

import (
	"strings"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// One risk level everywhere: a finding's level is the band of its resource's
// preferred risk score (risk.DeriveFinalLevelFromScore). The finding's own
// `severity` is the rule or CVE severity and is only a hint; alerts, counts,
// filters and exports that say "critical" or "high" use these bands.
const (
	riskLevelCriticalMinScore = 70.0
	riskLevelHighMinScore     = 40.0
)

// preferredRiskScoreOrderSQL picks the authoritative score row for a resource:
// the latest V3 row, else the latest row of any scorer version.
const preferredRiskScoreOrderSQL = `CASE LOWER(TRIM(COALESCE(scorer_version, ''))) WHEN 'v3' THEN 1 ELSE 0 END DESC, calculated_at DESC, id DESC`

// whereInsightRiskScoreAtLeast keeps findings whose resource's preferred score is at least min.
func whereInsightRiskScoreAtLeast(q *gorm.DB, table string, min float64) *gorm.DB {
	return q.Where(`EXISTS (
		SELECT 1 FROM `+preferredRiskScoreSubquerySQL+` AS s
		WHERE s.cluster_id = `+table+`.cluster_id
		  AND s.resource_uid = `+table+`.resource_uid
		  AND s.total_score >= ?
	)`, min)
}

func riskScoreKey(clusterID, resourceUID string) string {
	return strings.TrimSpace(clusterID) + "\x00" + strings.TrimSpace(resourceUID)
}

// preferredScoresForInsights returns the preferred total score of each finding's
// resource, keyed by riskScoreKey. Resources without a score are absent.
func preferredScoresForInsights(db *gorm.DB, insights []models.Insight) map[string]float64 {
	out := map[string]float64{}
	seen := map[string]struct{}{}
	uids := make([]string, 0, len(insights))
	for _, i := range insights {
		uid := strings.TrimSpace(i.ResourceUID)
		if uid == "" {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		uids = append(uids, uid)
	}
	if len(uids) == 0 || !db.Migrator().HasTable(&models.RiskScore{}) {
		return out
	}
	var rows []struct {
		ClusterID   string
		ResourceUID string
		TotalScore  float64
	}
	err := db.Raw(`SELECT z.cluster_id, z.resource_uid, z.total_score FROM (
		SELECT rs.cluster_id, rs.resource_uid, rs.total_score,
			ROW_NUMBER() OVER (PARTITION BY rs.cluster_id, rs.resource_uid ORDER BY `+qualifiedPreferredOrder("rs")+`) AS rn
		FROM risk_scores rs
		WHERE rs.deleted_at IS NULL AND rs.resource_uid IN ?
	) z WHERE z.rn = 1`, uids).Scan(&rows).Error
	if err != nil {
		return out
	}
	for _, r := range rows {
		out[riskScoreKey(r.ClusterID, r.ResourceUID)] = r.TotalScore
	}
	return out
}

func qualifiedPreferredOrder(alias string) string {
	return `CASE LOWER(TRIM(COALESCE(` + alias + `.scorer_version, ''))) WHEN 'v3' THEN 1 ELSE 0 END DESC, ` +
		alias + `.calculated_at DESC, ` + alias + `.id DESC`
}
