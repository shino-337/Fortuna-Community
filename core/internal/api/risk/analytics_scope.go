package risk

import (
	"net/http"
	"strings"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// analyticsScope is resolved once per request, before any data query.
type analyticsScope struct {
	clusterID  string
	clusterIDs []string
	restricted bool
}

func resolveAnalyticsScope(db *gorm.DB, c *gin.Context) (analyticsScope, bool) {
	s := analyticsScope{clusterID: strings.TrimSpace(c.Query("cluster"))}
	alias := strings.TrimSpace(c.Query("clusterId"))
	if s.clusterID != "" && alias != "" && s.clusterID != alias {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "conflicting cluster filters"})
		return s, false
	}
	if s.clusterID == "" {
		s.clusterID = alias
	}
	raw, ok := c.Get("user")
	user, valid := raw.(*models.User)
	if !ok || !valid || user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return s, false
	}
	if s.clusterID != "" && !middleware.ClusterAllowed(c, s.clusterID) {
		middleware.AbortClusterScopeDenied(db, c, s.clusterID)
		return s, false
	}
	s.clusterIDs, s.restricted = middleware.ScopedClusterIDs(c)
	return s, true
}

func (s analyticsScope) apply(q *gorm.DB, column string) *gorm.DB {
	if s.clusterID != "" {
		q = q.Where(column+" = ?", s.clusterID)
	}
	if s.restricted {
		q = q.Where(column+" IN ?", s.clusterIDs)
	}
	return q
}

// Historical rows require retained pod ownership when a cluster filter applies.
// Orphan SBOMs cannot establish authorization from namespace or node names.
func (s analyticsScope) sboms(db *gorm.DB) *gorm.DB {
	q := db.Model(&models.SBOM{})
	if s.clusterID != "" || s.restricted {
		pods := s.apply(db.Unscoped().Model(&models.Pod{}).Select("uid"), "cluster_id")
		q = q.Where("pod_uid IN (?)", pods)
	}
	return q
}

// currentScores selects the latest V3 observation per resource before display
// filters. Filtering score first could resurrect an older, higher-risk row.
func (s analyticsScope) currentScores(db *gorm.DB) *gorm.DB {
	ranked := s.apply(db.Model(&models.RiskScore{}), "cluster_id").
		Where("LOWER(TRIM(COALESCE(scorer_version, ''))) = ?", "v3").
		Select("id, ROW_NUMBER() OVER (PARTITION BY resource_type, resource_uid, cluster_id ORDER BY calculated_at DESC, id DESC) AS score_rank")
	ids := db.Table("(?) AS ranked_scores", ranked).Select("id").Where("score_rank = 1")
	return db.Model(&models.RiskScore{}).Where("id IN (?)", ids)
}

// Sync uses retained pod ownership when scope is restricted or explicitly selected.
// Unrestricted requests retain the existing all-resource behavior.
func (s analyticsScope) syncUIDs(db *gorm.DB) ([]string, error) {
	query := db.Model(&models.Insight{}).
		Where("status IN ?", []string{"active", "acknowledged"}).
		Where("TRIM(resource_uid) != ''")
	if s.restricted || s.clusterID != "" {
		pods := s.apply(db.Unscoped().Model(&models.Pod{}).Select("uid"), "cluster_id")
		query = query.Where("resource_uid IN (?)", pods)
	}
	var uids []string
	err := query.Distinct("resource_uid").Order("resource_uid").Pluck("resource_uid", &uids).Error
	return uids, err
}
