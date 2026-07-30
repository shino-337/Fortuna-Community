package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

// DevelopmentPrincipal attaches a synthetic admin principal with all permissions.
// Used only when API authentication is disabled (local/dev); NOT a security boundary.
func DevelopmentPrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get(CtxPermissions); exists {
			c.Next()
			return
		}
		dev := &models.User{
			ID:       0,
			Username: "development",
			Email:    "dev@local",
			Role:     models.RoleAdmin,
			Active:   true,
		}
		c.Set("user", dev)
		c.Set("userID", dev.ID)
		c.Set("username", dev.Username)
		c.Set("role", dev.Role)
		c.Set(CtxNormalizedRole, models.RoleAdmin)
		c.Set(CtxPermissions, authorization.AllPermissions())
		c.Set("auth_source", "dev_principal")
		c.Next()
	}
}
