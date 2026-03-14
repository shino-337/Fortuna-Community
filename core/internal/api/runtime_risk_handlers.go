package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

type PodRiskProfileDTO struct {
	PodUID       string    `json:"podUid"`
	Namespace    string    `json:"namespace"`
	StaticRisk   int       `json:"staticRisk"`
	RuntimeScore int       `json:"runtimeScore"`
	TotalRisk    int       `json:"totalRisk"`
	Capabilities []string  `json:"capabilities"`
	LastEventAt *time.Time `json:"lastEventAt,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type RuntimeEventDTO struct {
	ID          uint      `json:"id"`
	PodUID      string    `json:"podUid"`
	Namespace   string    `json:"namespace"`
	Syscall     string    `json:"syscall"`
	TargetPath  string    `json:"targetPath"`
	Capability  string    `json:"capability"`
	CreatedAt   time.Time `json:"createdAt"`
}

// GetPodRiskProfile returns risk profile for a specific pod UID.
func GetPodRiskProfile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}

		var profile models.PodRiskProfile
		if err := db.Where("pod_uid = ?", podUID).First(&profile).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "risk profile not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		totalRisk := profile.StaticRisk + profile.RuntimeScore
		if totalRisk > 100 {
			totalRisk = 100
		}

		dto := PodRiskProfileDTO{
			PodUID:       profile.PodUID,
			Namespace:    profile.Namespace,
			StaticRisk:   profile.StaticRisk,
			RuntimeScore: profile.RuntimeScore,
			TotalRisk:    totalRisk,
			Capabilities: []string(profile.Capabilities),
			LastEventAt: profile.LastEventAt,
			CreatedAt:    profile.CreatedAt,
			UpdatedAt:    profile.UpdatedAt,
		}

		c.JSON(http.StatusOK, dto)
	}
}

// GetPodRuntimeEvents returns runtime events for a specific pod UID.
func GetPodRuntimeEvents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}

		limit := 100
		if l := c.Query("limit"); l != "" {
			if parsed, err := parseInt(l); err == nil && parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}

		var events []models.RuntimeEvent
		if err := db.Where("pod_uid = ?", podUID).
			Order("created_at DESC").
			Limit(limit).
			Find(&events).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		dtos := make([]RuntimeEventDTO, len(events))
		for i, e := range events {
			dtos[i] = RuntimeEventDTO{
				ID:         e.ID,
				PodUID:     e.PodUID,
				Namespace:  e.Namespace,
				Syscall:    e.Syscall,
				TargetPath: e.TargetPath,
				Capability: e.Capability,
				CreatedAt:  e.CreatedAt,
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"podUid": podUID,
			"events": dtos,
			"total":  len(dtos),
		})
	}
}

// GetRuntimeRiskSummary returns summary of runtime risks across all pods.
func GetRuntimeRiskSummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var stats struct {
			TotalPods           int64 `json:"totalPods"`
			PodsWithRuntimeRisk int64 `json:"podsWithRuntimeRisk"`
			CriticalCount       int64 `json:"criticalCount"`
			HighCount           int64 `json:"highCount"`
			MediumCount         int64 `json:"mediumCount"`
		}

		db.Model(&models.PodRiskProfile{}).Where("pod_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL)").Count(&stats.TotalPods)
		db.Model(&models.PodRiskProfile{}).Where("runtime_score > 0 AND pod_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL)").Count(&stats.PodsWithRuntimeRisk)
		
		// Count by severity from pod_capabilities with runtime capabilities (active pods only)
		db.Model(&models.PodCapability{}).
			Joins("JOIN pods p ON p.uid = pod_capabilities.pod_uid AND p.deleted_at IS NULL").
			Where("pod_capabilities.capability_id IN (?)", []string{"ESC_RUNTIME_ACTIVE", "ESC_RUNTIME_PROBE"}).
			Where("pod_capabilities.severity = ?", "CRITICAL").
			Count(&stats.CriticalCount)
		
		db.Model(&models.PodCapability{}).
			Joins("JOIN pods p ON p.uid = pod_capabilities.pod_uid AND p.deleted_at IS NULL").
			Where("pod_capabilities.capability_id IN (?)", []string{"ESC_RUNTIME_ACTIVE", "ESC_RUNTIME_PROBE"}).
			Where("pod_capabilities.severity = ?", "HIGH").
			Count(&stats.HighCount)
		
		db.Model(&models.PodCapability{}).
			Joins("JOIN pods p ON p.uid = pod_capabilities.pod_uid AND p.deleted_at IS NULL").
			Where("pod_capabilities.capability_id IN (?)", []string{"ESC_RUNTIME_ACTIVE", "ESC_RUNTIME_PROBE"}).
			Where("pod_capabilities.severity = ?", "MEDIUM").
			Count(&stats.MediumCount)

		c.JSON(http.StatusOK, stats)
	}
}

// GetTopRuntimeRisks returns pods with highest runtime scores (active pods only).
func GetTopRuntimeRisks(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := 10
		if l := c.Query("limit"); l != "" {
			if parsed, err := parseInt(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		var profiles []models.PodRiskProfile
		if err := db.Where("runtime_score > 0 AND pod_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL)").
			Order("runtime_score DESC, updated_at DESC").
			Limit(limit).
			Find(&profiles).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		dtos := make([]PodRiskProfileDTO, len(profiles))
		for i, p := range profiles {
			totalRisk := p.StaticRisk + p.RuntimeScore
			if totalRisk > 100 {
				totalRisk = 100
			}
			dtos[i] = PodRiskProfileDTO{
				PodUID:       p.PodUID,
				Namespace:    p.Namespace,
				StaticRisk:   p.StaticRisk,
				RuntimeScore: p.RuntimeScore,
				TotalRisk:    totalRisk,
				Capabilities: []string(p.Capabilities),
				LastEventAt: p.LastEventAt,
				CreatedAt:    p.CreatedAt,
				UpdatedAt:    p.UpdatedAt,
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"pods":  dtos,
			"total": len(dtos),
		})
	}
}

func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
