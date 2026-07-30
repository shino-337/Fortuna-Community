package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// GetPromotionRulesList returns all promotion rules. Uses GORM model so JSONB required_capabilities is scanned via JSONBStringArray.
func GetPromotionRulesList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rules []models.PromotionRule
		if err := db.Order("capability_id, signal_type, promote_to").Find(&rules).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"rules": rules,
			"count": len(rules),
		})
	}
}

// GetPromotionRulesByCapability returns promotion rules for a specific capability
func GetPromotionRulesByCapability(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		capabilityID := c.Param("capabilityId")
		if capabilityID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "capability_id is required"})
			return
		}
		var rules []models.PromotionRule
		if err := db.Where("capability_id = ?", capabilityID).Order("signal_type, promote_to").Find(&rules).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"capabilityId": capabilityID,
			"rules":       rules,
			"count":       len(rules),
		})
	}
}

// GetPromotionRulesBySignalType returns promotion rules for a specific signal type
func GetPromotionRulesBySignalType(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		signalType := c.Param("signalType")
		if signalType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "signal_type is required"})
			return
		}
		var rules []models.PromotionRule
		if err := db.Where("signal_type = ?", signalType).Order("capability_id, promote_to").Find(&rules).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"signalType": signalType,
			"rules":     rules,
			"count":     len(rules),
		})
	}
}
