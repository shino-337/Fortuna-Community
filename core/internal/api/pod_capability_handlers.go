package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

type PodCapabilityDTO struct {
	PodUID          string                 `json:"podUid"`
	PodName         string                 `json:"podName,omitempty"`
	Namespace       string                 `json:"namespace"`
	CapabilityID    string                 `json:"capabilityId"`
	Group           string                 `json:"group"`
	Severity        string                 `json:"severity"`
	Evidence        map[string]interface{} `json:"evidence"`
	MitreTechniques []string               `json:"mitreTechniques"`
	CreatedAt       string                 `json:"createdAt"`
	UpdatedAt       string                 `json:"updatedAt"`
	LastSeenAt      string                 `json:"lastSeenAt,omitempty"` // RFC3339; for PCE drill-down "Last Seen" column
}

// hasPodCapabilitiesTable returns true if pod_capabilities table exists (migration 041 has run).
func hasPodCapabilitiesTable(db *gorm.DB) bool {
	return db.Migrator().HasTable("pod_capabilities")
}

func getPodCapabilitiesByUID(c *gin.Context, db *gorm.DB, podUID string) {
	if !hasPodCapabilitiesTable(db) {
		c.JSON(http.StatusOK, gin.H{"podUid": podUID, "capabilities": []PodCapabilityDTO{}, "total": 0})
		return
	}
	q := db.Where("pod_uid = ?", podUID)
	if class := c.Query("class"); class != "" {
		q = q.Where("capability_class = ?", class)
	}
	var caps []models.PodCapability
	if err := q.Order("created_at DESC").Find(&caps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dtos := mapPodCapabilities(caps)
	c.JSON(http.StatusOK, gin.H{
		"podUid":       podUID,
		"capabilities": dtos,
		"total":        len(dtos),
	})
}

// GetPodCapabilities returns capabilities for a specific pod (by UID). Used by /pods/:podUid/capabilities.
func GetPodCapabilities(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		getPodCapabilitiesByUID(c, db, podUID)
	}
}

// GetPodCapabilitiesList returns paginated pod capabilities with filters.
func GetPodCapabilitiesList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok {
			return
		}
		db := db.WithContext(c.Request.Context())
		if !hasPodCapabilitiesTable(db) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Capability inventory is unavailable; migration required"})
			return
		}
		limit := 50
		if l := c.Query("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
				limit = parsed
			}
		}
		offset := 0
		if o := c.Query("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		query := db.Model(&models.PodCapability{}).
			Select("pod_capabilities.*, p.name as pod_name").
			Joins("JOIN pods p ON p.uid = pod_capabilities.pod_uid AND p.deleted_at IS NULL")
		query = scopedInventoryQuery(query, scope, "p.cluster_id")

		if podUID := c.Query("podUid"); podUID != "" {
			query = query.Where("pod_capabilities.pod_uid = ?", podUID)
		}
		if podName := c.Query("podName"); podName != "" {
			query = query.Where("p.name LIKE ?", "%"+podName+"%")
		}
		if ns := c.Query("namespace"); ns != "" {
			query = query.Where("pod_capabilities.namespace = ?", ns)
		}
		if clusterID := c.Query("clusterId"); clusterID != "" {
			query = query.Where("p.cluster_id = ?", clusterID)
		}
		if capID := c.Query("capabilityId"); capID != "" {
			query = query.Where("pod_capabilities.capability_id = ?", capID)
		}
		if sev := c.Query("severity"); sev != "" {
			query = query.Where("pod_capabilities.severity = ?", sev)
		}
		if class := c.Query("class"); class != "" {
			query = query.Where("pod_capabilities.capability_class = ?", class)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(500, gin.H{"error": "Unable to count capabilities"})
			return
		}

		// Use Scan() instead of Find() to properly map custom SELECT fields
		type ScanResult struct {
			models.PodCapability
			PodName string `gorm:"column:pod_name"`
		}
		var caps []ScanResult
		if err := query.Order("pod_capabilities.created_at DESC").Offset(offset).Limit(limit).Scan(&caps).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load capabilities"})
			return
		}

		// Convert to DTOs
		dtos := make([]PodCapabilityDTO, 0, len(caps))
		for _, cap := range caps {
			var evidence map[string]interface{}
			if cap.Evidence != "" {
				_ = json.Unmarshal([]byte(cap.Evidence), &evidence)
			}
			lastSeenAt := ""
			if cap.LastSeenAt != nil {
				lastSeenAt = cap.LastSeenAt.Format(time.RFC3339)
			}
			dtos = append(dtos, PodCapabilityDTO{
				PodUID:          cap.PodUID,
				PodName:         cap.PodName,
				Namespace:       cap.Namespace,
				CapabilityID:    cap.CapabilityID,
				Group:           cap.CapabilityGroup,
				Severity:        cap.Severity,
				Evidence:        evidence,
				MitreTechniques: []string(cap.MitreTechniques),
				CreatedAt:       cap.CreatedAt.Format(time.RFC3339),
				UpdatedAt:       cap.UpdatedAt.Format(time.RFC3339),
				LastSeenAt:      lastSeenAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"capabilities": dtos,
			"total":        total,
			"limit":        limit,
			"offset":       offset,
		})
	}
}

// GetPodCapabilitiesSummary returns aggregated counts by cluster/namespace/capability.
func GetPodCapabilitiesSummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok {
			return
		}
		db := db.WithContext(c.Request.Context())
		type summaryRow struct {
			ClusterID    string `json:"clusterId"`
			Namespace    string `json:"namespace"`
			CapabilityID string `json:"capabilityId"`
			Severity     string `json:"severity"`
			Count        int    `json:"count"`
		}

		if !hasPodCapabilitiesTable(db) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Capability inventory is unavailable; migration required"})
			return
		}
		clusterID := c.Query("clusterId")
		namespace := c.Query("namespace")
		capabilityID := c.Query("capabilityId")
		severity := c.Query("severity")

		query := db.Table("pod_capabilities AS pc").
			Select("p.cluster_id AS cluster_id, pc.namespace AS namespace, pc.capability_id AS capability_id, pc.severity AS severity, COUNT(*) AS count").
			Joins("JOIN pods p ON p.uid = pc.pod_uid AND p.deleted_at IS NULL")

		query = scopedInventoryQuery(query, scope, "p.cluster_id")

		if clusterID != "" {
			query = query.Where("p.cluster_id = ?", clusterID)
		}
		if namespace != "" {
			query = query.Where("pc.namespace = ?", namespace)
		}
		if capabilityID != "" {
			query = query.Where("pc.capability_id = ?", capabilityID)
		}
		if severity != "" {
			query = query.Where("pc.severity = ?", severity)
		}

		rows := []summaryRow{}
		if err := query.Group("p.cluster_id, pc.namespace, pc.capability_id, pc.severity").
			Order("p.cluster_id, pc.namespace, pc.capability_id, pc.severity").
			Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load capabilities"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"summary": rows,
			"total":   len(rows),
		})
	}
}

// GetPodCapabilitiesSummaryByCluster returns aggregated counts by cluster only.
func GetPodCapabilitiesSummaryByCluster(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok {
			return
		}
		db := db.WithContext(c.Request.Context())
		if !hasPodCapabilitiesTable(db) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Capability inventory is unavailable; migration required"})
			return
		}
		type row struct {
			ClusterID string `json:"clusterId"`
			Count     int    `json:"count"`
		}

		query := db.Table("pod_capabilities AS pc").
			Select("p.cluster_id AS cluster_id, COUNT(*) AS count").
			Joins("JOIN pods p ON p.uid = pc.pod_uid AND p.deleted_at IS NULL")

		query = scopedInventoryQuery(query, scope, "p.cluster_id")

		if clusterID := c.Query("clusterId"); clusterID != "" {
			query = query.Where("p.cluster_id = ?", clusterID)
		}

		rows := []row{}
		if err := query.Group("p.cluster_id").Order("p.cluster_id").Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load capabilities"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"summary": rows,
			"total":   len(rows),
		})
	}
}

// GetPodCapabilitiesSummaryByCapability returns aggregated counts by capability only (active pods only).
func GetPodCapabilitiesSummaryByCapability(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok {
			return
		}
		db := db.WithContext(c.Request.Context())
		if !hasPodCapabilitiesTable(db) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Capability inventory is unavailable; migration required"})
			return
		}
		type row struct {
			CapabilityID string `json:"capabilityId"`
			Severity     string `json:"severity"`
			Count        int    `json:"count"`
		}

		query := db.Table("pod_capabilities AS pc").
			Select("pc.capability_id AS capability_id, pc.severity AS severity, COUNT(*) AS count").
			Joins("JOIN pods p ON p.uid = pc.pod_uid AND p.deleted_at IS NULL")

		query = scopedInventoryQuery(query, scope, "p.cluster_id")

		if capabilityID := c.Query("capabilityId"); capabilityID != "" {
			query = query.Where("pc.capability_id = ?", capabilityID)
		}
		if severity := c.Query("severity"); severity != "" {
			query = query.Where("pc.severity = ?", severity)
		}
		if clusterID := c.Query("clusterId"); clusterID != "" {
			query = query.Where("p.cluster_id = ?", clusterID)
		}

		rows := []row{}
		if err := query.Group("pc.capability_id, pc.severity").
			Order("pc.capability_id, pc.severity").Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load capabilities"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"summary": rows,
			"total":   len(rows),
		})
	}
}

// GetPodCapabilitiesSummaryByNamespace returns aggregated counts by namespace and severity (active pods only).
func GetPodCapabilitiesSummaryByNamespace(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok {
			return
		}
		db := db.WithContext(c.Request.Context())
		if !hasPodCapabilitiesTable(db) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Capability inventory is unavailable; migration required"})
			return
		}
		type row struct {
			Namespace string `json:"namespace"`
			Severity  string `json:"severity"`
			Count     int    `json:"count"`
		}

		query := db.Table("pod_capabilities AS pc").
			Select("pc.namespace AS namespace, pc.severity AS severity, COUNT(*) AS count").
			Joins("JOIN pods p ON p.uid = pc.pod_uid AND p.deleted_at IS NULL")

		query = scopedInventoryQuery(query, scope, "p.cluster_id")

		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("pc.namespace = ?", namespace)
		}
		if severity := c.Query("severity"); severity != "" {
			query = query.Where("pc.severity = ?", severity)
		}
		if capabilityID := c.Query("capabilityId"); capabilityID != "" {
			query = query.Where("pc.capability_id = ?", capabilityID)
		}
		if clusterID := c.Query("clusterId"); clusterID != "" {
			query = query.Where("p.cluster_id = ?", clusterID)
		}

		rows := []row{}
		if err := query.Group("pc.namespace, pc.severity").
			Order("pc.namespace, pc.severity").Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load capabilities"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"summary": rows,
			"total":   len(rows),
		})
	}
}

// GetPodCapabilitiesSummaryBySeverity returns aggregated counts by severity only (active pods only).
func GetPodCapabilitiesSummaryBySeverity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok {
			return
		}
		db := db.WithContext(c.Request.Context())
		if !hasPodCapabilitiesTable(db) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Capability inventory is unavailable; migration required"})
			return
		}
		type row struct {
			Severity string `json:"severity"`
			Count    int    `json:"count"`
		}

		query := db.Table("pod_capabilities AS pc").
			Select("pc.severity AS severity, COUNT(*) AS count").
			Joins("JOIN pods p ON p.uid = pc.pod_uid AND p.deleted_at IS NULL")

		query = scopedInventoryQuery(query, scope, "p.cluster_id")

		if severity := c.Query("severity"); severity != "" {
			query = query.Where("pc.severity = ?", severity)
		}
		if capabilityID := c.Query("capabilityId"); capabilityID != "" {
			query = query.Where("pc.capability_id = ?", capabilityID)
		}
		if clusterID := c.Query("clusterId"); clusterID != "" {
			query = query.Where("p.cluster_id = ?", clusterID)
		}

		rows := []row{}
		if err := query.Group("pc.severity").Order("pc.severity").Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load capabilities"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"summary": rows,
			"total":   len(rows),
		})
	}
}

// GetPodCapabilitiesTrend returns time-series counts by severity.
// Always returns one point per day for the last `days` (fill missing days with zeros) so dashboard chart renders.
func GetPodCapabilitiesTrend(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok {
			return
		}
		db := db.WithContext(c.Request.Context())
		if !hasPodCapabilitiesTable(db) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Capability inventory is unavailable; migration required"})
			return
		}
		type row struct {
			Date     string `json:"date"`
			Critical int    `json:"critical"`
			High     int    `json:"high"`
			Medium   int    `json:"medium"`
			Low      int    `json:"low"`
		}

		days := 7
		if d := c.Query("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 90 {
				days = parsed
			}
		}

		today := time.Now().UTC().Truncate(24 * time.Hour)
		start := today.AddDate(0, 0, 1-days)
		query := db.Table("pod_capabilities AS pc").Select("pc.created_at, pc.severity").
			Joins("JOIN pods p ON p.uid = pc.pod_uid AND p.deleted_at IS NULL").
			Where("pc.created_at >= ? AND pc.created_at < ?", start, today.AddDate(0, 0, 1))

		query = scopedInventoryQuery(query, scope, "p.cluster_id")

		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("pc.namespace = ?", namespace)
		}
		if capabilityID := c.Query("capabilityId"); capabilityID != "" {
			query = query.Where("pc.capability_id = ?", capabilityID)
		}
		if podUID := c.Query("podUid"); podUID != "" {
			query = query.Where("pc.pod_uid = ?", podUID)
		}
		if clusterID := c.Query("clusterId"); clusterID != "" {
			query = query.Where("p.cluster_id = ?", clusterID)
		}

		var observations []struct {
			CreatedAt time.Time
			Severity  string
		}
		if err := query.Scan(&observations).Error; err != nil {
			c.JSON(500, gin.H{"error": "Unable to load capability trends"})
			return
		}
		result := make([]row, days)
		for i := range result {
			result[i].Date = start.AddDate(0, 0, i).Format("2006-01-02")
		}
		for _, observation := range observations {
			index := int(observation.CreatedAt.UTC().Sub(start) / (24 * time.Hour))
			if index < 0 || index >= days {
				continue
			}
			switch strings.ToLower(observation.Severity) {
			case "critical":
				result[index].Critical++
			case "high":
				result[index].High++
			case "medium":
				result[index].Medium++
			case "low":
				result[index].Low++
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"points": result,
			"total":  len(result),
		})
	}
}

func mapPodCapabilities(caps []models.PodCapability) []PodCapabilityDTO {
	dtos := make([]PodCapabilityDTO, 0, len(caps))
	for _, cap := range caps {
		var evidence map[string]interface{}
		if cap.Evidence != "" {
			_ = json.Unmarshal([]byte(cap.Evidence), &evidence)
		}
		lastSeenAt := ""
		if cap.LastSeenAt != nil {
			lastSeenAt = cap.LastSeenAt.Format(time.RFC3339)
		}
		dtos = append(dtos, PodCapabilityDTO{
			PodUID:          cap.PodUID,
			Namespace:       cap.Namespace,
			CapabilityID:    cap.CapabilityID,
			Group:           cap.CapabilityGroup,
			Severity:        cap.Severity,
			Evidence:        evidence,
			MitreTechniques: []string(cap.MitreTechniques),
			CreatedAt:       cap.CreatedAt.Format(time.RFC3339),
			UpdatedAt:       cap.UpdatedAt.Format(time.RFC3339),
			LastSeenAt:      lastSeenAt,
		})
	}
	return dtos
}
