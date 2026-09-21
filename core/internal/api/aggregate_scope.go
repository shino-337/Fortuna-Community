package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/fortuna/core/internal/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sort"
	"strings"
)

// Authorize before cache lookup. Query scope is always narrower than user scope.
func aggregateScope(db *gorm.DB, c *gin.Context) (RiskFilter, bool) {
	filter := RiskFilter{ClusterID: c.Query("clusterId")}
	ok := applyRiskFilterClusterScope(db, c, &filter)
	return filter, ok
}

func scopedAggregateQuery(_ *gorm.DB, query *gorm.DB, filter RiskFilter, column string) *gorm.DB {
	// All current callers scope Insight rows. Scope directly by the persisted
	// cluster identity instead of projecting through Pod UID, because Pod UID is
	// only unique inside a cluster.
	clusterColumn := "cluster_id"
	if dot := strings.LastIndex(column, "."); dot >= 0 {
		clusterColumn = column[:dot+1] + "cluster_id"
	}
	if filter.ClusterID != "" {
		return query.Where(clusterColumn+" = ?", filter.ClusterID)
	}
	if len(filter.ScopedClusterIDs) > 0 {
		return query.Where(clusterColumn+" IN ?", filter.ScopedClusterIDs)
	}
	return query
}

func authorizationCacheKey(c *gin.Context, key string) string {
	ids, restricted := middleware.ScopedClusterIDs(c)
	ids = append([]string(nil), ids...)
	sort.Strings(ids)
	return encodedCacheKey(key+":scope:", struct {
		Restricted bool
		IDs        []string
	}{restricted, ids})
}

func encodedCacheKey(prefix string, value interface{}) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return prefix + hex.EncodeToString(sum[:])
}
