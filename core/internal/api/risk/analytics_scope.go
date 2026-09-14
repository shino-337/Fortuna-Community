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
