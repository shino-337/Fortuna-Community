package nvd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/cve"
)

// Client queries NVD API for CVE data
type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
	logger  *log.Logger
}

// NewClient creates a new NVD API client
func NewClient() *Client {
	return &Client{
		baseURL: "https://services.nvd.nist.gov/rest/json/cves/2.0",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: log.New(log.Writer(), "[NVDAPIClient] ", log.LstdFlags),
	}
}

// SetAPIKey sets NVD API key (optional, increases rate limit)
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// Query queries NVD API for CVEs
func (c *Client) Query(
	ctx context.Context,
	ecosystem string,
	name string,
	version string,
) ([]*cve.CVE, error) {
	// Query NVD API
	// https://services.nvd.nist.gov/rest/json/cves/2.0?keywordSearch=openssl

	url := fmt.Sprintf("%s?keywordSearch=%s&resultsPerPage=100", c.baseURL, name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("apiKey", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call NVD API: %w", err)
	}
	defer resp.Body.Close()

	// 429 Too Many Requests: respect Retry-After and retry once
	if resp.StatusCode == http.StatusTooManyRequests {
		_ = resp.Body.Close()
		retryAfter := 30 * time.Second
		if s := resp.Header.Get("Retry-After"); s != "" {
			if sec, err := strconv.Atoi(s); err == nil && sec > 0 && sec <= 120 {
				retryAfter = time.Duration(sec) * time.Second
			}
		}
		c.logger.Printf("NVD API rate limit (429); waiting %v before retry (hint: NVD_API_KEY increases limit)", retryAfter)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("NVD API rate limit (429): %w", ctx.Err())
		case <-time.After(retryAfter):
			// Retry once (new request; GET has no body so req is reusable)
			resp2, err2 := c.client.Do(req)
			if err2 != nil {
				return nil, fmt.Errorf("NVD API rate limit (429), retry failed: %w", err2)
			}
			resp = resp2
			if resp.StatusCode != http.StatusOK {
				defer resp.Body.Close()
				return nil, fmt.Errorf("NVD API returned status %d (after 429 retry); set NVD_API_KEY for higher rate limit", resp.StatusCode)
			}
		}
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NVD API returned status %d", resp.StatusCode)
	}

	// Parse response
	var nvdResp NVDResponse
	if err := json.NewDecoder(resp.Body).Decode(&nvdResp); err != nil {
		return nil, fmt.Errorf("failed to decode NVD response: %w", err)
	}

	// Convert to our CVE format
	cves := make([]*cve.CVE, 0)
	for _, item := range nvdResp.Vulnerabilities {
		cve := c.convertNVDtoCVE(item, ecosystem, name, version)
		if cve != nil {
			cves = append(cves, cve)
		}
	}

	c.logger.Printf("Found %d CVEs from NVD API for %s", len(cves), name)
	return cves, nil
}

// convertNVDtoCVE converts NVD format to our CVE format
func (c *Client) convertNVDtoCVE(item NVDVulnerability, ecosystem, name, version string) *cve.CVE {
	// Extract CVSS score
	cvssScore := 0.0
	cvssVector := ""

	if item.CVE.Metrics != nil {
		if item.CVE.Metrics.CVSSMetricV31 != nil && len(item.CVE.Metrics.CVSSMetricV31) > 0 {
			cvssScore = item.CVE.Metrics.CVSSMetricV31[0].CVSSData.BaseScore
			cvssVector = item.CVE.Metrics.CVSSMetricV31[0].CVSSData.VectorString
		} else if item.CVE.Metrics.CVSSMetricV30 != nil && len(item.CVE.Metrics.CVSSMetricV30) > 0 {
			cvssScore = item.CVE.Metrics.CVSSMetricV30[0].CVSSData.BaseScore
			cvssVector = item.CVE.Metrics.CVSSMetricV30[0].CVSSData.VectorString
		} else if item.CVE.Metrics.CVSSMetricV2 != nil && len(item.CVE.Metrics.CVSSMetricV2) > 0 {
			cvssScore = item.CVE.Metrics.CVSSMetricV2[0].CVSSData.BaseScore
			cvssVector = item.CVE.Metrics.CVSSMetricV2[0].CVSSData.VectorString
		}
	}

	// Determine severity from CVSS score
	severity := "LOW"
	if cvssScore >= 9.0 {
		severity = "CRITICAL"
	} else if cvssScore >= 7.0 {
		severity = "HIGH"
	} else if cvssScore >= 4.0 {
		severity = "MEDIUM"
	}

	// Extract references
	references := make([]string, 0)
	for _, ref := range item.CVE.References {
		if ref.URL != "" {
			references = append(references, ref.URL)
		}
	}

	var published, modified time.Time
	if t, ok := parseNVDTime(item.CVE.PublishedStr); ok {
		published = t
	}
	if t, ok := parseNVDTime(item.CVE.LastModifiedStr); ok {
		modified = t
	}
	return &cve.CVE{
		ID:          item.CVE.ID,
		Description: item.CVE.Descriptions[0].Value, // Use first description
		Severity:    severity,
		CVSSScore:   cvssScore,
		CVSSVector:  cvssVector,
		Constraint:  "", // NVD doesn't provide version constraints directly
		Published:   published,
		Modified:    modified,
		References:  references,
	}
}

// NVDResponse represents NVD API response
type NVDResponse struct {
	Vulnerabilities []NVDVulnerability `json:"vulnerabilities"`
	TotalResults    int                `json:"totalResults"`
}

// NVD date layouts: API may return with or without timezone (e.g. "2004-12-31T05:00:00.000" or "2004-12-31T05:00:00.000Z").
var nvdTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05.000",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
}

func parseNVDTime(s string) (t time.Time, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range nvdTimeLayouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// NVDVulnerability represents a vulnerability in NVD format
type NVDVulnerability struct {
	CVE struct {
		ID           string `json:"id"`
		PublishedStr  string `json:"published"`
		LastModifiedStr string `json:"lastModified"`
		Descriptions []struct {
			Value string `json:"value"`
		} `json:"descriptions"`
		Metrics *struct {
			CVSSMetricV31 []struct {
				CVSSData struct {
					BaseScore   float64 `json:"baseScore"`
					VectorString string `json:"vectorString"`
				} `json:"cvssData"`
			} `json:"cvssMetricV31"`
			CVSSMetricV30 []struct {
				CVSSData struct {
					BaseScore   float64 `json:"baseScore"`
					VectorString string `json:"vectorString"`
				} `json:"cvssData"`
			} `json:"cvssMetricV30"`
			CVSSMetricV2 []struct {
				CVSSData struct {
					BaseScore   float64 `json:"baseScore"`
					VectorString string `json:"vectorString"`
				} `json:"cvssData"`
			} `json:"cvssMetricV2"`
		} `json:"metrics"`
		References []struct {
			URL string `json:"url"`
		} `json:"references"`
	} `json:"cve"`
}


