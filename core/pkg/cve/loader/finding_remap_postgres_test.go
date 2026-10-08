package loader_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/cve/loader"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRemapFindingsToCanonicalPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()
	schema := fmt.Sprintf("vuln_remap_%d", time.Now().UnixNano())
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

	require.NoError(t, db.AutoMigrate(&models.CatalogGeneration{}, &models.Insight{}, &models.ExceptionPolicy{}))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX idx_insight_resource_identity ON insights (cluster_id, resource_uid, cve_id, insight_type)`).Error)
	require.NoError(t, migrations.Migration157_VulnCatalogVersioned(db))
	ctx := context.Background()
	gen := &models.CatalogGeneration{CatalogType: "cve", SourceName: "osv", Status: "active", StartedAt: time.Now()}
	require.NoError(t, db.Create(gen).Error)

	cve := func(id, relation string) loader.AdvisoryRef {
		return loader.AdvisoryRef{RefID: id, Relation: relation, RefKind: "cve"}
	}
	advisories := []*loader.ParsedAdvisory{
		{ID: "GHSA-aaaa-bbbb-cccc", Source: "ghsa", Refs: []loader.AdvisoryRef{cve("CVE-2024-0001", "alias")}},
		{ID: "PYSEC-2024-1", Source: "pypa", Refs: []loader.AdvisoryRef{cve("CVE-2024-0001", "alias")}},
		{ID: "RHSA-2024:0001", Source: "redhat", Refs: []loader.AdvisoryRef{cve("CVE-2024-0002", "upstream"), cve("CVE-2024-0003", "upstream")}},
		{ID: "GHSA-only-only-only", Source: "ghsa"},
	}
	for _, a := range advisories {
		a.Kind, a.References, a.ContentSHA256 = "vulnerability", "[]", fmt.Sprintf("%064d", len(a.ID))
	}
	_, err = loader.NewVersionedCatalog(db, gen.ID).Write(ctx, advisories)
	require.NoError(t, err)

	insight := func(uid, cveID, status, assignee string) {
		require.NoError(t, db.Create(&models.Insight{ClusterID: "c1", ResourceType: "Pod", ResourceName: uid, ResourceUID: uid,
			InsightType: "vulnerability", Severity: "high", Title: cveID + " in pkg", Description: "d", Status: status,
			Evidence: "{}", ViolatedRules: "[]", Remediation: "{}",
			CVEID: cveID, AssigneeUsername: assignee, DetectedAt: time.Now()}).Error)
	}
	// pod-a: two advisories of one CVE; the dismissed one wins the rename.
	insight("pod-a", "GHSA-aaaa-bbbb-cccc", "active", "")
	insight("pod-a", "PYSEC-2024-1", "dismissed", "alice")
	// pod-b: the CVE finding exists already; the advisory one brings its assignee.
	insight("pod-b", "CVE-2024-0001", "active", "")
	insight("pod-b", "GHSA-aaaa-bbbb-cccc", "active", "bob")
	// pod-c: an erratum splits into its CVEs.
	insight("pod-c", "RHSA-2024:0001", "dismissed", "")
	// pod-d: no CVE, stays.
	insight("pod-d", "GHSA-only-only-only", "active", "")
	require.NoError(t, db.Create(&models.ExceptionPolicy{ClusterID: "c1", ResourceUID: "pod-c", CVEID: "RHSA-2024:0001",
		InsightType: "vulnerability", Reason: "accepted"}).Error)

	moved, err := loader.RemapFindingsToCanonical(ctx, db)
	require.NoError(t, err)
	require.Equal(t, 3, moved)

	live := func() []string {
		var rows []models.Insight
		require.NoError(t, db.Order("resource_uid, cve_id").Find(&rows).Error)
		var out []string
		for _, r := range rows {
			out = append(out, fmt.Sprintf("%s %s %s %s [%s]", r.ResourceUID, r.CVEID, r.Status, r.AssigneeUsername, r.Title))
		}
		return out
	}
	want := []string{
		"pod-a CVE-2024-0001 dismissed alice [CVE-2024-0001 in pkg]",
		"pod-b CVE-2024-0001 active bob [CVE-2024-0001 in pkg]",
		"pod-c CVE-2024-0002 dismissed  [CVE-2024-0002 in pkg]",
		"pod-c CVE-2024-0003 dismissed  [CVE-2024-0003 in pkg]",
		"pod-d GHSA-only-only-only active  [GHSA-only-only-only in pkg]",
	}
	require.Equal(t, want, live())

	var exceptions []string
	require.NoError(t, db.Model(&models.ExceptionPolicy{}).Pluck("cve_id", &exceptions).Error)
	sort.Strings(exceptions)
	require.Equal(t, []string{"CVE-2024-0002", "CVE-2024-0003"}, exceptions)

	// Running again changes nothing.
	moved, err = loader.RemapFindingsToCanonical(ctx, db)
	require.NoError(t, err)
	require.Zero(t, moved)
	require.Equal(t, want, live())
}
