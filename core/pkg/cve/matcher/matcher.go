package matcher

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Matcher matches CVEs against SBOM components
type Matcher struct {
	dbManager   *database.Manager
	comparator  *VersionComparator
	db          *gorm.DB
	logger      *log.Logger
}

// NewMatcher creates a new CVE matcher
func NewMatcher(
	dbManager *database.Manager,
	db *gorm.DB,
) *Matcher {
	return &Matcher{
		dbManager:  dbManager,
		comparator: NewVersionComparator(),
		db:         db,
		logger:     log.New(log.Writer(), "[CVEMatcher] ", log.LstdFlags),
	}
}

// MatchSBOM matches CVEs against an SBOM
func (m *Matcher) MatchSBOM(
	ctx context.Context,
	sbom *models.SBOM,
) ([]*models.CVEMatch, error) {
	m.logger.Printf("Matching CVEs for SBOM ID %d (%d packages)", sbom.ID, sbom.PackageCount)

	// Load SBOM components
	var components []models.SBOMComponent
		if err := m.db.WithContext(ctx).
			Where("sbom_id = ? AND deleted_at IS NULL", sbom.ID).
			Find(&components).Error; err != nil {
			return nil, fmt.Errorf("failed to load SBOM components: %w", err)
		}

	m.logger.Printf("Found %d components to match", len(components))

	matches := make([]*models.CVEMatch, 0)

	// OPTIMIZATION: Group components by ecosystem and query CVEs in bulk
	ecosystemPackages := make(map[string][]string)
	componentsByName := make(map[string]*models.SBOMComponent)
	purlsByName := make(map[string]*PURL)

	for i := range components {
		component := &components[i]

		// 1. Parse PURL
		purl, err := ParsePURL(component.PURL)
		if err != nil {
			m.logger.Printf("⚠️  Failed to parse PURL %s: %v", component.PURL, err)
			continue
		}

		// Normalize ecosystem for DB queries
		queryEcosystem := normalizeQueryEcosystem(purl)

		// Group by ecosystem
		ecosystemPackages[queryEcosystem] = append(ecosystemPackages[queryEcosystem], purl.Name)
		componentsByName[component.ComponentName] = component
		purlsByName[component.ComponentName] = purl
	}

	// 2. Bulk query CVEs for all packages per ecosystem
	for ecosystem, packageNames := range ecosystemPackages {
		m.logger.Printf("Bulk querying CVEs for %d packages in ecosystem %s", len(packageNames), ecosystem)

		packageCVEs, err := m.dbManager.GetVulnerabilitiesForPackages(ctx, ecosystem, packageNames)
		if err != nil {
			m.logger.Printf("⚠️  Failed to bulk query CVEs for ecosystem %s: %v", ecosystem, err)
			continue
		}

		totalCVEs := 0
		for _, cves := range packageCVEs {
			totalCVEs += len(cves)
		}
		m.logger.Printf("Found %d total CVEs for %d packages in ecosystem %s", totalCVEs, len(packageNames), ecosystem)

		// 3. Process each package's CVEs
		for pkgName, cves := range packageCVEs {
			component := componentsByName[pkgName]
			purl := purlsByName[pkgName]

			if component == nil || purl == nil {
				continue
			}

			// Check version constraints for each CVE
			for _, cveData := range cves {
				vulnerable, err := m.comparator.IsVulnerable(
					component.ComponentVersion,
					cveData.Constraint,
					purl.Ecosystem,
				)
				if err != nil {
					m.logger.Printf("⚠️  Version comparison failed for %s: %v", component.ComponentName, err)
					continue
				}

				if !vulnerable {
					continue // Not vulnerable
				}

				// 4. Create match
				match := &models.CVEMatch{
					SBOMID:         sbom.ID,
					PodUID:         sbom.PodUID,
					ContainerName:  sbom.ContainerName,
					CVEID:          cveData.ID,
					PackageName:    component.ComponentName,
					PackageVersion: component.ComponentVersion,
					PURL:           component.PURL,
					Severity:       strings.ToUpper(cveData.Severity),
					CVSS:           float32(cveData.CVSSScore), // Convert to float32
					FixedVersion:   cveData.FixedVersion,
					MatchedBy:      "fortuna-core-cve-matcher",
					MatchedAt:      component.CreatedAt,
				}

				matches = append(matches, match)
			}
		}
	}

	m.logger.Printf("✅ Found %d CVE matches for SBOM ID %d", len(matches), sbom.ID)
	return matches, nil
}

// normalizeQueryEcosystem maps PURL ecosystem/namespace into the ecosystem values
// stored in PostgreSQL by the OSV loader (e.g., debian/ubuntu/alpine/go).
func normalizeQueryEcosystem(p *PURL) string {
	if p == nil {
		return ""
	}
	eco := strings.ToLower(strings.TrimSpace(p.Ecosystem))
	ns := strings.ToLower(strings.TrimSpace(p.Namespace))

	switch eco {
	case "deb", "package_type_dpkg", "package_type_deb":
		// OSV loader stores ecosystem as distro (debian/ubuntu)
		// Handle both "deb", "package_type_dpkg", and "package_type_deb" formats
		if ns != "" {
			return ns
		}
		return "debian"
	case "apk", "package_type_apk":
		// OSV loader stores "alpine"
		// Handle both "apk" and "package_type_apk" formats
		if ns != "" {
			return ns
		}
		return "alpine"
	case "rpm", "package_type_rpm":
		// Use namespace if present (e.g., centos/redhat)
		if ns != "" {
			return ns
		}
		return "linux" // Most RPM vulnerabilities are in "linux" ecosystem
	case "golang", "go":
		// OSV loader normalizes to "go"
		return "go"
	default:
		// For unknown ecosystems, try to use namespace or return as-is
		if ns != "" {
			return ns
		}
		return eco
	}
}

// FilterBySeverity filters matches by severity
func (m *Matcher) FilterBySeverity(
	matches []*models.CVEMatch,
	severities []string,
) []*models.CVEMatch {
	if len(severities) == 0 {
		return matches // No filter
	}

	filtered := make([]*models.CVEMatch, 0)
	severityMap := make(map[string]bool)
	for _, sev := range severities {
		severityMap[strings.ToUpper(sev)] = true
	}

	for _, match := range matches {
		if severityMap[strings.ToUpper(match.Severity)] {
			filtered = append(filtered, match)
		}
	}

	return filtered
}

