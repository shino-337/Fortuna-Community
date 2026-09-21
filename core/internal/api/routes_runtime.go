package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
)

// registerRuntimeRoutes registers /api/v1/runtime/* (processes, network, events, metrics, signals).
// Pod-scoped routes use :uid (Kubernetes pod UID) and resolve cluster ownership before handlers run.
func registerRuntimeRoutes(api *gin.RouterGroup, db *gorm.DB) {
	rt := api.Group("/runtime")
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}

	rt.GET("/network-activity", p(authorization.PermissionRuntimeRead), GetNetworkActivity(db))

	pods := rt.Group("/pods")
	pods.Use(middleware.RequirePodUIDClusterScope(db, "uid"))
	pods.GET("/:uid/processes", p(authorization.PermissionRuntimeRead), GetPodProcessesByUIDScoped(db))
	pods.GET("/:uid/network/top-destinations", p(authorization.PermissionRuntimeRead), GetPodNetworkTopDestinationsByUIDScoped(db))
	pods.GET("/:uid/network", p(authorization.PermissionRuntimeRead), GetPodNetworkConnectionsByUIDScoped(db))
	pods.GET("/:uid/events", p(authorization.PermissionRuntimeRead), GetPodEventsByUIDScoped(db))
	pods.GET("/:uid/metrics", p(authorization.PermissionRuntimeRead), GetPodRuntimeMetricsByUIDScoped(db))
	pods.GET("/:uid/signals", p(authorization.PermissionRuntimeRead), GetRuntimeSignalsByPodScoped(db))
	rt.GET("/signals", p(authorization.PermissionRuntimeRead), GetRuntimeSignalsList(db))
	rt.GET("/signals/suppression-stats", p(authorization.PermissionRuntimeRead), GetRuntimeSignalSuppressionStats(db))
	rt.GET("/signal-step-mappings", p(authorization.PermissionRuntimeRead), GetRuntimeSignalStepMappings(db))
	rt.POST("/signal-step-mappings", p(authorization.PermissionRuntimeMappingWrite), UpsertRuntimeSignalStepMapping(db))
	rt.PATCH("/signal-step-mappings/:id/enabled", p(authorization.PermissionRuntimeMappingWrite), SetRuntimeSignalStepMappingEnabled(db))
}

// registerRuntimeV2Routes registers /api/v2/runtime/* layer-oriented runtime APIs.
func registerRuntimeV2Routes(api *gin.RouterGroup, db *gorm.DB) {
	rt := api.Group("/runtime")
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}
	pods := rt.Group("/pods")
	pods.Use(middleware.RequirePodUIDClusterScope(db, "uid"))
	pods.GET("/:uid/security-state", p(authorization.PermissionRuntimeRead), GetPodAssetSecurityState(db))
	pods.GET("/:uid/facts", p(authorization.PermissionRuntimeRead), GetPodRuntimeBehaviorFactsScoped(db))
	pods.GET("/:uid/incidents", p(authorization.PermissionRuntimeRead), GetPodRuntimeIncidentsScoped(db))
	pods.GET("/:uid/capabilities", p(authorization.PermissionInventoryRead), GetPodCapabilitiesScoped(db))
}
