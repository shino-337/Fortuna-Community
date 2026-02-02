package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type vulnerabilitySummary map[string]int

// SBOMSummaryDTO contains the summary data returned by /sbom
type SBOMSummaryDTO struct {
	PodID                string               `json:"podId"`
	PodName              string               `json:"podName"`
	Namespace            string               `json:"namespace"`
	Image                string               `json:"image"`
	ContainerName        string               `json:"containerName"`
	LastScan             time.Time            `json:"lastScan"`
	PackageCount         int                  `json:"packageCount"`
	VulnerabilitySummary vulnerabilitySummary `json:"vulnerabilitySummary"`
	PodCreatedAt         *time.Time           `json:"podCreatedAt,omitempty"` // from pods table when available
	PodStatus            string               `json:"podStatus,omitempty"`    // e.g. Running, Pending (when available)
}

// SBOMComponentDTO exposes component details for /sbom/{podId}
type SBOMComponentDTO struct {
	ID              uint               `json:"id"`
	Name            string             `json:"name"`
	Version         string             `json:"version"`
	Type            string             `json:"type"`
	PURL            string             `json:"purl,omitempty"`
	Vulnerabilities []VulnerabilityDTO `json:"vulnerabilities"`
}

// VulnerabilityDTO is the payload for each CVE
type VulnerabilityDTO struct {
	ID           string  `json:"id"`
	Severity     string  `json:"severity"`
	CVSSScore    float32 `json:"cvssScore"`
	Description  string  `json:"description,omitempty"`
	FixedVersion string  `json:"fixedVersion,omitempty"`
}

// SBOMDetailDTO is returned by GET /sbom/{podId}
type SBOMDetailDTO struct {
	PodID        string             `json:"podId"`
	Image        string             `json:"image"`
	Namespace    string             `json:"namespace"`
	PodName      string             `json:"podName"`
	Container    string             `json:"container"`
	GeneratedAt  time.Time          `json:"generatedAt"`
	PackageCount int                `json:"packageCount"`
	Components   []SBOMComponentDTO `json:"components"`
}

// GetSBOMList returns paginated SBOM summaries (pod-level) with vulnerability counts.
func GetSBOMList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		podNameFilter := strings.TrimSpace(c.Query("podName"))
		namespaceFilter := strings.TrimSpace(c.Query("namespace"))

		baseWhere := "deleted_at IS NULL"
		args := []interface{}{}
		if podNameFilter != "" {
			baseWhere += " AND pod_name ILIKE ?"
			args = append(args, "%"+podNameFilter+"%")
		}
		if namespaceFilter != "" {
			baseWhere += " AND namespace ILIKE ?"
			args = append(args, "%"+namespaceFilter+"%")
		}

		var total int64
		countQuery := db.Model(&models.SBOM{}).Where("deleted_at IS NULL")
		if podNameFilter != "" {
			countQuery = countQuery.Where("pod_name ILIKE ?", "%"+podNameFilter+"%")
		}
		if namespaceFilter != "" {
			countQuery = countQuery.Where("namespace ILIKE ?", "%"+namespaceFilter+"%")
		}
		if err := countQuery.Distinct("pod_uid").Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		query := `
			SELECT * FROM (
				SELECT DISTINCT ON (pod_uid) *
				FROM sboms
				WHERE ` + baseWhere + `
				ORDER BY pod_uid, created_at DESC
			) AS latest
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		queryArgs := append(append([]interface{}{}, args...), limit, offset)
		var sboms []models.SBOM
		if err := db.Raw(query, queryArgs...).Scan(&sboms).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Batch lookup pod created_at by uid (pods table has uid, created_at)
		podUIDs := make([]string, 0, len(sboms))
		for _, s := range sboms {
			if s.PodUID != "" {
				podUIDs = append(podUIDs, s.PodUID)
			}
		}
		uidToCreatedAt := make(map[string]time.Time)
		if len(podUIDs) > 0 {
			var podRows []struct {
				UID       string
				CreatedAt time.Time
			}
			if err := db.Table("pods").Select("uid, created_at").Where("uid IN ? AND deleted_at IS NULL", podUIDs).Find(&podRows).Error; err == nil {
				for _, r := range podRows {
					uidToCreatedAt[r.UID] = r.CreatedAt
				}
			}
		}

		summaries := make([]SBOMSummaryDTO, 0, len(sboms))
		for _, sbom := range sboms {
			var podCreatedAt *time.Time
			if t, ok := uidToCreatedAt[sbom.PodUID]; ok {
				podCreatedAt = &t
			}
			summary := SBOMSummaryDTO{
				PodID:         sbom.PodUID,
				PodName:       sbom.PodName,
				Namespace:     sbom.Namespace,
				Image:         fmt.Sprintf("%s:%s", sbom.ImageName, sbom.ImageTag),
				ContainerName: sbom.ContainerName,
				LastScan:      sbom.GeneratedAt,
				PackageCount:  sbom.PackageCount,
				PodCreatedAt:  podCreatedAt,
				PodStatus:     "", // pods table has no status column; can be extended later
				VulnerabilitySummary: vulnerabilitySummary{
					"critical": 0,
					"high":     0,
					"medium":   0,
					"low":      0,
				},
			}

			var rows []struct {
				Severity string
				Count    int
			}
			if err := db.Model(&models.CVEMatch{}).
				Select("LOWER(severity) as severity, COUNT(*) as count").
				Where("sbom_id = ? AND deleted_at IS NULL", sbom.ID).
				Group("LOWER(severity)").
				Scan(&rows).Error; err == nil {
				for _, r := range rows {
					sev := strings.ToLower(r.Severity)
					if _, ok := summary.VulnerabilitySummary[sev]; ok {
						summary.VulnerabilitySummary[sev] = r.Count
					} else {
						summary.VulnerabilitySummary[sev] = r.Count
					}
				}
			}

			summaries = append(summaries, summary)
		}

		c.JSON(http.StatusOK, gin.H{
			"sboms":  summaries,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// GetSBOMDetail returns the component + CVE detail for a specific pod UID.
func GetSBOMDetail(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("podId")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podId is required"})
			return
		}

		var sbom models.SBOM
		if err := db.Where("pod_uid = ? AND deleted_at IS NULL", podUID).Order("created_at DESC").First(&sbom).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "sbom not found for pod"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var components []models.SBOMComponent
		if err := db.Where("sbom_id = ? AND deleted_at IS NULL", sbom.ID).Find(&components).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var matches []models.CVEMatch
		if err := db.Where("sbom_id = ? AND deleted_at IS NULL", sbom.ID).Order("severity DESC").Find(&matches).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		byName := make(map[string][]models.CVEMatch)
		for _, match := range matches {
			byName[match.PackageName] = append(byName[match.PackageName], match)
		}

		dto := SBOMDetailDTO{
			PodID:        sbom.PodUID,
			Image:        fmt.Sprintf("%s:%s", sbom.ImageName, sbom.ImageTag),
			Namespace:    sbom.Namespace,
			PodName:      sbom.PodName,
			Container:    sbom.ContainerName,
			GeneratedAt:  sbom.GeneratedAt,
			PackageCount: sbom.PackageCount,
			Components:   make([]SBOMComponentDTO, 0, len(components)),
		}

		for _, comp := range components {
			compDTO := SBOMComponentDTO{
				ID:              comp.ID,
				Name:            comp.ComponentName,
				Version:         comp.ComponentVersion,
				Type:            comp.ComponentType,
				PURL:            comp.PURL,
				Vulnerabilities: make([]VulnerabilityDTO, 0),
			}

			for _, match := range byName[comp.ComponentName] {
				compDTO.Vulnerabilities = append(compDTO.Vulnerabilities, VulnerabilityDTO{
					ID:           match.CVEID,
					Severity:     strings.ToLower(match.Severity),
					CVSSScore:    match.CVSS,
					Description:  match.CVE.Description,
					FixedVersion: match.FixedVersion,
				})
			}

			dto.Components = append(dto.Components, compDTO)
		}

		c.JSON(http.StatusOK, dto)
	}
}
