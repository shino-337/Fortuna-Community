package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerGraphRoutes registers /api/v1/graph/* (graph, blast-radius, shortest-path, accessible, query, attack-paths, permissions, risky-pods).
// Resource identifiers use :uid. Also registers attack-paths visualization under /graph/attack-paths/graph.
func registerGraphRoutes(api *gin.RouterGroup, db *gorm.DB) {
	g := api.Group("/graph")
	g.GET("", GetGraph(db))
	g.GET("/blast-radius/:uid", GetBlastRadius(db))
	g.GET("/shortest-path", GetShortestPath(db))
	g.GET("/accessible/:uid", GetAccessibleResources(db))
	g.POST("/query", ExecuteGraphQuery(db))
	g.GET("/attack-paths/:uid", GetAttackPaths(db))
	g.GET("/attack-paths/graph", AttackPathsGraph(db))
	g.GET("/permissions/:uid", GetServiceAccountPermissionsGraph(db))
	g.GET("/risky-pods", GetRiskyPods(db))
}
