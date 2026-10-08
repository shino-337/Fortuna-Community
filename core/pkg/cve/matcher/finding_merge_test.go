package matcher

import (
	"testing"

	"github.com/fortuna/core/pkg/cve"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestMergeMatchKeepsBestSeveritySource(t *testing.T) {
	m := &models.CVEMatch{CVEID: "CVE-2024-1"}
	rateMatch(m, &cve.CVE{ID: "CVE-2024-1", AdvisoryID: "DSA-1", Severity: "CRITICAL", CVSSScore: 9.8, SeverityTier: 5, SeveritySource: "errata_cvss"})
	// A vendor rating beats a higher CVSS from an erratum.
	mergeMatch(m, &cve.CVE{ID: "CVE-2024-1", AdvisoryID: "DEBIAN-CVE-2024-1", Severity: "LOW", SeverityTier: 1, SeveritySource: "vendor", FixedVersion: "1.2"})
	// A worse source does not override, and an advisory is listed once.
	mergeMatch(m, &cve.CVE{ID: "CVE-2024-1", AdvisoryID: "DSA-1", Severity: "HIGH", SeverityTier: 2, SeveritySource: "advisory_cvss", FixedVersion: "1.3"})
	// Legacy rows carry no tier and never override a ranked one.
	mergeMatch(m, &cve.CVE{ID: "CVE-2024-1", Severity: "CRITICAL"})
	require.Equal(t, "LOW", m.Severity)
	require.Equal(t, "vendor", m.SeveritySource)
	require.Equal(t, "1.2", m.FixedVersion)
	require.Equal(t, []string{"DSA-1", "DEBIAN-CVE-2024-1"}, []string(m.AdvisoryIDs))

	// Same tier: the higher severity wins.
	m2 := &models.CVEMatch{CVEID: "CVE-2024-2"}
	rateMatch(m2, &cve.CVE{AdvisoryID: "GO-1", Severity: "MEDIUM", SeverityTier: 1, SeveritySource: "vendor"})
	mergeMatch(m2, &cve.CVE{AdvisoryID: "GHSA-1", Severity: "HIGH", SeverityTier: 1, SeveritySource: "vendor"})
	require.Equal(t, "HIGH", m2.Severity)
}
