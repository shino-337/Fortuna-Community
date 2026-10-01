package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
)

// registerGraphRoutes registers /api/v1/graph/* (relational graph and attack-path views).
// Resource identifiers use :uid. Also registers attack-paths visualization under /graph/attack-paths/graph.
func registerGraphRoutes(api *gin.RouterGroup, db *gorm.DB) {
	g := api.Group("/graph")
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}
	g.GET("", p(authorization.PermissionGraphReadSummary), GetGraph(db))
	g.GET("/attack-paths/summary", p(authorization.PermissionGraphReadPaths), GetAttackPathsSummary(db))
	g.GET("/attack-paths/chains", p(authorization.PermissionGraphReadPaths), GetAttackChains(db))
	g.GET("/attack-paths/objectives", p(authorization.PermissionGraphReadPaths), GetAttackPathObjectives(db))
	g.GET("/attack-paths/bundle", p(authorization.PermissionGraphReadPaths), GetAttackPathsBundle(db))
	g.GET("/attack-paths/graph", p(authorization.PermissionGraphReadPaths), AttackPathsGraph(db))
	g.GET("/attack-paths/:uid", p(authorization.PermissionGraphReadPaths), middleware.RequirePodUIDClusterScope(db, "uid"), GetAttackPaths(db))
}
