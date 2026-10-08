package matcher_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/cve/loader"
	"github.com/fortuna/core/pkg/cve/matcher"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestVersionedCatalogMatchPostgres matches a Debian package against the versioned catalog: one
// finding per CVE listing every advisory that matched, rated from the best severity source.
func TestVersionedCatalogMatchPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()
	schema := fmt.Sprintf("vuln_match_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()

	require.NoError(t, db.AutoMigrate(&models.CatalogGeneration{}))
	require.NoError(t, migrations.Migration157_VulnCatalogVersioned(db))
	ctx := context.Background()
	now := time.Now()
	gen := &models.CatalogGeneration{CatalogType: "cve", SourceName: "osv", Status: "active", StartedAt: now, ActivatedAt: &now}
	require.NoError(t, db.Create(gen).Error)

	score := func(f float64) *float64 { return &f }
	cve := func(id, relation string) loader.AdvisoryRef {
		return loader.AdvisoryRef{RefID: id, Relation: relation, RefKind: "cve"}
	}
	openssl := func(fixed, vendor string) []loader.AffectedRange {
		return []loader.AffectedRange{{Ecosystem: "debian", Release: "12", PackageName: "openssl", RangeType: "ECOSYSTEM",
			Introduced: "0", Fixed: fixed, VendorSeverity: vendor}}
	}
	advisories := []*loader.ParsedAdvisory{
		// Debian's own entry, not yet rated.
		{ID: "DEBIAN-CVE-2024-0001", Source: "debian", Refs: []loader.AdvisoryRef{cve("CVE-2024-0001", "upstream")},
			Affected: openssl("3.0.5-1", "")},
		// A DSA fixing two CVEs: its 9.8 is the worst of the two.
		{ID: "DSA-5000-1", Source: "debian", CVSSv3Score: score(9.8), CVSSv3Vector: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			Refs: []loader.AdvisoryRef{cve("CVE-2024-0001", "upstream"), cve("CVE-2024-0002", "upstream")}, Affected: openssl("3.0.5-1", "")},
		// GitHub rates CVE-2024-0001 on its own (another ecosystem).
		{ID: "GHSA-aaaa-bbbb-cccc", Source: "ghsa", CVSSv3Score: score(5.3), Refs: []loader.AdvisoryRef{cve("CVE-2024-0001", "alias")},
			Affected: []loader.AffectedRange{{Ecosystem: "pypi", PackageName: "pyopenssl", RangeType: "ECOSYSTEM", Introduced: "0", Fixed: "1.0"}}},
		// Debian says unimportant, whatever the CVSS.
		{ID: "DEBIAN-CVE-2024-0003", Source: "debian", CVSSv3Score: score(9.8), Refs: []loader.AdvisoryRef{cve("CVE-2024-0003", "upstream")},
			Affected: openssl("", "unimportant")},
		// Fixed before the installed version.
		{ID: "DEBIAN-CVE-2024-0004", Source: "debian", Refs: []loader.AdvisoryRef{cve("CVE-2024-0004", "upstream")},
			Affected: openssl("3.0.0-1", "high")},
		// No CVE and no rating anywhere.
		{ID: "DEBIAN-TEMP-0005", Source: "debian", Affected: openssl("3.0.9-1", "")},
	}
	for _, a := range advisories {
		a.Kind, a.References, a.ContentSHA256 = "vulnerability", "[]", fmt.Sprintf("%064d", len(a.ID))
	}
	catalog := loader.NewVersionedCatalog(db, gen.ID)
	require.NotNil(t, catalog)
	_, err = catalog.Write(ctx, advisories)
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO vulnerabilities (vuln_id, nvd_cvss_v3_score, nvd_cvss_v3_vector) VALUES ('CVE-2024-0002', 7.5, 'CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:H')`).Error)

	sbom := &models.SBOM{ID: 1, Status: "complete", OSName: "debian", OSVersion: "12"}
	comp := &models.SBOMComponent{SBOMID: 1, ComponentName: "openssl", ComponentVersion: "3.0.1-1",
		PURL: "pkg:deb/debian/openssl@3.0.1-1?distro=debian-12", Ecosystem: "deb", TrustLevel: "high", SourceDetail: "agent-fields"}
	m := matcher.NewMatcher(database.NewPostgresManager(db), db)
	matches, complete, err := m.MatchSBOMComplete(ctx, sbom, []*models.SBOMComponent{comp})
	require.NoError(t, err)
	require.True(t, complete)

	type finding struct {
		Severity, Source, Fixed string
		Advisories              []string
	}
	got := map[string]finding{}
	for _, mt := range matches {
		require.Equal(t, "openssl", mt.PackageName)
		adv := append([]string(nil), mt.AdvisoryIDs...)
		sort.Strings(adv)
		got[mt.CVEID] = finding{mt.Severity, mt.SeveritySource, mt.FixedVersion, adv}
	}
	require.Equal(t, map[string]finding{
		"CVE-2024-0001":    {"MEDIUM", "cve_cvss", "3.0.5-1", []string{"DEBIAN-CVE-2024-0001", "DSA-5000-1"}},
		"CVE-2024-0002":    {"HIGH", "nvd", "3.0.5-1", []string{"DSA-5000-1"}},
		"CVE-2024-0003":    {"LOW", "vendor", "", []string{"DEBIAN-CVE-2024-0003"}},
		"DEBIAN-TEMP-0005": {"MEDIUM", "default", "3.0.9-1", nil},
	}, got)
}
