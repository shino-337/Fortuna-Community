package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetCapabilityMetadataList returns all capability metadata. Uses GORM model; preconditions/produces_attack_steps are JSONB scanned via JSONBStringArray.
func GetCapabilityMetadataList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var list []models.CapabilityMetadata
		if err := db.Order("capability_id").Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"metadata": list,
			"count":    len(list),
		})
	}
}

// GetCapabilityMetadata returns metadata for a specific capability
func GetCapabilityMetadata(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		capabilityID := c.Param("capabilityId")
		if capabilityID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "capability_id is required"})
			return
		}
		var m models.CapabilityMetadata
		if err := db.Where("capability_id = ?", capabilityID).First(&m).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "capability metadata not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, m)
	}
}
