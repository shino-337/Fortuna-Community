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
	CveCount        int                `json:"cveCount"`        // len(Vulnerabilities)
	MaxSeverity     string             `json:"maxSeverity"`     // highest severity in list
	MaxCVSS         float32            `json:"maxCvss"`         // highest CVSS in list
	FixVersion      string             `json:"fixVersion"`      // first fixed version if any
	Status          string             `json:"status,omitempty"` // active | allowed | fixed (default active)
}

// VulnerabilityDTO is the payload for each CVE
type VulnerabilityDTO struct {
	ID              string  `json:"id"`
	Severity        string  `json:"severity"`
	CVSSScore       float32 `json:"cvssScore"`
	Description     string  `json:"description,omitempty"`
	FixedVersion    string  `json:"fixedVersion,omitempty"`
	Status          string  `json:"status,omitempty"`          // active | allowed | fixed
	ExploitKnown    bool    `json:"exploitKnown,omitempty"`   // public exploit available
	ExploitMaturity string  `json:"exploitMaturity,omitempty"` // poc | functional | high
	Allowed         bool    `json:"allowed,omitempty"`        // allowed by policy
	Source          string  `json:"source,omitempty"`         // nvd | fortuna-core-cve-matcher (OSV)
	Confidence      string  `json:"confidence,omitempty"`     // P1-3: high (OSV) | low (nvd-fallback)
}

// SBOMDetailDTO is returned by GET /sbom/{podId}
type SBOMDetailDTO struct {
	PodID                  string               `json:"podId"`
	Image                  string               `json:"image"`
	Namespace              string               `json:"namespace"`
	PodName                string               `json:"podName"`
	Container              string               `json:"container"`
	GeneratedAt            time.Time            `json:"generatedAt"`
	PackageCount           int                  `json:"packageCount"`
	VulnerablePackageCount int                  `json:"vulnerablePackageCount"` // packages with ≥1 CVE
	VulnerabilitySummary   vulnerabilitySummary `json:"vulnerabilitySummary"`   // critical/high/medium/low counts
	Components             []SBOMComponentDTO   `json:"components"`
	// Finding #8.4: distroless/heuristic SBOM – for dashboard badge and audit
	SbomSource  string `json:"sbomSource,omitempty"`  // parsers | distroless-heuristic | label-metadata
	Confidence  string `json:"confidence,omitempty"`  // low | medium | high
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

// GetSBOMDetail returns the component + CVE detail for a specific pod by pod UID (K8s uid).
// Contract: :podUid must be the pod UID string; dashboard and list API use this consistently.
func GetSBOMDetail(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}

		var sbom models.SBOM
		if err := db.Where("pod_uid = ? AND deleted_at IS NULL", podUID).Order("created_at DESC").Limit(1).Find(&sbom).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if sbom.ID == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error":  "sbom not found for pod",
				"podUid": podUID,
				"hint":   "Ensure Agent has sent SBOM for this pod (pod watcher → SBOM queue → Core gRPC SendSBOMFinding).",
				"checks": []string{
					"Pod must run on a node where Fortuna Agent is running (DaemonSet).",
					"Agent logs: look for [SBOMProcessor] or [SBOMQueue] for this pod; check for 'Queue full' or 'SBOM extraction failed' or 'SendSBOMFinding RPC failed'.",
					"Core logs: look for [SBOM] Received SBOM / Created new SBOM for this pod_uid.",
					"See docs/03-components/sbom/SBOM-Not-Loading-Checklist.md for full checklist.",
				},
			})
			return
		}

		var components []models.SBOMComponent
		if err := db.Where("sbom_id = ? AND deleted_at IS NULL", sbom.ID).Find(&components).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var matches []models.CVEMatch
		if err := db.Where("sbom_id = ? AND deleted_at IS NULL", sbom.ID).Preload("CVE").Order("severity DESC").Find(&matches).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		byName := make(map[string][]models.CVEMatch)
		summary := vulnerabilitySummary{"critical": 0, "high": 0, "medium": 0, "low": 0}
		for _, match := range matches {
			byName[match.PackageName] = append(byName[match.PackageName], match)
			sev := strings.ToLower(match.Severity)
			if _, ok := summary[sev]; ok {
				summary[sev]++
			}
		}
		vulnerablePackageCount := len(byName)

		dto := SBOMDetailDTO{
			PodID:                   sbom.PodUID,
			Image:                   fmt.Sprintf("%s:%s", sbom.ImageName, sbom.ImageTag),
			Namespace:               sbom.Namespace,
			PodName:                 sbom.PodName,
			Container:               sbom.ContainerName,
			GeneratedAt:             sbom.GeneratedAt,
			PackageCount:            sbom.PackageCount,
			VulnerablePackageCount:  vulnerablePackageCount,
			VulnerabilitySummary:    summary,
			Components:              make([]SBOMComponentDTO, 0, len(components)),
			SbomSource:              sbom.SbomSource,
			Confidence:              sbom.Confidence,
		}

		severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
		for _, comp := range components {
			compMatches := byName[comp.ComponentName]
			compDTO := SBOMComponentDTO{
				ID:              comp.ID,
				Name:            comp.ComponentName,
				Version:         comp.ComponentVersion,
				Type:            comp.ComponentType,
				PURL:            comp.PURL,
				Vulnerabilities: make([]VulnerabilityDTO, 0, len(compMatches)),
				CveCount:        len(compMatches),
				Status:          "active",
			}
			var maxSev string
			var maxCVSS float32
			var fixVer string
			for _, match := range compMatches {
				sev := strings.ToLower(match.Severity)
				if maxSev == "" || severityOrder[sev] < severityOrder[maxSev] {
					maxSev = sev
				}
				if match.CVSS > maxCVSS {
					maxCVSS = match.CVSS
				}
				if match.FixedVersion != "" && fixVer == "" {
					fixVer = match.FixedVersion
				}
				desc := ""
				exploitKnown := false
				exploitMaturity := ""
				if match.CVE.ID != 0 {
					desc = match.CVE.Description
					exploitKnown = match.CVE.ExploitAvailable
					exploitMaturity = match.CVE.ExploitMaturity
				}
				source := match.MatchedBy
				if source == "nvd-fallback" {
					source = "nvd"
				}
				confidence := "high" // OSV/package_vulnerabilities
				if match.MatchedBy == "nvd-fallback" {
					confidence = "low"
				}
				compDTO.Vulnerabilities = append(compDTO.Vulnerabilities, VulnerabilityDTO{
					ID:              match.CVEID,
					Severity:        sev,
					CVSSScore:       match.CVSS,
					Description:     desc,
					FixedVersion:    match.FixedVersion,
					Status:          "active",
					ExploitKnown:    exploitKnown,
					ExploitMaturity: exploitMaturity,
					Allowed:         false,
					Source:          source,
					Confidence:      confidence,
				})
			}
			compDTO.MaxSeverity = maxSev
			compDTO.MaxCVSS = maxCVSS
			compDTO.FixVersion = fixVer
			dto.Components = append(dto.Components, compDTO)
		}

		c.JSON(http.StatusOK, dto)
	}
}
