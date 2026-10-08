package loader_test

import (
	"context"
	"crypto/sha256"
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

func TestVersionedCatalogPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()

	schema := fmt.Sprintf("vuln_catalog_%d", time.Now().UnixNano())
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
	// Only what PruneCVEGenerations touches of the legacy table.
	require.NoError(t, db.Exec(`CREATE TABLE package_vulnerabilities (id BIGSERIAL PRIMARY KEY, catalog_generation_id BIGINT, deleted_at TIMESTAMPTZ)`).Error)
	require.NoError(t, migrations.Migration157_VulnCatalogVersioned(db))
	ctx := context.Background()

	newGen := func(status string) uint {
		now := time.Now()
		g := &models.CatalogGeneration{CatalogType: "cve", SourceName: "osv", Status: status, StartedAt: now}
		if status == "active" {
			g.ActivatedAt = &now
		}
		require.NoError(t, db.Create(g).Error)
		return g.ID
	}
	activate := func(id uint) {
		now := time.Now()
		require.NoError(t, db.Model(&models.CatalogGeneration{}).Where("id = ?", id).
			Updates(map[string]interface{}{"status": "active", "activated_at": &now}).Error)
	}
	adv := func(id, content string, withdrawn bool) *loader.ParsedAdvisory {
		a := &loader.ParsedAdvisory{
			ID: id, Withdrawn: withdrawn, Source: "test", Kind: "vulnerability", Summary: content,
			References: "[]", ContentSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
			Refs:     []loader.AdvisoryRef{{RefID: "CVE-2024-" + id, Relation: "upstream", RefKind: "cve"}},
			Affected: []loader.AffectedRange{{Ecosystem: "debian", Release: "12", PackageName: "pkg-" + id, RangeType: "ECOSYSTEM", Introduced: "0", Fixed: content}},
		}
		return a
	}
	// asOf lists "id:summary" of the advisories generation g reads, and checks refs and
	// affected rows agree with them.
	asOf := func(g uint) []string {
		var rows []struct{ AdvisoryID, Summary string }
		require.NoError(t, db.Raw(`SELECT advisory_id, summary FROM vuln_advisories
			WHERE valid_from_gen <= ? AND (valid_to_gen IS NULL OR valid_to_gen > ?)`, g, g).Scan(&rows).Error)
		var out []string
		for _, r := range rows {
			out = append(out, r.AdvisoryID+":"+r.Summary)
		}
		sort.Strings(out)
		for _, table := range []string{"vuln_advisory_refs", "vuln_affected"} {
			var n int64
			require.NoError(t, db.Raw(`SELECT COUNT(*) FROM `+table+`
				WHERE valid_from_gen <= ? AND (valid_to_gen IS NULL OR valid_to_gen > ?)`, g, g).Scan(&n).Error)
			require.EqualValues(t, len(out), n, "%s rows as of generation %d", table, g)
		}
		return out
	}
	var current int64

	unlock, err := loader.LockCatalogLoad(ctx, db)
	require.NoError(t, err)
	_, err = loader.LockCatalogLoad(ctx, db)
	require.ErrorContains(t, err, "another vulnerability catalog load is running")
	unlock()
	unlock, err = loader.LockCatalogLoad(ctx, db)
	require.NoError(t, err)
	unlock()

	empty, err := loader.VersionedCatalogEmpty(ctx, db)
	require.NoError(t, err)
	require.True(t, empty)

	// Generation 1 loads A, B, C.
	g1 := newGen("running")
	c1 := loader.NewVersionedCatalog(db, g1)
	require.NotNil(t, c1)
	st, err := c1.Write(ctx, []*loader.ParsedAdvisory{adv("A", "a1", false), adv("B", "b1", false), adv("C", "c1", false)})
	require.NoError(t, err)
	require.Equal(t, loader.CatalogWriteStats{Written: 3}, st)
	activate(g1)
	require.Equal(t, []string{"A:a1", "B:b1", "C:c1"}, asOf(g1))

	// Generation 2 changes A, keeps B, withdraws C, adds D; only the changes are written.
	g2 := newGen("running")
	c2 := loader.NewVersionedCatalog(db, g2)
	writeG2 := func() {
		st, err := c2.Write(ctx, []*loader.ParsedAdvisory{adv("A", "a2", false), adv("B", "b1", false), adv("C", "c1", true), adv("D", "d1", false)})
		require.NoError(t, err)
		require.Equal(t, loader.CatalogWriteStats{Written: 2, Unchanged: 1, Closed: 2}, st)
	}
	writeG2()
	require.Equal(t, []string{"A:a1", "B:b1", "C:c1"}, asOf(g1), "the active generation must not see a running load")
	require.Equal(t, []string{"A:a2", "B:b1", "D:d1"}, asOf(g2))

	// A failed load is rolled back to exactly generation 1.
	require.NoError(t, loader.RollbackCatalogGeneration(ctx, db, g2))
	require.Equal(t, []string{"A:a1", "B:b1", "C:c1"}, asOf(g2))
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM vuln_advisories WHERE valid_from_gen = ? OR valid_to_gen = ?`, g2, g2).Scan(&current).Error)
	require.Zero(t, current)

	// Retry, then the same advisory loaded twice in one generation keeps one version.
	writeG2()
	st, err = c2.Write(ctx, []*loader.ParsedAdvisory{adv("D", "d2", false)})
	require.NoError(t, err)
	require.Equal(t, 1, st.Written)
	require.Equal(t, []string{"A:a2", "B:b1", "D:d2"}, asOf(g2))
	activate(g2)

	// Generation 3 is a full load that no longer has B.
	g3 := newGen("running")
	c3 := loader.NewVersionedCatalog(db, g3)
	st, err = c3.Write(ctx, []*loader.ParsedAdvisory{adv("A", "a2", false), adv("D", "d2", false)})
	require.NoError(t, err)
	require.Equal(t, 2, st.Unchanged)
	closed, err := c3.CloseMissing(ctx, []string{"A", "D"})
	require.NoError(t, err)
	require.Equal(t, 1, closed)
	activate(g3)
	require.Equal(t, []string{"A:a2", "D:d2"}, asOf(g3))
	require.Equal(t, []string{"A:a2", "B:b1", "D:d2"}, asOf(g2))

	// Keeping two generations (3 and 2) drops the versions only generation 1 could read.
	retired, _, err := loader.PruneCVEGenerations(ctx, db, 2)
	require.NoError(t, err)
	require.Equal(t, []uint{g1}, retired)
	require.Equal(t, []string{"A:a2", "B:b1", "D:d2"}, asOf(g2))
	require.Equal(t, []string{"A:a2", "D:d2"}, asOf(g3))
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM vuln_advisories`).Scan(&current).Error)
	require.EqualValues(t, 3, current, "A:a1 and C:c1 are gone; B:b1 stays for generation 2")

	// A load killed mid-way is undone before the next one starts.
	g4 := newGen("running")
	c4 := loader.NewVersionedCatalog(db, g4)
	_, err = c4.Write(ctx, []*loader.ParsedAdvisory{adv("A", "a3", false), adv("E", "e1", false)})
	require.NoError(t, err)
	_, err = c4.Close(ctx, []string{"D"})
	require.NoError(t, err)
	require.NoError(t, loader.RollbackUnfinishedCatalogGenerations(ctx, db))
	require.Equal(t, []string{"A:a2", "D:d2"}, asOf(g4))
	var status string
	require.NoError(t, db.Raw(`SELECT status FROM catalog_generations WHERE id = ?`, g4).Scan(&status).Error)
	require.Equal(t, "failed", status)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM vuln_advisories WHERE valid_to_gen IS NULL`).Scan(&current).Error)
	require.EqualValues(t, 2, current)
}
