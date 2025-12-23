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
	m.logger.Printf("Matching CVEs for SBOM ID %d (%d components)", sbom.ID, sbom.ComponentCount)

	// Load SBOM components
	var components []models.SBOMComponent
	if err := m.db.WithContext(ctx).
		Where("sbom_id = ? AND deleted_at IS NULL", sbom.ID).
		Find(&components).Error; err != nil {
		return 0.0, fmt.Errorf("failed to load SBOM components: %w", err)
	}

	m.logger.Printf("Found %d components to match", len(components))

	matches := make([]*models.CVEMatch, 0)

	// Process each component
	for _, component := range components {
		// 1. Parse PURL
		purl, err := ParsePURL(component.PURL)
		if err != nil {
			m.logger.Printf("⚠️  Failed to parse PURL %s: %v", component.PURL, err)
			continue
		}

		// Normalize ecosystem for DB queries (align SBOM PURL with OSV loader ecosystem values)
		queryEcosystem := normalizeQueryEcosystem(purl)

		// 2. Query CVE database
		cves, err := m.dbManager.GetVulnerabilitiesForPackage(
			ctx,
			queryEcosystem,
			purl.Name,
			component.ComponentVersion,
		)
		if err != nil {
			m.logger.Printf("⚠️  Failed to query CVEs for %s: %v", component.ComponentName, err)
			continue
		}

		if len(cves) == 0 {
			continue // No CVEs found
		}

		m.logger.Printf("Found %d potential CVEs for %s", len(cves), component.ComponentName)

		// 3. Check version constraints
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
				SBOMID:      sbom.ID,
				ComponentID: component.ID,
				CVEID:       cveData.ID,
				Severity:    strings.ToUpper(cveData.Severity),
				CVSSScore:   &cveData.CVSSScore,
				FixedVersion: cveData.FixedVersion,
				MatchedAt:   component.CreatedAt, // Use component creation time
				Matcher:     "custom",
				DBVersion:   "", // Will be set if available
			}

			matches = append(matches, match)
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
	case "deb":
		// OSV loader stores ecosystem as distro (debian/ubuntu)
		if ns != "" {
			return ns
		}
		return "debian"
	case "apk":
		// OSV loader stores "alpine"
		if ns != "" {
			return ns
		}
		return "alpine"
	case "rpm":
		// Use namespace if present (e.g., centos/redhat)
		if ns != "" {
			return ns
		}
		return "rpm"
	case "golang":
		// OSV loader normalizes to "go"
		return "go"
	default:
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

