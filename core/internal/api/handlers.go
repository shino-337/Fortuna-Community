package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/k8s"
	"github.com/fortuna/core/pkg/models"
)

// ActiveClusterCutoff is how long since last sync to consider a cluster "active" for dashboard display.
// Clusters not synced within this window are excluded from /clusters and dashboard stats (stale data).
const ActiveClusterCutoff = 7 * 24 * time.Hour

// getClustersForAPI returns clusters for API responses (active by default; optional includeStale).
// Single source for cluster list query so GetClusters and GetClustersStats stay in sync.
func getClustersForAPI(db *gorm.DB, c *gin.Context) ([]models.Cluster, error) {
	var clusters []models.Cluster
	query := db.Model(&models.Cluster{})
	if c.Query("includeStale") != "true" {
		cutoff := time.Now().Add(-ActiveClusterCutoff)
		query = query.Where("last_sync >= ?", cutoff)
	}
	if err := query.Order("last_sync DESC").Find(&clusters).Error; err != nil {
		return nil, err
	}
	return clusters, nil
}

// GetClusters returns clusters that have synced recently (within ActiveClusterCutoff).
// Stale clusters (no sync in 7 days) are excluded so dashboard only shows current environment.
func GetClusters(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusters, err := getClustersForAPI(db, c)
		if err != nil {
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

// GetClustersStats returns clusters (with recent sync by default) and their statistics.
func GetClustersStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusters, err := getClustersForAPI(db, c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		stats := make([]ClusterStats, 0, len(clusters))

		for _, cluster := range clusters {
			stat := ClusterStats{
				Cluster: cluster,
			}

			// Count resources for this cluster (GORM automatically filters deleted_at IS NULL)
			db.Model(&models.ServiceAccount{}).Where("cluster_id = ?", cluster.ID).Count(&stat.ServiceAccountCount)
			db.Model(&models.Role{}).Where("cluster_id = ?", cluster.ID).Count(&stat.RoleCount)
			db.Model(&models.ClusterRole{}).Where("cluster_id = ?", cluster.ID).Count(&stat.ClusterRoleCount)
			db.Model(&models.RoleBinding{}).Where("cluster_id = ?", cluster.ID).Count(&stat.RoleBindingCount)
			db.Model(&models.ClusterRoleBinding{}).Where("cluster_id = ?", cluster.ID).Count(&stat.ClusterRoleBindingCount)
			// Count distinct UIDs to avoid duplicates (GORM automatically filters deleted_at IS NULL)
			var podCount int64
			db.Raw("SELECT COUNT(DISTINCT uid) FROM pods WHERE cluster_id = ? AND deleted_at IS NULL", cluster.ID).Scan(&podCount)
			stat.PodCount = podCount
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
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

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
		pageSizeParam, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))

		// Special handling: pageSize=-1 means return all records
		// For security, set a maximum limit (1000) when fetching all
		// Note: pageSize=0 is not supported as GORM treats Limit(0) specially
		fetchAll := pageSizeParam == -1

		var pageSize int
		var responsePageSize int
		if fetchAll {
			// When fetching all, use a large limit (1000) but don't apply offset
			pageSize = 1000      // Max limit for "all" requests
			responsePageSize = 0 // Will be set to actual count later
		} else {
			pageSize = pageSizeParam
			// Validate pageSize (max 1000 for normal pagination)
			// pageSize=0 is treated as invalid and defaults to 50
			if pageSize < 1 {
				pageSize = 50 // Default if invalid (including 0)
			}
			if pageSize > 1000 {
				pageSize = 1000 // Max limit
			}
			responsePageSize = pageSize
		}

		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		// Build query with pagination
		resultQuery := query
		if fetchAll {
			// For "all", apply limit but no offset (start from beginning)
			resultQuery = resultQuery.Limit(pageSize)
		} else {
			// Normal pagination with offset and limit
			resultQuery = resultQuery.Offset(offset).Limit(pageSize)
		}

		if err := resultQuery.Find(&serviceAccounts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// If fetching all, update responsePageSize to actual returned count
		if fetchAll {
			responsePageSize = len(serviceAccounts)
		}

		c.JSON(http.StatusOK, gin.H{
			"serviceAccounts": serviceAccounts,
			"total":           total,
			"page":            page,
			"pageSize":        responsePageSize,
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
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

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
			"id":                  deployment.ID,
			"clusterId":           deployment.ClusterID,
			"uid":                 deployment.UID,
			"name":                deployment.Name,
			"namespace":           deployment.Namespace,
			"replicasDesired":     deployment.Replicas,
			"replicasReady":       deployment.ReadyReplicas,
			"replicasAvailable":   deployment.AvailableReplicas,
			"replicasUnavailable": deployment.UnavailableReplicas,
			"replicasUpdated":     deployment.UpdatedReplicas,
			"strategy":            deployment.Strategy,
			"containers":          containers,
			"labels":              labels,
			"annotations":         annotations,
			"selector":            selector,
			"conditions":          conditions,
			"createdAt":           deployment.CreatedAt,
			"updatedAt":           deployment.UpdatedAt,
		}

		c.JSON(http.StatusOK, response)
	}
}

// GetReplicaSets returns all replicasets with filters and pagination
func GetReplicaSets(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var replicasets []models.ReplicaSet
		query := db.Model(&models.ReplicaSet{})
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

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
			"id":                   replicaset.ID,
			"clusterId":            replicaset.ClusterID,
			"uid":                  replicaset.UID,
			"name":                 replicaset.Name,
			"namespace":            replicaset.Namespace,
			"replicas":             replicaset.Replicas,
			"readyReplicas":        replicaset.ReadyReplicas,
			"availableReplicas":    replicaset.AvailableReplicas,
			"fullyLabeledReplicas": replicaset.FullyLabeledReplicas,
			"ownerKind":            replicaset.OwnerKind,
			"ownerName":            replicaset.OwnerName,
			"ownerUid":             replicaset.OwnerUID,
			"containers":           containers,
			"labels":               labels,
			"annotations":          annotations,
			"selector":             selector,
			"conditions":           conditions,
			"createdAt":            replicaset.CreatedAt,
			"updatedAt":            replicaset.UpdatedAt,
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

// GetPods returns all pods with optional filters
func GetPods(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var pods []models.Pod
		query := db.Model(&models.Pod{})
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Filter by service account
		if serviceAccount := c.Query("serviceAccount"); serviceAccount != "" {
			query = query.Where("service_account = ?", serviceAccount)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		// GORM automatically filters soft-deleted records
		if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&pods).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"pods":     pods,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

// GetPod returns a specific pod by ID
func GetPod(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var pod models.Pod
		// GORM automatically filters soft-deleted records
		if err := db.Preload("Cluster").First(&pod, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, pod)
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

		reports := make([]Report, 0)
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
