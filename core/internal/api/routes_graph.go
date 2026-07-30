package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
)

// registerGraphRoutes registers /api/v1/graph/* (graph, blast-radius, shortest-path, accessible, query, attack-paths, permissions, risky-pods).
// Resource identifiers use :uid. Also registers attack-paths visualization under /graph/attack-paths/graph.
func registerGraphRoutes(api *gin.RouterGroup, db *gorm.DB) {
	g := api.Group("/graph")
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}
	graphTraversal := func() gin.HandlerFunc {
		return middleware.RequireAnyPermission(db,
			authorization.PermissionGraphQueryEntity,
			authorization.PermissionGraphQueryTraversal,
			authorization.PermissionGraphQueryAdvanced,
		)
	}

	g.GET("", p(authorization.PermissionGraphReadSummary), GetGraph(db))
	g.GET("/blast-radius/:uid", p(authorization.PermissionGraphReadPaths), GetBlastRadius(db))
	g.GET("/shortest-path", graphTraversal(), GetShortestPath(db))
	g.GET("/accessible/:uid", graphTraversal(), GetAccessibleResources(db))
	g.POST("/query", p(authorization.PermissionGraphQueryAdvanced), middleware.GraphQueryRateLimit(), ExecuteGraphQuery(db))
	g.GET("/attack-paths/summary", p(authorization.PermissionGraphReadPaths), GetAttackPathsSummary(db))
	g.GET("/attack-paths/chains", p(authorization.PermissionGraphReadPaths), GetAttackChains(db))
	g.GET("/attack-paths/objectives", p(authorization.PermissionGraphReadPaths), GetAttackPathObjectives(db))
	g.GET("/attack-paths/bundle", p(authorization.PermissionGraphReadPaths), GetAttackPathsBundle(db))
	g.GET("/attack-paths/graph", p(authorization.PermissionGraphReadPaths), AttackPathsGraph(db))
	g.GET("/attack-paths/:uid", p(authorization.PermissionGraphReadPaths), GetAttackPaths(db))
	g.GET("/permissions/:uid", p(authorization.PermissionGraphQueryEntity), GetServiceAccountPermissionsGraph(db))
	g.GET("/risky-pods", p(authorization.PermissionGraphReadPaths), GetRiskyPods(db))
}
