//go:build legacy_trivy

package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
	"gorm.io/gorm"
)

// CVEProcessor processes CVE scan results and creates insights
type CVEProcessor struct {
	db             *gorm.DB
	insightManager *riskengine.InsightManager
	severityFilter []string // e.g., ["CRITICAL", "HIGH"]
	logger         *log.Logger
}

// NewCVEProcessor creates a new CVE processor
func NewCVEProcessor(db *gorm.DB, insightManager *riskengine.InsightManager) *CVEProcessor {
	return &CVEProcessor{
		db:             db,
		insightManager: insightManager,
		severityFilter: []string{"CRITICAL", "HIGH"}, // Only process critical and high by default
		logger:         log.New(log.Writer(), "[CVEProcessor] ", log.LstdFlags),
	}
}

// Vulnerability represents a vulnerability from Trivy scan
type Vulnerability struct {
	VulnerabilityID  string   `json:"VulnerabilityID"`
	PkgName          string   `json:"PkgName"`
	PkgPath          string   `json:"PkgPath"`
	InstalledVersion string   `json:"InstalledVersion"`
	FixedVersion     string   `json:"FixedVersion"`
	Severity         string   `json:"Severity"`
	Title            string   `json:"Title"`
	Description      string   `json:"Description"`
	CVSS             map[string]interface{} `json:"CVSS"`
	References       []string `json:"References"`
}

// ProcessScanResult processes scan results and creates insights for critical/high CVEs
func (p *CVEProcessor) ProcessScanResult(
	ctx context.Context,
	podUID string,
	podName string,
	podNamespace string,
	clusterID string,
	containerName string,
	scanResult *models.ImageScanResult,
) error {
	// Parse vulnerabilities from JSON
	var vulns []Vulnerability
	if err := json.Unmarshal([]byte(scanResult.Vulnerabilities), &vulns); err != nil {
		return fmt.Errorf("failed to parse vulnerabilities: %w", err)
	}

	// Filter by severity (CRITICAL + HIGH only)
	filtered := p.filterVulnerabilities(vulns)

	p.logger.Printf("Processing %d vulnerabilities for pod %s/%s (filtered from %d total)",
		len(filtered), podNamespace, podName, len(vulns))

	// Create insight for each CVE
	createdCount := 0
	for _, vuln := range filtered {
		// Enrich with database info
		cveInfo, err := p.enrichCVE(ctx, vuln.VulnerabilityID)
		if err != nil {
			p.logger.Printf("Warning: Failed to enrich CVE %s: %v", vuln.VulnerabilityID, err)
			// Continue anyway with basic info
		}

		// Create insight
		if err := p.createInsight(ctx, podUID, podName, podNamespace, clusterID, containerName, vuln, cveInfo, scanResult); err != nil {
			p.logger.Printf("Failed to create insight for %s: %v", vuln.VulnerabilityID, err)
			continue
		}

		createdCount++
		p.logger.Printf("Created insight for %s in pod %s/%s",
			vuln.VulnerabilityID, podNamespace, podName)
	}

	p.logger.Printf("Created %d insights for pod %s/%s", createdCount, podNamespace, podName)
	return nil
}

// filterVulnerabilities filters vulnerabilities by severity
func (p *CVEProcessor) filterVulnerabilities(vulns []Vulnerability) []Vulnerability {
	filtered := []Vulnerability{}
	for _, vuln := range vulns {
		severity := strings.ToUpper(vuln.Severity)
		for _, filter := range p.severityFilter {
			if severity == filter {
				filtered = append(filtered, vuln)
				break
			}
		}
	}
	return filtered
}

// enrichCVE adds database info to CVE
func (p *CVEProcessor) enrichCVE(ctx context.Context, cveID string) (*models.CVE, error) {
	var cve models.CVE
	if err := p.db.WithContext(ctx).Where("cve_id = ? AND deleted_at IS NULL", cveID).First(&cve).Error; err != nil {
		return nil, err
	}
	return &cve, nil
}

// createInsight creates insight for CVE
func (p *CVEProcessor) createInsight(
	ctx context.Context,
	podUID string,
	podName string,
	podNamespace string,
	clusterID string,
	containerName string,
	vuln Vulnerability,
	cveInfo *models.CVE,
	scanResult *models.ImageScanResult,
) error {
	// Extract CVSS score
	var cvssScore float64
	if vuln.CVSS != nil {
		if nvd, ok := vuln.CVSS["nvd"].(map[string]interface{}); ok {
			if v3, ok := nvd["V3Score"].(float64); ok {
				cvssScore = v3
			} else if v2, ok := nvd["V2Score"].(float64); ok {
				cvssScore = v2
			}
		}
	}
	// Use database CVSS if available
	if cveInfo != nil && cveInfo.CVSSScore > 0 {
		cvssScore = float64(cveInfo.CVSSScore)
	}

	// Build description
	description := fmt.Sprintf(`Pod '%s' in namespace '%s' is running container '%s' with vulnerable image.

Vulnerability Details:
• CVE ID: %s
• Severity: %s (CVSS %.1f)
• Package: %s
• Installed Version: %s
• Fixed Version: %s

Description: %s

Recommendation: Update the image to use version %s or later.`,
		podName,
		podNamespace,
		containerName,
		vuln.VulnerabilityID,
		vuln.Severity,
		cvssScore,
		vuln.PkgName,
		vuln.InstalledVersion,
		vuln.FixedVersion,
		vuln.Description,
		vuln.FixedVersion,
	)

	// Add exploit warning if available
	if cveInfo != nil && cveInfo.ExploitAvailable {
		description += fmt.Sprintf(`

⚠️  WARNING: Public exploit is available!
• Exploit Maturity: %s
• Sources: %s

This vulnerability is being actively exploited. Immediate patching is recommended.`,
			cveInfo.ExploitMaturity,
			strings.Join(strings.Split(cveInfo.ExploitSources, ","), ", "),
		)
	}

	// Map severity
	severity := strings.ToLower(vuln.Severity)
	if severity == "critical" {
		severity = "critical"
	} else if severity == "high" {
		severity = "high"
	} else {
		severity = "medium" // Fallback
	}

	// Build affected resources JSON
	affectedResources := []map[string]interface{}{
		{
			"type":      "Pod",
			"uid":       podUID,
			"name":      podName,
			"namespace": podNamespace,
			"clusterId": clusterID,
		},
	}
	affectedResourcesJSON, _ := json.Marshal(affectedResources)

	// Build details JSON
	details := map[string]interface{}{
		"container_name":    containerName,
		"image":             fmt.Sprintf("%s:%s", scanResult.ImageName, scanResult.ImageTag),
		"installed_version": vuln.InstalledVersion,
		"references":        vuln.References,
	}
	if cveInfo != nil {
		details["exploit_maturity"] = cveInfo.ExploitMaturity
		details["exploit_sources"] = strings.Split(cveInfo.ExploitSources, ",")
		details["cwe_ids"] = strings.Split(cveInfo.CWEIDs, ",")
	}
	detailsJSON, _ := json.Marshal(details)

	// Create insight
	insight := &models.Insight{
		Type:              "vulnerability",
		Severity:          severity,
		Description:       description,
		AffectedResources: string(affectedResourcesJSON),
		RecommendedAction: fmt.Sprintf("Update image %s:%s to version %s or later", scanResult.ImageName, scanResult.ImageTag, vuln.FixedVersion),
		Status:            "active",
		Source:            "cve-scanner",

		// CVE-specific fields
		CVEID:            vuln.VulnerabilityID,
		CVSSScore:        &cvssScore,
		CVSSVector:       getCVSSVector(cveInfo, vuln),
		ExploitAvailable: cveInfo != nil && cveInfo.ExploitAvailable,
		PackageName:      vuln.PkgName,
		InstalledVersion: vuln.InstalledVersion,
		FixedVersion:     vuln.FixedVersion,
	}

	// Save insight (this triggers risk scoring automatically!)
	return p.insightManager.CreateOrUpdateInsight(insight)
}

// getCVSSVector extracts CVSS vector from CVE info or vulnerability
func getCVSSVector(cveInfo *models.CVE, vuln Vulnerability) string {
	if cveInfo != nil && cveInfo.CVSSVector != "" {
		return cveInfo.CVSSVector
	}
	// Try to extract from vuln.CVSS
	if vuln.CVSS != nil {
		if nvd, ok := vuln.CVSS["nvd"].(map[string]interface{}); ok {
			if vector, ok := nvd["V3Vector"].(string); ok {
				return vector
			}
			if vector, ok := nvd["V2Vector"].(string); ok {
				return vector
			}
		}
	}
	return ""
}

