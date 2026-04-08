package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerRuntimeRoutes registers /api/v1/runtime/* (processes, network, events, metrics, signals).
// Pod-scoped routes use :uid (Kubernetes pod UID).
func registerRuntimeRoutes(api *gin.RouterGroup, db *gorm.DB) {
	rt := api.Group("/runtime")

	// Cluster-wide network activity (aggregates pod_network_connections from Pod Detail ingest).
	rt.GET("/network-activity", GetNetworkActivity(db))

	pods := rt.Group("/pods")
	pods.GET("/:uid/processes", GetPodProcessesByUID(db))
	pods.GET("/:uid/network", GetPodNetworkConnectionsByUID(db))
	pods.GET("/:uid/events", GetPodEventsByUID(db))
	pods.GET("/:uid/metrics", GetPodRuntimeMetricsByUID(db))
	pods.GET("/:uid/signals", GetRuntimeSignalsByPod(db))
	rt.GET("/signals", GetRuntimeSignalsList(db))
	rt.GET("/signals/suppression-stats", GetRuntimeSignalSuppressionStats(db))
}

// registerRuntimeV2Routes registers /api/v2/runtime/* layer-oriented runtime APIs.
func registerRuntimeV2Routes(api *gin.RouterGroup, db *gorm.DB) {
	rt := api.Group("/runtime")
	pods := rt.Group("/pods")
	pods.GET("/:uid/security-state", GetPodAssetSecurityState(db))
	pods.GET("/:uid/facts", GetPodRuntimeBehaviorFacts(db))
	pods.GET("/:uid/incidents", GetPodRuntimeIncidents(db))
	// G-API-01: formal v2 alias for pod capabilities (same handler/query as v1 inventory).
	pods.GET("/:uid/capabilities", GetPodCapabilities(db))
}
