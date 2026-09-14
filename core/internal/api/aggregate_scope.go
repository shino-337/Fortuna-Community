package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sort"
)

// Authorize before cache lookup. Query scope is always narrower than user scope.
func aggregateScope(db *gorm.DB, c *gin.Context) (RiskFilter, bool) {
	filter := RiskFilter{ClusterID: c.Query("clusterId")}
	ok := applyRiskFilterClusterScope(db, c, &filter)
	return filter, ok
}

func scopedAggregateQuery(db *gorm.DB, query *gorm.DB, filter RiskFilter, column string) *gorm.DB {
	pods := db.Model(&models.Pod{}).Select("uid")
	if filter.ClusterID != "" {
		pods = pods.Where("cluster_id = ?", filter.ClusterID)
	} else if len(filter.ScopedClusterIDs) > 0 {
		pods = pods.Where("cluster_id IN ?", filter.ScopedClusterIDs)
	} else {
		return query
	}
	return query.Where(column+" IN (?)", pods)
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
