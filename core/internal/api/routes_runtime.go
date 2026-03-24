package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerRuntimeRoutes registers /api/v1/runtime/* (processes, network, events, metrics, signals).
// Pod-scoped routes use :uid (Kubernetes pod UID).
func registerRuntimeRoutes(api *gin.RouterGroup, db *gorm.DB) {
	rt := api.Group("/runtime")

	pods := rt.Group("/pods")
	pods.GET("/:uid/processes", GetPodProcessesByUID(db))
	pods.GET("/:uid/network", GetPodNetworkConnectionsByUID(db))
	pods.GET("/:uid/events", GetPodEventsByUID(db))
	pods.GET("/:uid/metrics", GetPodRuntimeMetricsByUID(db))
	pods.GET("/:uid/signals", GetRuntimeSignalsByPod(db))

	rt.POST("/events", PostRuntimeEvents(db))
	rt.GET("/signals", GetRuntimeSignalsList(db))
	rt.GET("/signals/suppression-stats", GetRuntimeSignalSuppressionStats(db))
}
