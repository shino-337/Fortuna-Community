package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// ListSecurityActivity returns paginated security_activity_logs for investigation (requires system.audit.read).
func ListSecurityActivity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable(&models.SecurityActivityLog{}) {
			c.JSON(http.StatusOK, gin.H{"items": []any{}, "total": 0})
			return
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", ""))
		if limit < 1 {
			if v, ok := c.Get("investigation_limit_default"); ok {
				if s, ok2 := v.(string); ok2 && s != "" {
					limit, _ = strconv.Atoi(s)
				}
			}
		}
		if limit < 1 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		if offset < 0 {
			offset = 0
		}
		filtered := func() *gorm.DB {
			q := db.Model(&models.SecurityActivityLog{})
			if a := strings.TrimSpace(c.Query("action")); a != "" {
				q = q.Where("action = ?", a)
			}
			if ap := strings.TrimSpace(c.Query("action_prefix")); ap != "" {
				q = q.Where("action LIKE ?", ap+"%")
			}
			if cid := strings.TrimSpace(c.Query("correlation_id")); cid != "" {
				q = q.Where("correlation_id = ?", cid)
			}
			if rid := strings.TrimSpace(c.Query("resource_id")); rid != "" {
				q = q.Where("resource_id = ?", rid)
			}
			if dom := strings.TrimSpace(c.Query("domain")); dom != "" && dom != "all" {
				switch strings.ToLower(dom) {
				case "graph":
					q = q.Where("action LIKE ? OR resource_type = ?", "graph%", "graph")
				case "export":
					q = q.Where("action LIKE ? OR action LIKE ?", "%export%", "%insights_export%")
				case "rbac":
					q = q.Where("action LIKE ? OR action LIKE ? OR action LIKE ? OR action LIKE ?",
						"%rbac%", "%user_%", "session%", "sessions_%")
				case "findings":
					q = q.Where("action LIKE ? OR resource_type = ?", "findings%", "findings")
				case "sessions":
					q = q.Where("action LIKE ?", "%session%")
				case "auth":
					q = q.Where("result = ? OR action LIKE ?", "deny", "%login%")
				}
			}
			if rt := strings.TrimSpace(c.Query("resource_type")); rt != "" {
				q = q.Where("resource_type = ?", rt)
			}
			if sev := strings.TrimSpace(c.Query("severity")); sev != "" {
				q = q.Where("severity = ?", sev)
			}
			if r := strings.TrimSpace(c.Query("result")); r != "" {
				q = q.Where("result = ?", r)
			}
			if aid := strings.TrimSpace(c.Query("actor_user_id")); aid != "" {
				if id64, err := strconv.ParseUint(aid, 10, 32); err == nil && id64 > 0 {
					q = q.Where("actor_user_id = ?", uint(id64))
				}
			}
			if from := strings.TrimSpace(c.Query("from")); from != "" {
				if t, err := time.Parse(time.RFC3339, from); err == nil {
					q = q.Where("created_at >= ?", t)
				}
			}
			if to := strings.TrimSpace(c.Query("to")); to != "" {
				if t, err := time.Parse(time.RFC3339, to); err == nil {
					q = q.Where("created_at <= ?", t)
				}
			}
			return q
		}
		var total int64
		_ = filtered().Count(&total).Error
		var rows []models.SecurityActivityLog
		if err := filtered().Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows, "total": total, "limit": limit, "offset": offset})
	}
}

// EmergencyAccessPlaceholder returns an empty emergency-access payload until break-glass workflows ship.
func EmergencyAccessPlaceholder(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"items":       []any{},
		"message":     "Emergency access workflow is not enabled yet (reserved for future break-glass integration).",
		"placeholder": true,
	})
}
