package matcher

import (
	"context"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
)

// matchGoStdlib matches Go stdlib vulnerabilities based on SBOM-level GoVersion
// and OSV mirror entries for ecosystem=go, package_name=stdlib.
func (m *Matcher) matchGoStdlib(
	ctx context.Context,
	sbom *models.SBOM,
	matches *[]*models.CVEMatch,
) {
	v := strings.TrimSpace(sbom.GoVersion)
	if v == "" {
		return
	}
	v = strings.TrimPrefix(v, "go") // go1.21.1 -> 1.21.1

	cves, err := m.dbManager.GetGoStdlibVulns(ctx)
	if err != nil || len(cves) == 0 {
		return
	}

	for _, cveData := range cves {
		vulnerable, err := m.comparator.IsVulnerable(v, cveData.Constraint, "go")
		if err != nil || !vulnerable {
			continue
		}
		*matches = append(*matches, &models.CVEMatch{
			SBOMID:         sbom.ID,
			PodUID:         sbom.PodUID,
			ContainerName:  sbom.ContainerName,
			CVEID:          cveData.ID,
			PackageName:    "stdlib",
			PackageVersion: v,
			PURL:           "",
			Severity:       strings.ToUpper(cveData.Severity),
			CVSS:           float32(cveData.CVSSScore),
			FixedVersion:   cveData.FixedVersion,
			MatchedBy:      "fortuna-go-stdlib-matcher",
			HasConstraint:       strings.TrimSpace(cveData.Constraint) != "",
			ConstraintSatisfied: strings.TrimSpace(cveData.Constraint) != "",
			MatchedAt:      time.Now(),
		})
	}
}

