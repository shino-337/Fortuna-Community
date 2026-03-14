package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/internal/middleware"
)

// registerInventoryRoutes registers /api/v1/inventory/* (pods, serviceaccounts, clusters, deployments, replicasets, sbom).
// All resource identifiers use :uid (Kubernetes UID); database IDs are not exposed.
func registerInventoryRoutes(api *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	inv := api.Group("/inventory")

	// Pods
	pods := inv.Group("/pods")
	pods.GET("", GetPods(db))
	pods.GET("/:uid", GetPodByUID(db))
	pods.GET("/:uid/spec", GetPodSpecYAMLByUID(db))
	pods.GET("/:uid/capabilities", GetPodCapabilities(db))
	pods.GET("/:uid/sbom", GetSBOMDetail(db))

	// SBOM list (standalone)
	inv.GET("/sbom", GetSBOMList(db))

	// ServiceAccounts
	sas := inv.Group("/serviceaccounts")
	sas.GET("", GetServiceAccounts(db))
	sas.GET("/:uid", GetServiceAccountByUID(db))
	sas.GET("/:uid/permissions", GetServiceAccountPermissionsByUID(db))
	sas.PUT("/:uid", UpdateServiceAccountByUID(db))
	sas.DELETE("/:uid", DeleteServiceAccountByUID(db))
	sas.POST("/bulk/disable", middleware.RequireAdmin(), BulkDisableServiceAccounts(db))
	sas.POST("/bulk/delete", middleware.RequireAdmin(), BulkDeleteServiceAccounts(db))
	sas.POST("/disable-inactive", middleware.RequireAdmin(), DisableInactiveServiceAccounts(db))

	// Clusters
	inv.GET("/clusters", GetClusters(db))
	inv.GET("/clusters/stats", GetClustersStats(db))
	inv.GET("/clusters/:id/overview", GetClusterOverview(db))
	inv.GET("/clusters/:id/inventory", GetClusterInventory(db))
	inv.GET("/clusters/:id/agents", GetClusterAgents(db))
	inv.GET("/clusters/:id/security-summary", GetClusterSecuritySummary(db))
	inv.GET("/clusters/:id/nodes/:nodeName", GetClusterNode(db))
	inv.GET("/clusters/:id", GetCluster(db))

	// Deployments & ReplicaSets
	inv.GET("/deployments", GetDeployments(db))
	inv.GET("/deployments/:id", GetDeployment(db))
	inv.GET("/replicasets", GetReplicaSets(db))
	inv.GET("/replicasets/:id", GetReplicaSet(db))

	// Pod capabilities (inventory metadata: securityContext, privileged, hostNetwork…)
	inv.GET("/pod-capabilities", GetPodCapabilitiesList(db))
	inv.GET("/pod-capabilities/summary", GetPodCapabilitiesSummary(db))
	inv.GET("/pod-capabilities/summary/cluster", GetPodCapabilitiesSummaryByCluster(db))
	inv.GET("/pod-capabilities/summary/capability", GetPodCapabilitiesSummaryByCapability(db))
	inv.GET("/pod-capabilities/summary/namespace", GetPodCapabilitiesSummaryByNamespace(db))
	inv.GET("/pod-capabilities/summary/severity", GetPodCapabilitiesSummaryBySeverity(db))
	inv.GET("/pod-capabilities/trends", GetPodCapabilitiesTrend(db))
}
