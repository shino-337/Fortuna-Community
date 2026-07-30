package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/authorization"
)

func registerInvestigationRoutes(api *gin.RouterGroup, db *gorm.DB, p func(authorization.Permission) gin.HandlerFunc) {
	inv := api.Group("/investigations")
	{
		inv.GET("", p(authorization.PermissionInvestigationsRead), ListInvestigationCases(db))
		inv.GET("/stats", p(authorization.PermissionInvestigationsRead), GetInvestigationCaseStats(db))
		inv.POST("", p(authorization.PermissionInvestigationsWrite), CreateInvestigationCase(db))
		inv.GET("/:id/timeline", p(authorization.PermissionInvestigationsRead), ListInvestigationTimeline(db))
		inv.GET("/:id", p(authorization.PermissionInvestigationsRead), GetInvestigationCase(db))
		inv.POST("/:id/pin", p(authorization.PermissionInvestigationsWrite), PinInvestigationEntity(db))
		inv.PATCH("/:id", p(authorization.PermissionInvestigationsWrite), PatchInvestigationCase(db))
		inv.DELETE("/:id", p(authorization.PermissionInvestigationsDelete), DeleteInvestigationCase(db))
	}
}
