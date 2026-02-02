package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetResources returns a normalized resource list for the dashboard explorer.
// Query params: kind, namespace, cluster
func GetResources(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		kind := c.Query("kind")
		namespace := c.Query("namespace")
		clusterID := c.Query("cluster")

		resources := make([]map[string]interface{}, 0)

		appendResource := func(kind string, name string, namespace string, uid string, clusterID string) {
			resources = append(resources, map[string]interface{}{
				"kind":      kind,
				"name":      name,
				"namespace": namespace,
				"uid":       uid,
				"clusterId": clusterID,
			})
		}

		if kind == "" || kind == "Pod" {
			var pods []models.Pod
			query := db.Model(&models.Pod{})
			if namespace != "" {
				query = query.Where("namespace = ?", namespace)
			}
			if clusterID != "" {
				query = query.Where("cluster_id = ?", clusterID)
			}
			query.Find(&pods)
			for _, pod := range pods {
				appendResource("Pod", pod.Name, pod.Namespace, pod.UID, pod.ClusterID)
			}
		}

		if kind == "" || kind == "ServiceAccount" {
			var sas []models.ServiceAccount
			query := db.Model(&models.ServiceAccount{})
			if namespace != "" {
				query = query.Where("namespace = ?", namespace)
			}
			if clusterID != "" {
				query = query.Where("cluster_id = ?", clusterID)
			}
			query.Find(&sas)
			for _, sa := range sas {
				appendResource("ServiceAccount", sa.Name, sa.Namespace, sa.UID, sa.ClusterID)
			}
		}

		if kind == "" || kind == "Role" {
			var roles []models.Role
			query := db.Model(&models.Role{})
			if namespace != "" {
				query = query.Where("namespace = ?", namespace)
			}
			if clusterID != "" {
				query = query.Where("cluster_id = ?", clusterID)
			}
			query.Find(&roles)
			for _, role := range roles {
				appendResource("Role", role.Name, role.Namespace, role.UID, role.ClusterID)
			}
		}

		if kind == "" || kind == "ClusterRole" {
			var roles []models.ClusterRole
			query := db.Model(&models.ClusterRole{})
			if clusterID != "" {
				query = query.Where("cluster_id = ?", clusterID)
			}
			query.Find(&roles)
			for _, role := range roles {
				appendResource("ClusterRole", role.Name, "", role.UID, role.ClusterID)
			}
		}

		if kind == "" || kind == "RoleBinding" {
			var bindings []models.RoleBinding
			query := db.Model(&models.RoleBinding{})
			if namespace != "" {
				query = query.Where("namespace = ?", namespace)
			}
			if clusterID != "" {
				query = query.Where("cluster_id = ?", clusterID)
			}
			query.Find(&bindings)
			for _, binding := range bindings {
				appendResource("RoleBinding", binding.Name, binding.Namespace, binding.UID, binding.ClusterID)
			}
		}

		if kind == "" || kind == "ClusterRoleBinding" {
			var bindings []models.ClusterRoleBinding
			query := db.Model(&models.ClusterRoleBinding{})
			if clusterID != "" {
				query = query.Where("cluster_id = ?", clusterID)
			}
			query.Find(&bindings)
			for _, binding := range bindings {
				appendResource("ClusterRoleBinding", binding.Name, "", binding.UID, binding.ClusterID)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"resources": resources,
			"total":     len(resources),
		})
	}
}
