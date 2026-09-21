package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func resolvedRuntimePodIdentity(c *gin.Context) (string, string, bool) {
	podUID := c.Param("uid")
	if podUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
		return "", "", false
	}
	clusterID, ok := middleware.ResolvedPodClusterID(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolved pod cluster is required"})
		return "", "", false
	}
	return clusterID, podUID, true
}

func runtimeReadLimit(c *gin.Context, defaultLimit int) int {
	limit := defaultLimit
	if raw := c.Query("limit"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 && value <= 1000 {
			limit = value
		}
	}
	return limit
}

// GetPodRuntimeEventsScoped returns events for exactly one canonical Pod identity.
func GetPodRuntimeEventsScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, podUID, ok := resolvedRuntimePodIdentity(c)
		if !ok {
			return
		}
		var events []models.RuntimeEvent
		if err := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).
			Order("created_at DESC").
			Limit(runtimeReadLimit(c, 100)).
			Find(&events).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		dtos := make([]RuntimeEventDTO, len(events))
		for i, e := range events {
			dtos[i] = RuntimeEventDTO{
				ID: e.ID, EventID: e.EventID, ObservedAt: e.ObservedAt, IngestedAt: e.IngestedAt,
				ResolutionState: e.ResolutionState, SourceKind: e.SourceKind, SourceSensorID: e.SourceSensorID,
				SourceRule: e.SourceRule, PodUID: e.PodUID, PodName: e.PodName, Namespace: e.Namespace,
				NodeName: e.NodeName, Runtime: e.Runtime, EventType: e.EventType, Signal: e.Signal,
				Mitre: e.Mitre, Severity: e.Severity, Confidence: e.Confidence, Syscall: e.Syscall,
				TargetPath: e.TargetPath, Capability: e.Capability, CreatedAt: e.CreatedAt,
			}
		}
		c.JSON(http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "events": dtos, "total": len(dtos)})
	}
}

// GetRuntimeSignalsByPodScoped returns signals for exactly one canonical Pod identity.
func GetRuntimeSignalsByPodScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, podUID, ok := resolvedRuntimePodIdentity(c)
		if !ok {
			return
		}
		query := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID)
		if raw := c.Query("sinceMinutes"); raw != "" {
			if minutes, err := strconv.Atoi(raw); err == nil && minutes > 0 && minutes <= 43200 {
				query = query.Where("created_at >= ?", time.Now().Add(-time.Duration(minutes)*time.Minute))
			}
		}
		if signalType := c.Query("signalType"); signalType != "" {
			query = query.Where("signal_type = ?", signalType)
		}
		if category := c.Query("category"); category != "" {
			query = query.Where("category = ?", category)
		}
		var signals []models.RuntimeSignal
		if err := query.Order("created_at DESC").Limit(runtimeReadLimit(c, 100)).Find(&signals).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "signals": signals, "count": len(signals)})
	}
}

// GetPodRuntimeBehaviorFactsScoped returns behavior facts for one canonical Pod identity.
func GetPodRuntimeBehaviorFactsScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, podUID, ok := resolvedRuntimePodIdentity(c)
		if !ok {
			return
		}
		if !db.Migrator().HasTable(&models.RuntimeBehaviorFact{}) {
			c.JSON(http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "facts": []models.RuntimeBehaviorFact{}, "total": 0})
			return
		}
		var facts []models.RuntimeBehaviorFact
		if err := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).
			Order("observed_at DESC").Limit(runtimeReadLimit(c, 100)).Find(&facts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "facts": facts, "total": len(facts)})
	}
}

// GetPodRuntimeIncidentsScoped returns incidents for one canonical Pod identity.
func GetPodRuntimeIncidentsScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, podUID, ok := resolvedRuntimePodIdentity(c)
		if !ok {
			return
		}
		if !db.Migrator().HasTable(&models.RuntimeIncident{}) {
			c.JSON(http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "incidents": []models.RuntimeIncident{}, "total": 0})
			return
		}
		var incidents []models.RuntimeIncident
		if err := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).
			Order("last_seen_at DESC").Limit(runtimeReadLimit(c, 100)).Find(&incidents).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"clusterId": clusterID, "podUid": podUID, "incidents": incidents, "total": len(incidents)})
	}
}
