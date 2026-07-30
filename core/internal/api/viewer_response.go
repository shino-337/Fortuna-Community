package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

func isViewerPrincipal(c *gin.Context) bool {
	return strings.EqualFold(strings.TrimSpace(c.GetString(middleware.CtxNormalizedRole)), models.RoleViewer)
}

// viewerJSON applies server-side redaction for the viewer role before writing JSON (RBAC.md §14).
func viewerJSON(c *gin.Context, code int, body interface{}) {
	if isViewerPrincipal(c) {
		body = authorization.RedactViewerValue(body)
	}
	c.JSON(code, body)
}

func viewerGraphJSON(c *gin.Context, code int, body interface{}) {
	if isViewerPrincipal(c) {
		body = authorization.RedactViewerValuePreservingStructuralKeys(body, map[string]struct{}{
			"edges":   {},
			"links":   {},
			"paths":   {},
			"results": {},
		})
	}
	c.JSON(code, body)
}
