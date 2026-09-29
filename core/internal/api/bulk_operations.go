package api

import (
	"context"
	"github.com/fortuna/core/internal/k8s"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/mutations"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"
	"net/http"
)

func auditUserID(c *gin.Context) uint {
	if v, ok := c.Get("userID"); ok {
		if id, ok := v.(uint); ok && id > 0 {
			return id
		}
	}
	return 0
}

type bulkServiceAccountRequest struct {
	IDs []uint `json:"ids" binding:"required,min=1,max=100"`
}

func bulkServiceAccountPermission(db *gorm.DB, c *gin.Context, action authorization.Permission) bool {
	if _, ok := resolveRiskGovernanceScope(db, c); !ok {
		return false
	}
	for _, permission := range []authorization.Permission{authorization.PermissionInventoryBulk, action} {
		if !authorization.HasPermission(middleware.GrantedPermissions(c), permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "required_permission": permission})
			return false
		}
	}
	return true
}

// Kubernetes has no ServiceAccount disabled field. Never substitute inventory deletion
// for credential revocation. Keep legacy endpoints explicit until a revocation workflow exists.
func BulkDisableServiceAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req bulkServiceAccountRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "ids must contain 1 to 100 positive IDs"})
			return
		}
		if !bulkServiceAccountPermission(db, c, authorization.PermissionInventoryModify) {
			return
		}
		c.JSON(http.StatusNotImplemented, gin.H{"error": "ServiceAccount disable is not implemented; no Kubernetes or inventory changes were made"})
	}
}
func DisableInactiveServiceAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !bulkServiceAccountPermission(db, c, authorization.PermissionInventoryModify) {
			return
		}
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Automatic ServiceAccount disable requires verified usage and credential revocation; no changes were made"})
	}
}
func BulkDeleteServiceAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !bulkServiceAccountPermission(db, c, authorization.PermissionInventoryDelete) {
			return
		}
		var req bulkServiceAccountRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "ids must contain 1 to 100 positive IDs"})
			return
		}
		unique := map[uint]bool{}
		ids := make([]uint, 0, len(req.IDs))
		for _, id := range req.IDs {
			if id == 0 {
				c.JSON(400, gin.H{"error": "IDs must be positive"})
				return
			}
			if !unique[id] {
				unique[id] = true
				ids = append(ids, id)
			}
		}
		// Authorize the entire batch before starting an irreversible Kubernetes operation.
		var sas []models.ServiceAccount
		if err := db.WithContext(c.Request.Context()).Where("id IN ?", ids).Order("id ASC").Find(&sas).Error; err != nil {
			c.JSON(500, gin.H{"error": "Unable to load ServiceAccounts"})
			return
		}
		if len(sas) != len(ids) {
			c.JSON(403, gin.H{"error": "One or more ServiceAccounts are unavailable or outside your scope; no changes made"})
			return
		}
		for i := range sas {
			if !authorizeServiceAccount(db, c, &sas[i]) {
				return
			}
		}
		type result struct {
			ID      uint   `json:"id"`
			Status  int    `json:"status"`
			Message string `json:"message"`
		}
		results := make([]result, 0, len(sas))
		count := 0
		for i := range sas {
			sa := &sas[i]
			code, message := deleteServiceAccountResource(db, c, sa)
			if code == 200 {
				count++
			}
			results = append(results, result{sa.ID, code, message})
		}
		code := http.StatusOK
		if count != len(sas) {
			code = http.StatusMultiStatus
		}
		c.JSON(code, gin.H{"count": count, "failed": len(sas) - count, "results": results, "message": "See each result for Kubernetes and inventory deletion status"})
	}
}

func deleteServiceAccountResource(db *gorm.DB, c *gin.Context, sa *models.ServiceAccount) (int, string) {
	if sa.UID == "" {
		return 409, "Kubernetes UID is required; inventory record retained"
	}
	var cluster models.Cluster
	if err := db.WithContext(c.Request.Context()).Where("id = ?", sa.ClusterID).First(&cluster).Error; err != nil {
		return 503, "Target cluster is unavailable; inventory record retained"
	}
	if cluster.Kubeconfig == "" {
		return 503, "cluster-specific Kubernetes credentials are required for deletion"
	}
	client, err := k8s.NewClientFromKubeconfig(cluster.Kubeconfig)
	if err != nil {
		return 502, "failed to initialize the target cluster client"
	}
	job, err := mutations.QueueDeletion(db.WithContext(c.Request.Context()), *sa, auditUserID(c), c.GetString("username"))
	if err != nil {
		return 503, "Unable to persist deletion intent; no Kubernetes changes made"
	}
	factory := func(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
		if clusterID != sa.ClusterID {
			return nil, mutations.ErrDrift
		}
		return client.Clientset, nil
	}
	if err = mutations.Process(c.Request.Context(), db, factory, job.ID); err != nil {
		return 503, "Deletion intent retained; worker will retry after persistence recovery"
	}
	if err = db.First(&job, "id = ?", job.ID).Error; err != nil {
		return 503, "Deletion status unavailable; durable intent retained"
	}
	if job.Status == "succeeded" {
		return 200, "ServiceAccount deleted from Kubernetes and inventory"
	}
	if job.Status == "blocked" {
		return 409, "Kubernetes identity or permission changed; review a new mutation preview"
	}
	return 502, "Kubernetes deletion pending durable retry; inventory record retained"
}
