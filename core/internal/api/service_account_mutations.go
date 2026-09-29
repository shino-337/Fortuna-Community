package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/mutations"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func mutationPermission(db *gorm.DB, c *gin.Context, action string) bool {
	required := []authorization.Permission{authorization.PermissionInventoryDelete}
	if action == "revoke" {
		required = append(required, authorization.PermissionInventoryModify)
	}
	for _, p := range required {
		if !authorization.HasPermission(middleware.GrantedPermissions(c), p) {
			c.JSON(403, gin.H{"error": "forbidden", "required_permission": p})
			return false
		}
	}
	return true
}
func PreviewServiceAccountMutation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Action string `json:"action" binding:"required,oneof=revoke delete"`
		}
		if c.ShouldBindJSON(&request) != nil {
			c.JSON(400, gin.H{"error": "action must be revoke or delete"})
			return
		}
		if !mutationPermission(db, c, request.Action) {
			return
		}
		sa, ok := loadScopedServiceAccountByUID(db, c, c.Param("uid"), false)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		client, err := mutations.ClusterClient(db)(ctx, sa.ClusterID)
		if err != nil {
			c.JSON(503, gin.H{"error": "Target cluster credentials unavailable"})
			return
		}
		plan, err := mutations.Preview(ctx, client, *sa, request.Action)
		if err != nil {
			c.JSON(409, gin.H{"error": "Kubernetes preview unavailable or identity changed"})
			return
		}
		job, err := mutations.SavePreview(db.WithContext(ctx), *sa, auditUserID(c), c.GetString("username"), request.Action, plan)
		if err != nil {
			c.JSON(503, gin.H{"error": "Unable to persist preview; no Kubernetes changes made"})
			return
		}
		c.JSON(200, gin.H{"operation": job, "plan": plan})
	}
}
func scopedMutation(db *gorm.DB, c *gin.Context) (models.ServiceAccountMutation, bool) {
	var job models.ServiceAccountMutation
	scope, ok := resolveRiskGovernanceScope(db, c)
	if !ok {
		return job, false
	}
	if err := scope.apply(db.WithContext(c.Request.Context()), "cluster_id").First(&job, "id = ? AND actor_id = ?", c.Param("operationID"), auditUserID(c)).Error; err != nil {
		c.JSON(404, gin.H{"error": "Operation unavailable"})
		return job, false
	}
	return job, true
}
func ExecuteServiceAccountMutation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		job, ok := scopedMutation(db, c)
		if !ok {
			return
		}
		if !mutationPermission(db, c, job.Action) {
			return
		}
		var request struct {
			Digest string `json:"digest" binding:"required"`
		}
		if c.ShouldBindJSON(&request) != nil {
			c.JSON(400, gin.H{"error": "preview digest required"})
			return
		}
		if err := mutations.Queue(db.WithContext(c.Request.Context()), job.ID, request.Digest, auditUserID(c)); err != nil {
			c.JSON(409, gin.H{"error": "Preview expired, changed, or already executed"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"operationId": job.ID, "status": "queued"})
	}
}
func GetServiceAccountMutation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		job, ok := scopedMutation(db, c)
		if !ok {
			return
		}
		var plan mutations.Plan
		if json.Unmarshal([]byte(job.Plan), &plan) != nil {
			c.JSON(503, gin.H{"error": "Operation evidence unavailable"})
			return
		}
		c.JSON(200, gin.H{"operation": job, "plan": plan})
	}
}
