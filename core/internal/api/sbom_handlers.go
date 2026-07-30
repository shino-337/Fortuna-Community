package api

import (
	"fmt"
	"net/http"
	"os"
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
	ImageDigest          string               `json:"imageDigest,omitempty"`
	ImageTrust           ImageTrustDTO        `json:"imageTrust"`
	ContainerName        string               `json:"containerName"`
	LastScan             time.Time            `json:"lastScan"`
	PackageCount         int                  `json:"packageCount"`
	VulnerabilitySummary vulnerabilitySummary `json:"vulnerabilitySummary"`
	PodCreatedAt         *time.Time           `json:"podCreatedAt,omitempty"` // from pods table when available
	PodStatus            string               `json:"podStatus,omitempty"`    // K8s phase from pods.phase when synced
	ActivePod            bool                 `json:"activePod"`
	LifecycleState       string               `json:"lifecycleState"` // current | stale
	// SBOM metadata (same as detail; list view for dashboard badges/filters)
	SbomSource string `json:"sbomSource,omitempty"` // parsers | distroless-heuristic | label-metadata
	Confidence string `json:"confidence,omitempty"` // low | medium | high
	GoVersion  string `json:"goVersion,omitempty"`  // Go toolchain / stdlib matcher (when applicable)
}

type ImageTrustDTO struct {
	Registry        string   `json:"registry"`
	RegistryClass   string   `json:"registryClass"` // trusted | blocked | public | internal | unknown
	DigestAvailable bool     `json:"digestAvailable"`
	MutableTag      bool     `json:"mutableTag"`
	Status          string   `json:"status"` // strong | weak | unknown
	Evidence        []string `json:"evidence,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

// MalwareMatchDTO is embedded on SBOM components when package@version hit malware_packages / malware_matches.
type MalwareMatchDTO struct {
	Reason        string  `json:"reason"`
	Confidence    float32 `json:"confidence"`
	MalwareFamily string  `json:"malwareFamily,omitempty"`
}

// SBOMComponentDTO exposes component details for /sbom/{podId}
type SBOMComponentDTO struct {
	ID              uint               `json:"id"`
	Name            string             `json:"name"`
	Version         string             `json:"version"`
	Type            string             `json:"type"`
	PURL            string             `json:"purl,omitempty"`
	Vulnerabilities []VulnerabilityDTO `json:"vulnerabilities"`
	CveCount        int                `json:"cveCount"`         // len(Vulnerabilities)
	MaxSeverity     string             `json:"maxSeverity"`      // highest severity in list
	MaxCVSS         float32            `json:"maxCvss"`          // highest CVSS in list
	FixVersion      string             `json:"fixVersion"`       // first fixed version if any
	Status          string             `json:"status,omitempty"` // active | allowed | fixed (default active)
	MalwareMatch    *MalwareMatchDTO   `json:"malwareMatch,omitempty"`
}

// VulnerabilityDTO is the payload for each CVE
type VulnerabilityDTO struct {
	ID              string  `json:"id"`
	Severity        string  `json:"severity"`
	CVSSScore       float32 `json:"cvssScore"`
	Description     string  `json:"description,omitempty"`
	FixedVersion    string  `json:"fixedVersion,omitempty"`
	Status          string  `json:"status,omitempty"`          // active | allowed | fixed
	ExploitKnown    bool    `json:"exploitKnown,omitempty"`    // public exploit available
	ExploitMaturity string  `json:"exploitMaturity,omitempty"` // poc | functional | high
	Allowed         bool    `json:"allowed,omitempty"`         // allowed by policy
	Source          string  `json:"source,omitempty"`          // fortuna-core-cve-matcher (and variants)
	Confidence      string  `json:"confidence,omitempty"`      // high (constrained OSV) | lower when matcher flags uncertainty
}

// SBOMDetailDTO is returned by GET /sbom/{podId}
type SBOMDetailDTO struct {
	PodID                  string               `json:"podId"`
	Image                  string               `json:"image"`
	ImageDigest            string               `json:"imageDigest,omitempty"`
	ImageTrust             ImageTrustDTO        `json:"imageTrust"`
	Namespace              string               `json:"namespace"`
	PodName                string               `json:"podName"`
	Container              string               `json:"container"`
	GeneratedAt            time.Time            `json:"generatedAt"`
	PackageCount           int                  `json:"packageCount"`
	VulnerablePackageCount int                  `json:"vulnerablePackageCount"` // packages with ≥1 CVE
	VulnerabilitySummary   vulnerabilitySummary `json:"vulnerabilitySummary"`   // critical/high/medium/low counts
	Components             []SBOMComponentDTO   `json:"components"`
	ActivePod              bool                 `json:"activePod"`
	LifecycleState         string               `json:"lifecycleState"` // current | stale
	// Finding #8.4: distroless/heuristic SBOM – for dashboard badge and audit
	SbomSource string `json:"sbomSource,omitempty"` // parsers | distroless-heuristic | label-metadata
	Confidence string `json:"confidence,omitempty"` // low | medium | high
	GoVersion  string `json:"goVersion,omitempty"`  // buildinfo / image config — Go stdlib CVE matching
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
		// UI parity: page (1-based) + pageSize when limit/offset not used (e.g. ?page=1&pageSize=20).
		if c.Query("limit") == "" && c.Query("offset") == "" {
			if ps := c.Query("pageSize"); ps != "" {
				if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 200 {
					limit = parsed
				}
			}
			if p := c.Query("page"); p != "" {
				if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
					offset = (parsed - 1) * limit
				}
			}
		}
		podNameFilter := strings.TrimSpace(c.Query("podName"))
		namespaceFilter := strings.TrimSpace(c.Query("namespace"))
		includeStale := strings.EqualFold(strings.TrimSpace(c.Query("includeStale")), "true") ||
			strings.EqualFold(strings.TrimSpace(c.Query("scope")), "all") ||
			strings.EqualFold(strings.TrimSpace(c.Query("scope")), "historical")

		baseWhere := "s.deleted_at IS NULL"
		args := []interface{}{}
		if podNameFilter != "" {
			baseWhere += " AND s.pod_name ILIKE ?"
			args = append(args, "%"+podNameFilter+"%")
		}
		if namespaceFilter != "" {
			baseWhere += " AND s.namespace ILIKE ?"
			args = append(args, "%"+namespaceFilter+"%")
		}

		var total int64
		countQuery := db.Model(&models.SBOM{}).Where("sboms.deleted_at IS NULL")
		if !includeStale {
			countQuery = countQuery.Joins("INNER JOIN pods ON pods.uid = sboms.pod_uid AND pods.deleted_at IS NULL")
		}
		if podNameFilter != "" {
			countQuery = countQuery.Where("sboms.pod_name ILIKE ?", "%"+podNameFilter+"%")
		}
		if namespaceFilter != "" {
			countQuery = countQuery.Where("sboms.namespace ILIKE ?", "%"+namespaceFilter+"%")
		}
		if err := countQuery.Distinct("pod_uid").Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		joinActivePods := ""
		if !includeStale {
			joinActivePods = "INNER JOIN pods p ON p.uid = s.pod_uid AND p.deleted_at IS NULL"
		}
		query := `
			SELECT * FROM (
				SELECT DISTINCT ON (s.pod_uid) s.*
				FROM sboms s
				` + joinActivePods + `
				WHERE ` + baseWhere + `
				ORDER BY s.pod_uid, s.created_at DESC
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
		uidToPhase := make(map[string]string)
		if len(podUIDs) > 0 {
			var podRows []struct {
				UID       string
				CreatedAt time.Time
				Phase     string
			}
			if err := db.Table("pods").Select("uid, created_at, phase").Where("uid IN ? AND deleted_at IS NULL", podUIDs).Find(&podRows).Error; err == nil {
				for _, r := range podRows {
					uidToCreatedAt[r.UID] = r.CreatedAt
					if strings.TrimSpace(r.Phase) != "" {
						uidToPhase[r.UID] = strings.TrimSpace(r.Phase)
					}
				}
			}
		}

		summaries := make([]SBOMSummaryDTO, 0, len(sboms))
		sbomIDs := make([]uint, 0, len(sboms))
		for _, s := range sboms {
			sbomIDs = append(sbomIDs, s.ID)
		}
		type sevAgg struct {
			SBOMID   uint
			Severity string
			Count    int
		}
		sevBySBOM := make(map[uint]map[string]int, len(sboms))
		if len(sbomIDs) > 0 {
			var rows []sevAgg
			if err := db.Model(&models.CVEMatch{}).
				Select("sbom_id, LOWER(severity) as severity, COUNT(*) as count").
				Where("sbom_id IN ? AND deleted_at IS NULL", sbomIDs).
				Group("sbom_id, LOWER(severity)").
				Scan(&rows).Error; err == nil {
				for _, row := range rows {
					if _, ok := sevBySBOM[row.SBOMID]; !ok {
						sevBySBOM[row.SBOMID] = map[string]int{}
					}
					sevBySBOM[row.SBOMID][strings.ToLower(strings.TrimSpace(row.Severity))] = row.Count
				}
			}
		}
		for _, sbom := range sboms {
			var podCreatedAt *time.Time
			_, activePod := uidToCreatedAt[sbom.PodUID]
			if t, ok := uidToCreatedAt[sbom.PodUID]; ok {
				podCreatedAt = &t
			}
			podPhase := uidToPhase[sbom.PodUID]
			summary := SBOMSummaryDTO{
				PodID:          sbom.PodUID,
				PodName:        sbom.PodName,
				Namespace:      sbom.Namespace,
				Image:          fmt.Sprintf("%s:%s", sbom.ImageName, sbom.ImageTag),
				ImageDigest:    strings.TrimSpace(sbom.ImageDigest),
				ImageTrust:     buildImageTrust(sbom.ImageName, sbom.ImageTag, sbom.ImageDigest),
				ContainerName:  sbom.ContainerName,
				LastScan:       sbom.GeneratedAt,
				PackageCount:   sbom.PackageCount,
				PodCreatedAt:   podCreatedAt,
				PodStatus:      podPhase,
				ActivePod:      activePod,
				LifecycleState: sbomLifecycleState(activePod),
				SbomSource:     sbom.SbomSource,
				Confidence:     sbom.Confidence,
				GoVersion:      sbom.GoVersion,
				VulnerabilitySummary: vulnerabilitySummary{
					"critical": 0,
					"high":     0,
					"medium":   0,
					"low":      0,
				},
			}

			if grouped, ok := sevBySBOM[sbom.ID]; ok {
				for sev, count := range grouped {
					summary.VulnerabilitySummary[sev] = count
				}
			}

			summaries = append(summaries, summary)
		}

		c.JSON(http.StatusOK, gin.H{
			"sboms":    summaries,
			"total":    total,
			"limit":    limit,
			"offset":   offset,
			"page":     offset/limit + 1,
			"pageSize": limit,
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
		includeStale := strings.EqualFold(strings.TrimSpace(c.Query("includeStale")), "true") ||
			strings.EqualFold(strings.TrimSpace(c.Query("scope")), "all") ||
			strings.EqualFold(strings.TrimSpace(c.Query("scope")), "historical")

		var sbom models.SBOM
		sbomQuery := db.Where("pod_uid = ? AND deleted_at IS NULL", podUID)
		if !includeStale {
			sbomQuery = sbomQuery.Where("EXISTS (SELECT 1 FROM pods p WHERE p.uid = sboms.pod_uid AND p.deleted_at IS NULL)")
		}
		if err := sbomQuery.Order("created_at DESC").Limit(1).Find(&sbom).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if sbom.ID == 0 {
			if !includeStale {
				var staleCount int64
				db.Model(&models.SBOM{}).Where("pod_uid = ? AND deleted_at IS NULL", podUID).Count(&staleCount)
				if staleCount > 0 {
					c.JSON(http.StatusNotFound, gin.H{
						"error":        "active sbom not found for pod",
						"podUid":       podUID,
						"state":        "stale",
						"hint":         "This pod UID has historical SBOM data but no active pod row. Use includeStale=true only for historical views.",
						"includeStale": false,
					})
					return
				}
			}
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

		byCompID := make(map[uint]models.MalwareMatch)
		byNameVer := make(map[string]models.MalwareMatch)
		if db.Migrator().HasTable("malware_matches") {
			var mmRows []models.MalwareMatch
			if err := db.Where("sbom_id = ?", sbom.ID).Find(&mmRows).Error; err == nil {
				for _, mm := range mmRows {
					if mm.ComponentID != 0 {
						byCompID[mm.ComponentID] = mm
					}
					k := strings.ToLower(strings.TrimSpace(mm.PackageName)) + ":" + strings.TrimSpace(mm.PackageVersion)
					byNameVer[k] = mm
				}
			}
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

		activePod := sbomHasActivePod(db, sbom.PodUID)
		dto := SBOMDetailDTO{
			PodID:                  sbom.PodUID,
			Image:                  fmt.Sprintf("%s:%s", sbom.ImageName, sbom.ImageTag),
			ImageDigest:            strings.TrimSpace(sbom.ImageDigest),
			ImageTrust:             buildImageTrust(sbom.ImageName, sbom.ImageTag, sbom.ImageDigest),
			Namespace:              sbom.Namespace,
			PodName:                sbom.PodName,
			Container:              sbom.ContainerName,
			GeneratedAt:            sbom.GeneratedAt,
			PackageCount:           sbom.PackageCount,
			VulnerablePackageCount: vulnerablePackageCount,
			VulnerabilitySummary:   summary,
			Components:             make([]SBOMComponentDTO, 0, len(components)),
			ActivePod:              activePod,
			LifecycleState:         sbomLifecycleState(activePod),
			SbomSource:             sbom.SbomSource,
			Confidence:             sbom.Confidence,
			GoVersion:              sbom.GoVersion,
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
				confidence := "high"
				if strings.Contains(strings.ToLower(match.MatchedBy), "low-confidence") {
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
			if mm, ok := byCompID[comp.ID]; ok {
				compDTO.MalwareMatch = &MalwareMatchDTO{
					Reason:        mm.Reason,
					Confidence:    mm.Confidence,
					MalwareFamily: mm.MalwareFamily,
				}
			} else {
				k := strings.ToLower(strings.TrimSpace(comp.ComponentName)) + ":" + strings.TrimSpace(comp.ComponentVersion)
				if mm, ok2 := byNameVer[k]; ok2 {
					compDTO.MalwareMatch = &MalwareMatchDTO{
						Reason:        mm.Reason,
						Confidence:    mm.Confidence,
						MalwareFamily: mm.MalwareFamily,
					}
				}
			}
			dto.Components = append(dto.Components, compDTO)
		}

		c.JSON(http.StatusOK, dto)
	}
}

func sbomHasActivePod(db *gorm.DB, podUID string) bool {
	if db == nil || strings.TrimSpace(podUID) == "" || !db.Migrator().HasTable("pods") {
		return false
	}
	var count int64
	if err := db.Model(&models.Pod{}).Where("uid = ? AND deleted_at IS NULL", podUID).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func sbomLifecycleState(activePod bool) string {
	if activePod {
		return "current"
	}
	return "stale"
}

func buildImageTrust(imageName, imageTag, imageDigest string) ImageTrustDTO {
	registry := imageRegistry(imageName)
	digestAvailable := isImageDigestAvailable(imageDigest)
	mutableTag := isMutableImageTag(imageTag)
	class := registryClass(registry)
	out := ImageTrustDTO{
		Registry:        registry,
		RegistryClass:   class,
		DigestAvailable: digestAvailable,
		MutableTag:      mutableTag,
		Status:          "unknown",
		Evidence:        []string{},
		Warnings:        []string{},
	}
	if digestAvailable {
		out.Evidence = append(out.Evidence, "image digest observed")
	} else {
		out.Warnings = append(out.Warnings, "image digest missing")
	}
	if mutableTag {
		out.Warnings = append(out.Warnings, "mutable or empty image tag")
	}
	if out.RegistryClass == "blocked" {
		out.Warnings = append(out.Warnings, "registry is blocked by policy")
	} else if out.RegistryClass == "unknown" {
		out.Warnings = append(out.Warnings, "registry is not classified")
	} else {
		out.Evidence = append(out.Evidence, "registry classified as "+out.RegistryClass)
	}

	switch {
	case out.RegistryClass == "blocked":
		out.Status = "blocked"
	case digestAvailable && !mutableTag && (out.RegistryClass == "trusted" || out.RegistryClass == "internal"):
		out.Status = "strong"
	case digestAvailable:
		out.Status = "weak"
	default:
		out.Status = "unknown"
	}
	return out
}

func isImageDigestAvailable(digest string) bool {
	digest = strings.ToLower(strings.TrimSpace(digest))
	return strings.HasPrefix(digest, "sha256:") && len(digest) > len("sha256:")
}

func isMutableImageTag(tag string) bool {
	tag = strings.ToLower(strings.TrimSpace(tag))
	return tag == "" || tag == "latest" || tag == "dev" || tag == "snapshot" || strings.HasPrefix(tag, "nightly")
}

func imageRegistry(imageName string) string {
	imageName = strings.TrimSpace(imageName)
	if imageName == "" {
		return "unknown"
	}
	first := strings.Split(imageName, "/")[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return strings.ToLower(first)
	}
	return "docker.io"
}

func registryClass(registry string) string {
	registry = strings.ToLower(strings.TrimSpace(registry))
	switch {
	case registry == "" || registry == "unknown":
		return "unknown"
	case registryMatchesPolicy(registry, os.Getenv("FORTUNA_BLOCKED_REGISTRIES")):
		return "blocked"
	case registryMatchesPolicy(registry, os.Getenv("FORTUNA_TRUSTED_REGISTRIES")):
		return "trusted"
	case registry == "localhost" ||
		strings.HasPrefix(registry, "127.") ||
		strings.HasPrefix(registry, "10.") ||
		strings.HasPrefix(registry, "192.168.") ||
		strings.HasSuffix(registry, ".svc") ||
		strings.HasSuffix(registry, ".cluster.local") ||
		strings.Contains(registry, ".svc."):
		return "internal"
	default:
		return "public"
	}
}

func registryMatchesPolicy(registry, rawPolicy string) bool {
	registry = strings.ToLower(strings.TrimSpace(registry))
	for _, entry := range strings.Split(rawPolicy, ",") {
		pattern := strings.ToLower(strings.TrimSpace(entry))
		if pattern == "" {
			continue
		}
		switch {
		case pattern == registry:
			return true
		case strings.HasPrefix(pattern, "*.") && strings.HasSuffix(registry, strings.TrimPrefix(pattern, "*")):
			return true
		case strings.HasSuffix(pattern, "/*") && strings.HasPrefix(registry, strings.TrimSuffix(pattern, "/*")):
			return true
		}
	}
	return false
}
