package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetNotifications returns notifications from the notifications table (real data).
func GetNotifications(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable("notifications") {
			c.JSON(http.StatusOK, gin.H{
				"notifications": []map[string]interface{}{},
				"total":         0,
			})
			return
		}
		var list []models.Notification
		if err := db.Where("deleted_at IS NULL").Order("created_at DESC").Limit(500).Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		notifications := make([]map[string]interface{}, 0, len(list))
		for _, n := range list {
			ts := n.CreatedAt.Format(time.RFC3339)
			notifications = append(notifications, map[string]interface{}{
				"id":        n.ID,
				"title":     n.Title,
				"message":   n.Message,
				"severity":  n.Severity,
				"timestamp": ts,
				"source":    n.Source,
				"readAt":    n.ReadAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"notifications": notifications,
			"total":         len(notifications),
		})
	}
}
