package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ksam/core/internal/k8s"
	"github.com/ksam/core/pkg/models"
)

// GetClusters returns all clusters
func GetClusters(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var clusters []models.Cluster
		if err := db.Find(&clusters).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"clusters": clusters})
	}
}

// GetCluster returns a specific cluster
func GetCluster(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var cluster models.Cluster
		if err := db.First(&cluster, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cluster)
	}
}

// ClusterStats represents cluster statistics and status
type ClusterStats struct {
	models.Cluster
	ServiceAccountCount     int64  `json:"serviceAccountCount"`
	RoleCount               int64  `json:"roleCount"`
	ClusterRoleCount        int64  `json:"clusterRoleCount"`
	RoleBindingCount        int64  `json:"roleBindingCount"`
	ClusterRoleBindingCount int64  `json:"clusterRoleBindingCount"`
	PodCount                int64  `json:"podCount"`
	DeploymentCount         int64  `json:"deploymentCount"`
	ConnectionStatus        string `json:"connectionStatus"` // connected, disconnected, unknown
	AgentVersion            string `json:"agentVersion,omitempty"`
}

// GetClustersStats returns all clusters with detailed statistics and connection status
func GetClustersStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var clusters []models.Cluster
		if err := db.Find(&clusters).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		stats := make([]ClusterStats, 0, len(clusters))
		
		for _, cluster := range clusters {
			stat := ClusterStats{
				Cluster: cluster,
			}

			// Count resources for this cluster
			db.Model(&models.ServiceAccount{}).Where("cluster_id = ?", cluster.ID).Count(&stat.ServiceAccountCount)
			db.Model(&models.Role{}).Where("cluster_id = ?", cluster.ID).Count(&stat.RoleCount)
			db.Model(&models.ClusterRole{}).Where("cluster_id = ?", cluster.ID).Count(&stat.ClusterRoleCount)
			db.Model(&models.RoleBinding{}).Where("cluster_id = ?", cluster.ID).Count(&stat.RoleBindingCount)
			db.Model(&models.ClusterRoleBinding{}).Where("cluster_id = ?", cluster.ID).Count(&stat.ClusterRoleBindingCount)
			db.Model(&models.Pod{}).Where("cluster_id = ?", cluster.ID).Count(&stat.PodCount)
			db.Model(&models.Deployment{}).Where("cluster_id = ?", cluster.ID).Count(&stat.DeploymentCount)

			// Determine connection status based on LastSync time
			// If lastSync is within last 5 minutes, consider connected
			// If status is "error", mark as disconnected
			// Otherwise unknown
			timeSinceSync := time.Since(cluster.LastSync)
			if cluster.Status == "error" {
				stat.ConnectionStatus = "disconnected"
			} else if timeSinceSync < 5*time.Minute {
				stat.ConnectionStatus = "connected"
			} else if timeSinceSync < 30*time.Minute {
				stat.ConnectionStatus = "degraded"
			} else {
				stat.ConnectionStatus = "disconnected"
			}

			// Agent version could be stored in cluster metadata in future
			// For now, leaving it empty
			stat.AgentVersion = "v1.0.0"

			stats = append(stats, stat)
		}

		c.JSON(http.StatusOK, gin.H{
			"clusters": stats,
			"total":    len(stats),
		})
	}
}

// GetServiceAccounts returns all service accounts with optional filters
func GetServiceAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var serviceAccounts []models.ServiceAccount
		query := db.Model(&models.ServiceAccount{})

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		if err := query.Offset(offset).Limit(pageSize).Find(&serviceAccounts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"serviceAccounts": serviceAccounts,
			"total":           total,
			"page":            page,
			"pageSize":        pageSize,
		})
	}
}

// GetServiceAccount returns a specific service account
func GetServiceAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sa models.ServiceAccount
		if err := db.Preload("Cluster").First(&sa, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sa)
	}
}

// GetDeployments returns all deployments with filtering and pagination
func GetDeployments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var deployments []models.Deployment
		query := db.Model(&models.Deployment{})

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&deployments).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"deployments": deployments,
			"total":       total,
			"page":        page,
			"pageSize":    pageSize,
		})
	}
}

// GetDeployment returns a specific deployment
func GetDeployment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var deployment models.Deployment
		if err := db.Preload("Cluster").First(&deployment, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Parse JSON fields for frontend
		var containers []interface{}
		var labels map[string]string
		var annotations map[string]string
		var selector map[string]string
		var conditions []interface{}
		var template map[string]interface{}

		json.Unmarshal([]byte(deployment.Template), &template)
		json.Unmarshal([]byte(deployment.Labels), &labels)
		json.Unmarshal([]byte(deployment.Annotations), &annotations)
		json.Unmarshal([]byte(deployment.Selector), &selector)
		json.Unmarshal([]byte(deployment.Conditions), &conditions)

		// Extract containers from template
		if template != nil {
			if containersList, ok := template["containers"]; ok {
				containers, _ = containersList.([]interface{})
			}
		}

		response := gin.H{
			"id":                    deployment.ID,
			"clusterId":             deployment.ClusterID,
			"uid":                   deployment.UID,
			"name":                  deployment.Name,
			"namespace":             deployment.Namespace,
			"replicasDesired":       deployment.Replicas,
			"replicasReady":         deployment.ReadyReplicas,
			"replicasAvailable":     deployment.AvailableReplicas,
			"replicasUnavailable":   deployment.UnavailableReplicas,
			"replicasUpdated":       deployment.UpdatedReplicas,
			"strategy":              deployment.Strategy,
			"containers":            containers,
			"labels":                labels,
			"annotations":           annotations,
			"selector":              selector,
			"conditions":            conditions,
			"createdAt":             deployment.CreatedAt,
			"updatedAt":             deployment.UpdatedAt,
		}

		c.JSON(http.StatusOK, response)
	}
}

// GetReplicaSets returns all replicasets with filters and pagination
func GetReplicaSets(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var replicasets []models.ReplicaSet
		query := db.Model(&models.ReplicaSet{})

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Filter by owner
		if ownerKind := c.Query("ownerKind"); ownerKind != "" {
			query = query.Where("owner_kind = ?", ownerKind)
		}
		if ownerName := c.Query("ownerName"); ownerName != "" {
			query = query.Where("owner_name = ?", ownerName)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&replicasets).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"replicasets": replicasets,
			"total":       total,
			"page":        page,
			"pageSize":    pageSize,
		})
	}
}

// GetReplicaSet returns a specific replicaset
func GetReplicaSet(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var replicaset models.ReplicaSet
		if err := db.Preload("Cluster").First(&replicaset, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ReplicaSet not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Parse JSON fields for frontend
		var containers []interface{}
		var labels map[string]string
		var annotations map[string]string
		var selector map[string]string
		var conditions []interface{}
		var template map[string]interface{}

		json.Unmarshal([]byte(replicaset.Template), &template)
		json.Unmarshal([]byte(replicaset.Labels), &labels)
		json.Unmarshal([]byte(replicaset.Annotations), &annotations)
		json.Unmarshal([]byte(replicaset.Selector), &selector)
		json.Unmarshal([]byte(replicaset.Conditions), &conditions)

		// Extract containers from template
		if template != nil {
			if containersList, ok := template["containers"]; ok {
				containers, _ = containersList.([]interface{})
			}
		}

		response := gin.H{
			"id":                    replicaset.ID,
			"clusterId":             replicaset.ClusterID,
			"uid":                   replicaset.UID,
			"name":                  replicaset.Name,
			"namespace":             replicaset.Namespace,
			"replicas":              replicaset.Replicas,
			"readyReplicas":         replicaset.ReadyReplicas,
			"availableReplicas":     replicaset.AvailableReplicas,
			"fullyLabeledReplicas":  replicaset.FullyLabeledReplicas,
			"ownerKind":             replicaset.OwnerKind,
			"ownerName":             replicaset.OwnerName,
			"ownerUid":              replicaset.OwnerUID,
			"containers":            containers,
			"labels":                labels,
			"annotations":           annotations,
			"selector":              selector,
			"conditions":            conditions,
			"createdAt":             replicaset.CreatedAt,
			"updatedAt":             replicaset.UpdatedAt,
		}

		c.JSON(http.StatusOK, response)
	}
}

// UpdateServiceAccount updates a service account
func UpdateServiceAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sa models.ServiceAccount
		if err := db.First(&sa, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var updateData map[string]interface{}
		if err := c.ShouldBindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Model(&sa).Updates(updateData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log audit
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID.(uint),
			Action:     "update",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username.(string),
			IP:         c.ClientIP(),
		}
		db.Create(&auditLog)

		c.JSON(http.StatusOK, sa)
	}
}

// DeleteServiceAccount deletes a service account
func DeleteServiceAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sa models.ServiceAccount
		if err := db.Preload("Cluster").First(&sa, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Try to delete from Kubernetes cluster first
		var k8sErr error
		if sa.Cluster.Kubeconfig != "" {
			// Use kubeconfig from cluster
			k8sClient, err := k8s.NewClientFromKubeconfig(sa.Cluster.Kubeconfig)
			if err == nil {
				k8sErr = k8sClient.DeleteServiceAccount(sa.Namespace, sa.Name)
			}
		} else {
			// Try to use default kubeconfig or in-cluster config
			k8sClient, err := k8s.NewClientFromPath("")
			if err == nil {
				k8sErr = k8sClient.DeleteServiceAccount(sa.Namespace, sa.Name)
			}
		}

		// Log audit before deletion
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID.(uint),
			Action:     "delete",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username.(string),
			IP:         c.ClientIP(),
		}
		
		// Add K8s deletion result to audit details
		if k8sErr != nil {
			auditLog.Details = fmt.Sprintf(`{"k8s_deletion":"failed","error":"%s"}`, k8sErr.Error())
		} else {
			auditLog.Details = `{"k8s_deletion":"success"}`
		}
		db.Create(&auditLog)

		// Delete from database
		if err := db.Delete(&sa).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Return success even if K8s deletion failed (for backward compatibility)
		message := "ServiceAccount deleted from database"
		if k8sErr == nil {
			message = "ServiceAccount deleted from Kubernetes cluster and database"
		} else {
			message = fmt.Sprintf("ServiceAccount deleted from database, but failed to delete from Kubernetes: %v", k8sErr)
		}

		c.JSON(http.StatusOK, gin.H{"message": message})
	}
}

// GetGraph is now in graph_handlers.go

// GetAuditLogs returns audit logs
func GetAuditLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var logs []models.AuditLog
		query := db.Model(&models.AuditLog{})

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by resource
		if resource := c.Query("resource"); resource != "" {
			query = query.Where("resource = ?", resource)
		}

		// Filter by action
		if action := c.Query("action"); action != "" {
			query = query.Where("action = ?", action)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"logs":     logs,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

// GetAuditReports returns audit reports
func GetAuditReports(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		type Report struct {
			Resource string `json:"resource"`
			Action   string `json:"action"`
			Count    int64  `json:"count"`
		}

		var reports []Report
		if err := db.Model(&models.AuditLog{}).
			Select("resource, action, COUNT(*) as count").
			Group("resource, action").
			Scan(&reports).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reports": reports})
	}
}

