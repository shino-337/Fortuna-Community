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

// GetServiceAccountPermissionsByUID returns permissions for a ServiceAccount by Kubernetes UID (inventory domain).
func GetServiceAccountPermissionsByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
			return
		}
		sa, ok := loadScopedServiceAccountByUID(db, c, uid, false)
		if !ok {
			return
		}
		permissions, err := buildServiceAccountPermissions(db.WithContext(c.Request.Context()), sa)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to resolve permissions from synchronized RBAC inventory"})
			return
		}
		c.JSON(http.StatusOK, permissions)
	}
}
