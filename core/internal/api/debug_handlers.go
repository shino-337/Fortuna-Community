package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fortuna/core/pkg/graph"
)

// GetTechniqueOverlayDigest exposes the loaded technique_overlay.yaml digest (audit / regression).
func GetTechniqueOverlayDigest(c *gin.Context) {
	c.JSON(http.StatusOK, graph.TechniqueOverlayDigest())
}
