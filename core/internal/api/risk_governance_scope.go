package api

import (
	"net/http"
	"strings"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type riskGovernanceScope struct {
	clusterID  string
	clusterIDs []string
	restricted bool
}

func resolveRiskGovernanceScope(db *gorm.DB, c *gin.Context) (riskGovernanceScope, bool) {
	s := riskGovernanceScope{clusterID: strings.TrimSpace(c.Query("clusterId"))}
	raw, ok := c.Get("user")
	user, valid := raw.(*models.User)
	if !ok || !valid || user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return s, false
	}
	alias := strings.TrimSpace(c.Query("cluster"))
	if s.clusterID != "" && alias != "" && s.clusterID != alias {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "conflicting cluster filters"})
		return s, false
	}
	if s.clusterID == "" {
		s.clusterID = alias
	}
	if s.clusterID != "" && !middleware.ClusterAllowed(c, s.clusterID) {
		middleware.AbortClusterScopeDenied(db, c, s.clusterID)
		return s, false
	}
	s.clusterIDs, s.restricted = middleware.ScopedClusterIDs(c)
	return s, true
}

func (s riskGovernanceScope) podUIDs(db *gorm.DB, historical bool) *gorm.DB {
	q := db.Model(&models.Pod{}).Select("uid")
	if historical {
		q = q.Unscoped()
	}
	if s.clusterID != "" {
		q = q.Where("cluster_id = ?", s.clusterID)
	}
	if s.restricted {
		q = q.Where("cluster_id IN ?", s.clusterIDs)
	}
	return q
}

func (s riskGovernanceScope) requireResource(db *gorm.DB, c *gin.Context, uid string) bool {
	if !s.restricted && s.clusterID == "" {
		return true
	}
	clusterID, err := resourceUIDClusterID(db, uid)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve resource ownership"})
		return false
	}
	if clusterID == "" || !middleware.ClusterAllowed(c, clusterID) || (s.clusterID != "" && s.clusterID != clusterID) {
		middleware.AbortClusterScopeDenied(db, c, clusterID)
		return false
	}
	return true
}

// These legacy workers evaluate and reconcile globally. Reject scoped requests
// before constructing workers until every nested read/write supports scope.
func requireGlobalRiskEvaluation(db *gorm.DB, c *gin.Context) bool {
	s, ok := resolveRiskGovernanceScope(db, c)
	if !ok {
		return false
	}
	if s.restricted {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "global risk evaluation requires unrestricted cluster scope",
			"reason": "global_operation",
		})
		return false
	}
	if s.clusterID != "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "cluster filters are not supported for global risk evaluation"})
		return false
	}
	return true
}
