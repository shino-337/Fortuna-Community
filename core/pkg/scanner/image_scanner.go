//go:build legacy_trivy

package scanner

import (
	"bytes"
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

// DEPRECATED: This scanner uses Trivy API/Server which is no longer used.
// The system now uses custom SBOM pipeline (zero-dependency).
// This code is kept for reference only and will be removed in future versions.
//
// ImageScanner scans container images using Trivy
type ImageScanner struct {
	db         *gorm.DB
	trivyURL   string
	httpClient *http.Client
	cacheTTL   time.Duration
	logger     *log.Logger
}

// NewImageScanner creates a new image scanner
func NewImageScanner(db *gorm.DB, trivyURL string) *ImageScanner {
	return &ImageScanner{
		db:       db,
		trivyURL: trivyURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		cacheTTL: 24 * time.Hour,
		logger:   log.New(log.Writer(), "[ImageScanner] ", log.LstdFlags),
	}
}

// TrivyResult represents the response from Trivy API
type TrivyResult struct {
	ArtifactName string `json:"ArtifactName"`
	Metadata     struct {
		Version string `json:"Version"`
	} `json:"Metadata"`
	Results []struct {
		Target          string          `json:"Target"`
		Vulnerabilities []Vulnerability `json:"Vulnerabilities"`
		Packages        []interface{}   `json:"Packages"`
	} `json:"Results"`
}

// ScanImage scans a container image
func (s *ImageScanner) ScanImage(ctx context.Context, imageRef string) (*models.ImageScanResult, error) {
	start := time.Now()

	// Parse image reference
	imageName, imageTag := parseImageRef(imageRef)

	// Check cache (< 24h old)
	cached, err := s.checkCache(ctx, imageName, imageTag)
	if err == nil && cached != nil {
		s.logger.Printf("Using cached scan for %s:%s", imageName, imageTag)
		return cached, nil
	}

	// Perform scan via Trivy
	trivyResult, err := s.trivyScan(ctx, imageRef)
	if err != nil {
		return nil, fmt.Errorf("trivy scan failed: %w", err)
	}

	// Convert to our model
	scanResult := &models.ImageScanResult{
		ImageName:          imageName,
		ImageTag:           imageTag,
		ImageDigest:        trivyResult.ArtifactName,
		ScannedAt:          time.Now(),
		ScannerVersion:     trivyResult.Metadata.Version,
		ScanDurationSeconds: time.Since(start).Seconds(),
		Status:             "completed",
	}

	// Process vulnerabilities
	if len(trivyResult.Results) > 0 {
		result := trivyResult.Results[0]
		for _, vuln := range result.Vulnerabilities {
			// Count by severity
			switch strings.ToUpper(vuln.Severity) {
			case "CRITICAL":
				scanResult.CriticalCount++
			case "HIGH":
				scanResult.HighCount++
			case "MEDIUM":
				scanResult.MediumCount++
			case "LOW":
				scanResult.LowCount++
			default:
				scanResult.UnknownCount++
			}
		}
		scanResult.TotalVulnerabilities = scanResult.CriticalCount + scanResult.HighCount +
			scanResult.MediumCount + scanResult.LowCount + scanResult.UnknownCount

		// Store vulnerabilities as JSON
		vulnsJSON, _ := json.Marshal(result.Vulnerabilities)
		pkgsJSON, _ := json.Marshal(result.Packages)
		scanResult.Vulnerabilities = string(vulnsJSON)
		scanResult.Packages = string(pkgsJSON)
	}

	// Set cache expiry
	expiresAt := time.Now().Add(s.cacheTTL)
	scanResult.ExpiresAt = &expiresAt
	scanResult.CacheKey = fmt.Sprintf("%s:%s", imageName, imageTag)

	// Save to database
	if err := s.saveScanResult(ctx, scanResult); err != nil {
		return nil, fmt.Errorf("failed to save scan result: %w", err)
	}

	s.logger.Printf("Scanned %s:%s - found %d vulnerabilities (Critical: %d, High: %d)",
		imageName, imageTag, scanResult.TotalVulnerabilities,
		scanResult.CriticalCount, scanResult.HighCount)

	return scanResult, nil
}

// checkCache checks for recent scan
func (s *ImageScanner) checkCache(ctx context.Context, imageName, imageTag string) (*models.ImageScanResult, error) {
	var result models.ImageScanResult
	err := s.db.WithContext(ctx).
		Where("image_name = ? AND image_tag = ? AND status = ? AND expires_at > ? AND deleted_at IS NULL",
			imageName, imageTag, "completed", time.Now()).
		Order("scanned_at DESC").
		First(&result).Error

	if err != nil {
		return nil, err
	}
	return &result, nil
}

// trivyScan calls Trivy server
func (s *ImageScanner) trivyScan(ctx context.Context, imageRef string) (*TrivyResult, error) {
	url := fmt.Sprintf("%s/v1/scan", s.trivyURL)
	reqBody := map[string]string{
		"image": imageRef,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Trivy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Trivy returned status %d", resp.StatusCode)
	}

	var result TrivyResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode Trivy response: %w", err)
	}

	return &result, nil
}

// saveScanResult stores result in database
func (s *ImageScanner) saveScanResult(ctx context.Context, result *models.ImageScanResult) error {
	return s.db.WithContext(ctx).Create(result).Error
}

// parseImageRef parses image reference into name and tag
func parseImageRef(imageRef string) (string, string) {
	return parseImageRefFromString(imageRef)
}

