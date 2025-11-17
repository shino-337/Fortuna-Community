package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ksam/core/internal/graph"
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

// GetGraph returns graph data for visualization
func GetGraph(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := c.Query("cluster")
		namespace := c.Query("namespace")

		graphService := graph.NewGraphService(db)
		graphData, err := graphService.BuildGraph(clusterID, namespace)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, graphData)
	}
}

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

