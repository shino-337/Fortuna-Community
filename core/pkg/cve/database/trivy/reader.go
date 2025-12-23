package trivy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
	"github.com/fortuna/core/pkg/cve"
)

// Reader reads CVE data from Trivy DB (BoltDB format)
// 
// IMPORTANT: This is a DATA SOURCE only, NOT a dependency on Trivy tool.
// - We read CVE data from Trivy's BoltDB database file (read-only)
// - We do NOT run Trivy CLI, Trivy server, or any Trivy binary
// - This is zero-dependency design - only need BoltDB library to read the file
// - Trivy DB file can be downloaded/updated independently via CronJob
// - If Trivy DB is unavailable, system falls back to NVD API
type Reader struct {
	db     *bolt.DB
	logger *log.Logger
}

// NewReader creates a new Trivy DB reader
func NewReader(dbPath string) (*Reader, error) {
	// Open Trivy DB (BoltDB format, read-only)
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to open Trivy DB: %w", err)
	}

	return &Reader{
		db:     db,
		logger: log.New(log.Writer(), "[TrivyDBReader] ", log.LstdFlags),
	}, nil
}

// Close closes the database
func (r *Reader) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// Query queries CVEs for a package
func (r *Reader) Query(
	ctx context.Context,
	ecosystem string,
	name string,
	version string,
) ([]*cve.CVE, error) {
	cves := make([]*cve.CVE, 0)

	err := r.db.View(func(tx *bolt.Tx) error {
		// Trivy DB structure:
		// - Bucket "vulnerability" contains CVE details
		// - Bucket "advisory" contains package->CVE mappings
		// Format: "ecosystem:distro:package:version" -> [CVE IDs]

		// Get advisory bucket
		advisoryBucket := tx.Bucket([]byte("advisory"))
		if advisoryBucket == nil {
			return nil // No advisories found
		}

		// Build advisory key
		key := r.buildAdvisoryKey(ecosystem, name, version)

		// Get CVE IDs for this package
		cveIDsData := advisoryBucket.Get([]byte(key))
		if cveIDsData == nil {
			// Try without version (package-level)
			keyNoVersion := r.buildAdvisoryKey(ecosystem, name, "")
			cveIDsData = advisoryBucket.Get([]byte(keyNoVersion))
			if cveIDsData == nil {
				return nil // No vulnerabilities found
			}
		}

		// Parse CVE IDs
		var cveIDs []string
		if err := json.Unmarshal(cveIDsData, &cveIDs); err != nil {
			return fmt.Errorf("failed to unmarshal CVE IDs: %w", err)
		}

		// Get vulnerability bucket
		vulnBucket := tx.Bucket([]byte("vulnerability"))
		if vulnBucket == nil {
			return nil // No vulnerability details
		}

		// Get details for each CVE
		for _, cveID := range cveIDs {
			vulnData := vulnBucket.Get([]byte(cveID))
			if vulnData == nil {
				continue
			}

			var vuln TrivyVulnerability
			if err := json.Unmarshal(vulnData, &vuln); err != nil {
				continue
			}

			// Convert to our CVE format
			cve := r.convertToCVE(vuln, ecosystem, name, version)
			if cve != nil {
				cves = append(cves, cve)
			}
		}

		return nil
	})

	return cves, err
}

// buildAdvisoryKey builds advisory key for Trivy DB
func (r *Reader) buildAdvisoryKey(ecosystem, name, version string) string {
	// Trivy DB key format varies by ecosystem
	switch ecosystem {
	case "deb", "debian", "ubuntu":
		if version != "" {
			return fmt.Sprintf("debian:%s:%s", name, version)
		}
		return fmt.Sprintf("debian:%s", name)
	case "apk", "alpine":
		if version != "" {
			return fmt.Sprintf("alpine:%s:%s", name, version)
		}
		return fmt.Sprintf("alpine:%s", name)
	case "rpm", "redhat", "centos":
		if version != "" {
			return fmt.Sprintf("redhat:%s:%s", name, version)
		}
		return fmt.Sprintf("redhat:%s", name)
	default:
		if version != "" {
			return fmt.Sprintf("%s:%s:%s", ecosystem, name, version)
		}
		return fmt.Sprintf("%s:%s", ecosystem, name)
	}
}

// convertToCVE converts Trivy vulnerability to our CVE format
func (r *Reader) convertToCVE(vuln TrivyVulnerability, ecosystem, name, version string) *cve.CVE {
	// Extract CVSS score
	cvssScore := 0.0
	if len(vuln.CVSS) > 0 {
		// Use first CVSS entry (usually NVD)
		for _, cvss := range vuln.CVSS {
			if cvss.V3Score > 0 {
				cvssScore = cvss.V3Score
				break
			} else if cvss.V2Score > 0 {
				cvssScore = cvss.V2Score
				break
			}
		}
	}

	// Extract fixed version
	fixedVersion := ""
	if len(vuln.FixedVersions) > 0 {
		fixedVersion = vuln.FixedVersions[0]
	}

	// Extract constraint
	constraint := ""
	if len(vuln.VulnerableVersions) > 0 {
		constraint = vuln.VulnerableVersions[0]
	}

	return &cve.CVE{
		ID:            vuln.ID,
		Description:   vuln.Description,
		Severity:      vuln.Severity,
		CVSSScore:     cvssScore,
		CVSSVector:    vuln.CVSSVector,
		Constraint:    constraint,
		FixedVersion:  fixedVersion,
		Published:     vuln.PublishedDate,
		Modified:      vuln.LastModifiedDate,
		References:     vuln.References,
	}
}

// TrivyVulnerability represents Trivy's vulnerability format
type TrivyVulnerability struct {
	ID                string    `json:"id"`
	Description       string    `json:"description"`
	Severity          string    `json:"severity"`
	CVSS              []CVSS    `json:"cvss"`
	CVSSVector        string    `json:"cvss_vector"`
	VulnerableVersions []string `json:"vulnerable_versions"`
	FixedVersions     []string  `json:"fixed_versions"`
	PublishedDate     time.Time `json:"published_date"`
	LastModifiedDate  time.Time `json:"last_modified_date"`
	References        []string  `json:"references"`
}

// CVSS represents CVSS score
type CVSS struct {
	V2Score float64 `json:"v2_score"`
	V3Score float64 `json:"v3_score"`
	Vector  string  `json:"vector"`
}

