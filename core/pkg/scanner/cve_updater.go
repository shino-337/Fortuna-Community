//go:build legacy_trivy

package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ksam/core/pkg/models"
	"gorm.io/gorm"
)

// CVEUpdater fetches and updates CVE data from NVD
type CVEUpdater struct {
	db         *gorm.DB
	nvdAPIKey  string
	nvdBaseURL string
	httpClient *http.Client
	logger     *log.Logger
}

// NewCVEUpdater creates a new CVE updater
func NewCVEUpdater(db *gorm.DB, nvdAPIKey string) *CVEUpdater {
	return &CVEUpdater{
		db:         db,
		nvdAPIKey:  nvdAPIKey,
		nvdBaseURL: "https://services.nvd.nist.gov/rest/json/cves/2.0",
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		logger: log.New(log.Writer(), "[CVEUpdater] ", log.LstdFlags),
	}
}

// NVDVulnerability represents a CVE from NVD API
type NVDVulnerability struct {
	ID       string `json:"id"`
	Source   string `json:"sourceIdentifier"`
	Published string `json:"published"`
	Modified  string `json:"lastModified"`
	VulnStatus string `json:"vulnStatus"`
	Descriptions struct {
		LangData []struct {
			Lang  string `json:"lang"`
			Value string `json:"value"`
		} `json:"langData"`
	} `json:"descriptions"`
	Metrics struct {
		CvssMetricV31 []struct {
			Source   string  `json:"source"`
			Type     string  `json:"type"`
			CvssData struct {
				Version     string  `json:"version"`
				VectorString string `json:"vectorString"`
				BaseScore   float64 `json:"baseScore"`
				BaseSeverity string `json:"baseSeverity"`
			} `json:"cvssData"`
		} `json:"cvssMetricV31"`
		CvssMetricV30 []struct {
			Source   string  `json:"source"`
			Type     string  `json:"type"`
			CvssData struct {
				Version     string  `json:"version"`
				VectorString string `json:"vectorString"`
				BaseScore   float64 `json:"baseScore"`
				BaseSeverity string `json:"baseSeverity"`
			} `json:"cvssData"`
		} `json:"cvssMetricV30"`
		CvssMetricV2 []struct {
			Source   string  `json:"source"`
			Type     string  `json:"type"`
			CvssData struct {
				Version     string  `json:"version"`
				VectorString string `json:"vectorString"`
				BaseScore   float64 `json:"baseScore"`
			} `json:"cvssData"`
		} `json:"cvssMetricV2"`
	} `json:"metrics"`
	Weaknesses []struct {
		Description []struct {
			Lang  string `json:"lang"`
			Value string `json:"value"`
		} `json:"description"`
	} `json:"weaknesses"`
	References struct {
		ReferenceData []struct {
			URL string `json:"url"`
		} `json:"referenceData"`
	} `json:"references"`
}

// NVDResponse represents NVD API response
type NVDResponse struct {
	ResultsPerPage int                `json:"resultsPerPage"`
	StartIndex     int                `json:"startIndex"`
	TotalResults   int                `json:"totalResults"`
	Vulnerabilities []NVDVulnerability `json:"vulnerabilities"`
}

// UpdateCVEDatabase fetches latest CVEs from NVD and updates database
func (u *CVEUpdater) UpdateCVEDatabase(ctx context.Context) error {
	u.logger.Println("Starting CVE database update...")

	// Get last update timestamp
	var lastUpdate time.Time
	u.db.Raw("SELECT MAX(published_date) FROM cves WHERE deleted_at IS NULL").Scan(&lastUpdate)

	if lastUpdate.IsZero() {
		// First run: fetch last 30 days
		lastUpdate = time.Now().AddDate(0, 0, -30)
		u.logger.Printf("No previous updates found, fetching CVEs from last 30 days")
	} else {
		u.logger.Printf("Last update: %s, fetching new CVEs since then", lastUpdate.Format(time.RFC3339))
	}

	// Fetch CVEs from NVD
	cves, err := u.fetchCVEsFromNVD(ctx, lastUpdate, time.Now())
	if err != nil {
		return fmt.Errorf("failed to fetch CVEs: %w", err)
	}

	u.logger.Printf("Fetched %d CVEs from NVD", len(cves))

	// Process each CVE
	processed := 0
	for _, nvdCVE := range cves {
		cve, pkgVulns, err := u.parseCVE(&nvdCVE)
		if err != nil {
			u.logger.Printf("Failed to parse CVE %s: %v", nvdCVE.ID, err)
			continue
		}

		// Upsert to database
		if err := u.upsertCVE(ctx, cve, pkgVulns); err != nil {
			u.logger.Printf("Failed to upsert CVE %s: %v", cve.CVEID, err)
			continue
		}

		processed++
	}

	u.logger.Printf("Updated %d CVEs in database", processed)
	return nil
}

// fetchCVEsFromNVD calls NVD API
func (u *CVEUpdater) fetchCVEsFromNVD(ctx context.Context, startDate, endDate time.Time) ([]NVDVulnerability, error) {
	url := fmt.Sprintf("%s?pubStartDate=%s&pubEndDate=%s&resultsPerPage=2000",
		u.nvdBaseURL,
		startDate.Format("2006-01-02T15:04:05.000"),
		endDate.Format("2006-01-02T15:04:05.000"))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if u.nvdAPIKey != "" {
		req.Header.Set("apiKey", u.nvdAPIKey)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call NVD API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NVD API returned status %d", resp.StatusCode)
	}

	var nvdResp NVDResponse
	if err := json.NewDecoder(resp.Body).Decode(&nvdResp); err != nil {
		return nil, fmt.Errorf("failed to decode NVD response: %w", err)
	}

	return nvdResp.Vulnerabilities, nil
}

// parseCVE converts NVD format to our model
func (u *CVEUpdater) parseCVE(nvdVuln *NVDVulnerability) (*models.CVE, []*models.PackageVulnerability, error) {
	// Extract description
	description := ""
	for _, desc := range nvdVuln.Descriptions.LangData {
		if desc.Lang == "en" {
			description = desc.Value
			break
		}
	}

	// Extract CVSS score and vector
	var cvssScore float64
	var cvssVector string
	var cvssVersion string
	severity := "MEDIUM"

	// Try CVSS v3.1 first
	if len(nvdVuln.Metrics.CvssMetricV31) > 0 {
		metric := nvdVuln.Metrics.CvssMetricV31[0]
		cvssScore = metric.CvssData.BaseScore
		cvssVector = metric.CvssData.VectorString
		cvssVersion = "3.1"
		severity = metric.CvssData.BaseSeverity
	} else if len(nvdVuln.Metrics.CvssMetricV30) > 0 {
		metric := nvdVuln.Metrics.CvssMetricV30[0]
		cvssScore = metric.CvssData.BaseScore
		cvssVector = metric.CvssData.VectorString
		cvssVersion = "3.0"
		severity = metric.CvssData.BaseSeverity
	} else if len(nvdVuln.Metrics.CvssMetricV2) > 0 {
		metric := nvdVuln.Metrics.CvssMetricV2[0]
		cvssScore = metric.CvssData.BaseScore
		cvssVector = metric.CvssData.VectorString
		cvssVersion = "2.0"
		// Map CVSS v2 to severity
		if cvssScore >= 7.0 {
			severity = "HIGH"
		} else if cvssScore >= 4.0 {
			severity = "MEDIUM"
		} else {
			severity = "LOW"
		}
	}

	// Parse dates
	publishedDate, _ := time.Parse(time.RFC3339, nvdVuln.Published)
	modifiedDate, _ := time.Parse(time.RFC3339, nvdVuln.Modified)

	// Extract CWE IDs
	cweIDs := []string{}
	for _, weakness := range nvdVuln.Weaknesses {
		for _, desc := range weakness.Description {
			if strings.HasPrefix(desc.Value, "CWE-") {
				cweIDs = append(cweIDs, desc.Value)
			}
		}
	}

	// Extract references
	references := []map[string]interface{}{}
	for _, ref := range nvdVuln.References.ReferenceData {
		references = append(references, map[string]interface{}{
			"url": ref.URL,
		})
	}
	referencesJSON, _ := json.Marshal(references)

	cve := &models.CVE{
		CVEID:            nvdVuln.ID,
		CVSSScore:        cvssScore,
		CVSSVector:       cvssVector,
		CVSSVersion:      cvssVersion,
		Severity:         severity,
		Description:      description,
		PublishedDate:    &publishedDate,
		LastModifiedDate: &modifiedDate,
		ExploitAvailable: false, // NVD doesn't provide this, would need separate source
		ExploitSources:   "",    // Would need separate source
		References:       string(referencesJSON),
		CWEIDs:           strings.Join(cweIDs, ","),
		Source:           "nvd",
		SourceURL:        fmt.Sprintf("https://nvd.nist.gov/vuln/detail/%s", nvdVuln.ID),
	}

	// Package vulnerabilities would be parsed from CPE (Common Platform Enumeration)
	// For now, return empty - this would need CPE parsing logic
	pkgVulns := []*models.PackageVulnerability{}

	return cve, pkgVulns, nil
}

// upsertCVE saves or updates CVE
func (u *CVEUpdater) upsertCVE(ctx context.Context, cve *models.CVE, pkgVulns []*models.PackageVulnerability) error {
	// Upsert CVE
	if err := u.db.WithContext(ctx).
		Where("cve_id = ?", cve.CVEID).
		Assign(cve).
		FirstOrCreate(cve).Error; err != nil {
		return fmt.Errorf("failed to upsert CVE: %w", err)
	}

	// Upsert package vulnerabilities
	for _, pkgVuln := range pkgVulns {
		pkgVuln.CVEID = cve.CVEID
		if err := u.db.WithContext(ctx).
			Where("cve_id = ? AND package_name = ? AND ecosystem = ? AND version_end_excluding = ?",
				pkgVuln.CVEID, pkgVuln.PackageName, pkgVuln.Ecosystem, pkgVuln.VersionEndExcluding).
			Assign(pkgVuln).
			FirstOrCreate(pkgVuln).Error; err != nil {
			u.logger.Printf("Warning: Failed to upsert package vulnerability: %v", err)
			continue
		}
	}

	return nil
}

