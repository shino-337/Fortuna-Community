package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
)

// registerInventoryRoutes registers /api/v1/inventory/* (pods, serviceaccounts, clusters, deployments, replicasets, sbom).
// All resource identifiers use :uid (Kubernetes UID); database IDs are not exposed.
func registerInventoryRoutes(api *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	inv := api.Group("/inventory")
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}

	// Pods
	pods := inv.Group("/pods")
	pods.GET("", p(authorization.PermissionInventoryRead), GetPods(db))
	pods.GET("/:uid", p(authorization.PermissionInventoryRead), middleware.RequirePodUIDClusterScope(db, "uid"), GetPodByUIDScoped(db))
	pods.GET("/:uid/spec", p(authorization.PermissionInventoryRead), middleware.RequirePodUIDClusterScope(db, "uid"), GetPodSpecYAMLByUID(db))
	pods.GET("/:uid/capabilities", p(authorization.PermissionInventoryRead), middleware.RequirePodUIDClusterScope(db, "uid"), GetPodCapabilities(db))
	pods.GET("/:uid/sbom", p(authorization.PermissionInventoryRead), middleware.RequirePodUIDClusterScope(db, "uid"), GetSBOMDetail(db))

	// SBOM list (standalone)
	inv.GET("/sbom", p(authorization.PermissionInventoryRead), GetSBOMList(db))

	// ServiceAccounts
	sas := inv.Group("/serviceaccounts")
	sas.GET("", p(authorization.PermissionInventoryRead), GetServiceAccounts(db))
	sas.GET("/:uid", p(authorization.PermissionInventoryRead), GetServiceAccountByUID(db))
	sas.GET("/:uid/permissions", p(authorization.PermissionInventoryRead), GetServiceAccountPermissionsByUID(db))
	sas.PUT("/:uid", p(authorization.PermissionInventoryModify), UpdateServiceAccountByUID(db))
	sas.DELETE("/:uid", p(authorization.PermissionInventoryDelete), DeleteServiceAccountByUID(db))
	sas.POST("/bulk/disable", p(authorization.PermissionInventoryBulk), BulkDisableServiceAccounts(db))
	sas.POST("/bulk/delete", p(authorization.PermissionInventoryBulk), BulkDeleteServiceAccounts(db))
	sas.POST("/disable-inactive", p(authorization.PermissionInventoryBulk), DisableInactiveServiceAccounts(db))

	// Clusters (list + stats global; :id routes enforce optional user cluster scope)
	inv.GET("/clusters", p(authorization.PermissionInventoryRead), GetClusters(db))
	inv.GET("/clusters/stats", p(authorization.PermissionInventoryRead), GetClustersStats(db))

	cl := inv.Group("/clusters")
	cl.Use(middleware.RequireClusterScope(db, "id"))
	cl.GET("/:id/overview", p(authorization.PermissionInventoryRead), GetClusterOverview(db))
	cl.GET("/:id/inventory", p(authorization.PermissionInventoryRead), GetClusterInventory(db))
	cl.GET("/:id/agents", p(authorization.PermissionInventoryRead), GetClusterAgents(db))
	cl.GET("/:id/security-summary", p(authorization.PermissionInventoryRead), GetClusterSecuritySummary(db))
	cl.GET("/:id/nodes/:nodeName", p(authorization.PermissionInventoryRead), GetClusterNode(db))
	cl.GET("/:id", p(authorization.PermissionInventoryRead), GetCluster(db))

	// Deployments & ReplicaSets
	inv.GET("/deployments", p(authorization.PermissionInventoryRead), GetDeployments(db))
	inv.GET("/deployments/:id", p(authorization.PermissionInventoryRead), GetDeployment(db))
	inv.GET("/replicasets", p(authorization.PermissionInventoryRead), GetReplicaSets(db))
	inv.GET("/replicasets/:id", p(authorization.PermissionInventoryRead), GetReplicaSet(db))

	// Pod capabilities (inventory metadata: securityContext, privileged, hostNetwork…)
	inv.GET("/pod-capabilities", p(authorization.PermissionInventoryRead), GetPodCapabilitiesList(db))
	inv.GET("/pod-capabilities/summary", p(authorization.PermissionInventoryRead), GetPodCapabilitiesSummary(db))
	inv.GET("/pod-capabilities/summary/cluster", p(authorization.PermissionInventoryRead), GetPodCapabilitiesSummaryByCluster(db))
	inv.GET("/pod-capabilities/summary/capability", p(authorization.PermissionInventoryRead), GetPodCapabilitiesSummaryByCapability(db))
	inv.GET("/pod-capabilities/summary/namespace", p(authorization.PermissionInventoryRead), GetPodCapabilitiesSummaryByNamespace(db))
	inv.GET("/pod-capabilities/summary/severity", p(authorization.PermissionInventoryRead), GetPodCapabilitiesSummaryBySeverity(db))
	inv.GET("/pod-capabilities/trends", p(authorization.PermissionInventoryRead), GetPodCapabilitiesTrend(db))
}
