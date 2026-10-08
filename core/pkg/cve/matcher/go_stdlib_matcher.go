package matcher

import (
	"context"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
)

// matchGoStdlib matches Go stdlib vulnerabilities based on SBOM-level GoVersion
// and OSV mirror entries for ecosystem=go, package_name=stdlib. It reports false when the
// stdlib advisories could not be loaded.
func (m *Matcher) matchGoStdlib(
	ctx context.Context,
	sbom *models.SBOM,
	matches *[]*models.CVEMatch,
) (ok bool) {
	v := strings.TrimSpace(sbom.GoVersion)
	if v == "" {
		return true
	}
	v = strings.TrimPrefix(v, "go") // go1.21.1 -> 1.21.1

	cves, err := m.dbManager.GetGoStdlibVulns(ctx)
	if err != nil {
		return false
	}

	findings := make(map[string]*models.CVEMatch)
	for _, cveData := range cves {
		vulnerable, err := m.comparator.IsVulnerable(v, cveData.Constraint, "go")
		if err != nil || !vulnerable {
			continue
		}
		if existing := findings[cveData.ID]; existing != nil {
			mergeMatch(existing, cveData)
			continue
		}
		match := &models.CVEMatch{
			ClusterID:           sbom.ClusterID,
			SBOMID:              sbom.ID,
			PodUID:              sbom.PodUID,
			ContainerName:       sbom.ContainerName,
			CVEID:               cveData.ID,
			PackageName:         "stdlib",
			PackageVersion:      v,
			PURL:                "",
			Severity:            strings.ToUpper(cveData.Severity),
			CVSS:                float32(cveData.CVSSScore),
			FixedVersion:        cveData.FixedVersion,
			MatchedBy:           "fortuna-go-stdlib-matcher",
			HasConstraint:       strings.TrimSpace(cveData.Constraint) != "",
			ConstraintSatisfied: strings.TrimSpace(cveData.Constraint) != "",
			MatchedAt:           time.Now(),
		}
		rateMatch(match, cveData)
		findings[cveData.ID] = match
		*matches = append(*matches, match)
	}
	return true
}
