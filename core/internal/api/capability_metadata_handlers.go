package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/listlimit"
	"github.com/fortuna/core/pkg/models"
)

func capabilityMetadataListQuery(db *gorm.DB, domain, search string) *gorm.DB {
	q := db.Model(&models.CapabilityMetadata{})
	if d := strings.TrimSpace(domain); d != "" && strings.ToLower(d) != "all" {
		q = q.Where("domain = ?", d)
	}
	if s := strings.TrimSpace(search); s != "" {
		pat := "%" + strings.ToLower(s) + "%"
		q = q.Where(
			"LOWER(capability_id) LIKE ? OR LOWER(COALESCE(name,'')) LIKE ? OR LOWER(description) LIKE ? OR LOWER(COALESCE(summary,'')) LIKE ? OR LOWER(category) LIKE ? OR LOWER(COALESCE(mitre_technique,'')) LIKE ? OR LOWER(COALESCE(mitre_tactic,'')) LIKE ? OR LOWER(COALESCE(kill_chain_stage,'')) LIKE ?",
			pat, pat, pat, pat, pat, pat, pat, pat,
		)
	}
	return q
}

// GetCapabilityMetadataList returns capability metadata with optional search, domain filter, and pagination.
// When both `limit` and `offset` are omitted, returns the full filtered list (backward compatible with older clients).
func GetCapabilityMetadataList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		domain := c.Query("domain")
		search := c.Query("search")
		base := capabilityMetadataListQuery(db, domain, search)

		limitStr := strings.TrimSpace(c.Query("limit"))
		offsetStr := strings.TrimSpace(c.Query("offset"))
		paginate := limitStr != "" || offsetStr != ""

		if !paginate {
			// Legacy "full list" mode is still hard-capped (same cap as the
			// paginated max); total stays exact when the cap is hit.
			var list []models.CapabilityMetadata
			if err := base.Order("capability_id").Limit(capabilityMetadataUnpaginatedMax + 1).Find(&list).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			list, truncated := listlimit.Trim(list, capabilityMetadataUnpaginatedMax)
			n := len(list)
			total := int64(n)
			if truncated {
				if err := capabilityMetadataListQuery(db, domain, search).Count(&total).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"metadata":  list,
				"count":     n,
				"total":     total,
				"limit":     n,
				"offset":    0,
				"truncated": truncated,
			})
			return
		}

		var total int64
		if err := base.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		limit := 500
		if limitStr != "" {
			if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
				limit = n
				if limit > 2000 {
					limit = 2000
				}
			}
		}
		offset := 0
		if offsetStr != "" {
			if n, err := strconv.Atoi(offsetStr); err == nil && n >= 0 {
				offset = n
			}
		}

		var list []models.CapabilityMetadata
		if err := capabilityMetadataListQuery(db, domain, search).
			Order("capability_id").
			Limit(limit).
			Offset(offset).
			Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"metadata": list,
			"count":    len(list),
			"total":    total,
			"limit":    limit,
			"offset":   offset,
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
