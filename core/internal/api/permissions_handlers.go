package api

import (
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/rbacinventory"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	rbacv1 "k8s.io/api/rbac/v1"
	"net/http"
)

type ServiceAccountPermissions = rbacinventory.ServiceAccountPermissions
type RoleBindingPermission = rbacinventory.RoleBindingPermission
type ClusterRoleBindingPermission = rbacinventory.ClusterRoleBindingPermission
type Rule = rbacinventory.Rule

func serviceAccountSubjectMatches(subject rbacv1.Subject, namespace string, sa *models.ServiceAccount) bool {
	return rbacinventory.SubjectMatches(subject, namespace, sa)
}
func buildServiceAccountPermissions(db *gorm.DB, sa *models.ServiceAccount) (ServiceAccountPermissions, error) {
	return rbacinventory.Resolve(db, sa)
}

// GetServiceAccountPermissions returns permissions for a specific ServiceAccount (by DB id).
func GetServiceAccountPermissions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sa models.ServiceAccount
		if err := db.First(&sa, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load ServiceAccount"})
			return
		}
		if !authorizeServiceAccount(db, c, &sa) {
			return
		}
		permissions, err := buildServiceAccountPermissions(db.WithContext(c.Request.Context()), &sa)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to resolve permissions from synchronized RBAC inventory"})
			return
		}
		c.JSON(http.StatusOK, permissions)
	}
}

// GetServiceAccountPermissionsByUID returns permissions for a ServiceAccount by Kubernetes UID (inventory domain).
func GetServiceAccountPermissionsByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
			return
		}
		var sa models.ServiceAccount
		if err := db.Where("uid = ?", uid).First(&sa).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load ServiceAccount"})
			return
		}
		if !authorizeServiceAccount(db, c, &sa) {
			return
		}
		permissions, err := buildServiceAccountPermissions(db.WithContext(c.Request.Context()), &sa)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to resolve permissions from synchronized RBAC inventory"})
			return
		}
		c.JSON(http.StatusOK, permissions)
	}
}
