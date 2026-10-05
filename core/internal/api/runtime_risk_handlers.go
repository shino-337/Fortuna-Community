package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

type PodRiskProfileDTO struct {
	PodUID       string     `json:"podUid"`
	Namespace    string     `json:"namespace"`
	StaticRisk   int        `json:"staticRisk"`
	RuntimeScore int        `json:"runtimeScore"`
	TotalRisk    int        `json:"totalRisk"`
	Capabilities []string   `json:"capabilities"`
	LastEventAt  *time.Time `json:"lastEventAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type RuntimeEventDTO struct {
	ID              uint       `json:"id"`
	EventID         string     `json:"eventId,omitempty"`
	ObservedAt      *time.Time `json:"observedAt,omitempty"`
	IngestedAt      *time.Time `json:"ingestedAt,omitempty"`
	ResolutionState string     `json:"resolutionState,omitempty"`
	SourceKind      string     `json:"sourceKind,omitempty"`
	SourceSensorID  string     `json:"sourceSensorId,omitempty"`
	SourceRule      string     `json:"sourceRule,omitempty"`
	PodUID          string     `json:"podUid"`
	PodName         string     `json:"podName,omitempty"`
	Namespace       string     `json:"namespace"`
	NodeName        string     `json:"nodeName,omitempty"`
	Runtime         string     `json:"runtime,omitempty"`
	EventType       string     `json:"eventType,omitempty"`
	Signal          string     `json:"signal,omitempty"`
	Mitre           string     `json:"mitreTechnique,omitempty"`
	Severity        string     `json:"severity,omitempty"`
	Confidence      float64    `json:"confidence,omitempty"`
	Syscall         string     `json:"syscall"`
	TargetPath      string     `json:"targetPath"`
	Capability      string     `json:"capability"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// GetPodRiskProfile returns risk profile for a specific pod UID.
func GetPodRiskProfile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}

		clusterID, ok := middleware.ResolvedPodClusterID(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "resolved pod cluster is required"})
			return
		}
		var profile models.PodRiskProfile
		if err := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).First(&profile).Error; err != nil {
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
			LastEventAt:  profile.LastEventAt,
			CreatedAt:    profile.CreatedAt,
			UpdatedAt:    profile.UpdatedAt,
		}

		c.JSON(http.StatusOK, dto)
	}
}

// GetPodRuntimeEvents returns runtime events for a specific pod UID.
// GetPodRuntimeEvents is retained only for source compatibility.
