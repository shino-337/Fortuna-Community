package worker

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestPersistMatches_ReplacesSBOMFindings(t *testing.T) {
	db := newReplayWorkerTestDB(t)
	w := &CVEMatcherWorker{db: db}
	sb := seedFinalizedSBOM(t, db)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX idx_cve_matches_unique_sbom_package_cve_all ON cve_matches (sbom_id, package_name, cve_id)`).Error)
	ctx := context.Background()
	match := func(cveID, severity string, advisories ...string) *models.CVEMatch {
		return &models.CVEMatch{SBOMID: sb.ID, PackageName: "openssl", PackageVersion: "3.0.1", CVEID: cveID,
			Severity: severity, AdvisoryIDs: advisories}
	}
	stored := func() map[string]string {
		var rows []models.CVEMatch
		require.NoError(t, db.Where("sbom_id = ?", sb.ID).Find(&rows).Error)
		out := map[string]string{}
		for _, r := range rows {
			out[r.CVEID] = r.Severity + " " + r.SeveritySource + " " + joinAdvisories(r.AdvisoryIDs)
		}
		return out
	}

	require.NoError(t, w.persistMatches(ctx, sb.ID, []*models.CVEMatch{match("DSA-5000-1", "HIGH"), match("CVE-2024-0001", "LOW")}, true))
	require.Equal(t, map[string]string{"DSA-5000-1": "HIGH  ", "CVE-2024-0001": "LOW  "}, stored())

	// A re-match under resolver v1.7: the CVE finding absorbs the DSA, with a new rating. Two
	// versions of the package share the row.
	rerun := func() []*models.CVEMatch {
		a := match("CVE-2024-0001", "CRITICAL", "DSA-5000-1")
		a.SeveritySource = "vendor"
		b := match("CVE-2024-0001", "CRITICAL", "DEBIAN-CVE-2024-0001")
		b.PackageVersion = "3.0.2"
		return []*models.CVEMatch{a, b}
	}
	// An incomplete run updates what it found and removes nothing.
	require.NoError(t, w.persistMatches(ctx, sb.ID, rerun(), false))
	require.Equal(t, map[string]string{"DSA-5000-1": "HIGH  ", "CVE-2024-0001": "CRITICAL vendor DSA-5000-1,DEBIAN-CVE-2024-0001"}, stored())
	// A complete run replaces the SBOM's findings.
	require.NoError(t, w.persistMatches(ctx, sb.ID, rerun(), true))
	require.Equal(t, map[string]string{"CVE-2024-0001": "CRITICAL vendor DSA-5000-1,DEBIAN-CVE-2024-0001"}, stored())
	// Nothing matches any more.
	require.NoError(t, w.persistMatches(ctx, sb.ID, nil, true))
	require.Empty(t, stored())
}

func joinAdvisories(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	return out
}
