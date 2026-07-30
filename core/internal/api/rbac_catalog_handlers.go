package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fortuna/core/pkg/authorization"
)

// GetRBACPermissionCatalog returns the v2 permission taxonomy for governance UIs (admin audit role).
func GetRBACPermissionCatalog(c *gin.Context) {
	type row struct {
		Permission string `json:"permission"`
		Level      string `json:"level"`
	}
	out := make([]row, 0, len(authorization.AllPermissions()))
	for _, p := range authorization.AllPermissions() {
		out = append(out, row{Permission: string(p), Level: string(authorization.ClassifyPermission(p))})
	}
	c.JSON(http.StatusOK, gin.H{"permissions": out})
}
