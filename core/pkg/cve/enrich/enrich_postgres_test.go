package enrich_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/cve/enrich"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestEnrichPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()
	schema := fmt.Sprintf("vuln_enrich_%d", time.Now().UnixNano())
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
	require.NoError(t, migrations.Migration157_VulnCatalogVersioned(db))
	ctx := context.Background()

	kev := `{"vulnerabilities":[{"cveID":"CVE-2024-3094","dateAdded":"2024-04-01","dueDate":"2024-04-22","knownRansomwareCampaignUse":"Known"},{"cveID":"CVE-2021-44228","dateAdded":"2021-12-10"}]}`
	epss := "#model_version:v1,score_date:2026-10-08T00:00:00Z\ncve,epss,percentile\nCVE-2024-3094,0.84,0.99\nCVE-2023-0001,0.01,0.2\n"
	var kevHits, epssHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kev.json":
			kevHits.Add(1)
			etag := fmt.Sprintf(`"kev-%d"`, len(kev))
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", etag)
			_, _ = w.Write([]byte(kev))
		case "/epss.csv":
			epssHits.Add(1)
			_, _ = w.Write([]byte(epss))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	type row struct {
		VulnID         string
		KEVAddedAt     *time.Time
		KEVRansomware  *bool
		EPSSScore      *float64
		EPSSPercentile *float64
	}
	load := func() map[string]row {
		var rows []row
		require.NoError(t, db.Raw(`SELECT vuln_id, kev_added_at, kev_ransomware, epss_score::float8 AS epss_score, epss_percentile::float8 AS epss_percentile FROM vulnerabilities`).Scan(&rows).Error)
		out := map[string]row{}
		for _, r := range rows {
			out[r.VulnID] = r
		}
		return out
	}

	results := enrich.Run(ctx, db, srv.Client(), srv.URL+"/kev.json", srv.URL+"/epss.csv")
	require.Len(t, results, 2)
	for _, r := range results {
		require.NoError(t, r.Err, r.Feed)
	}
	got := load()
	require.Len(t, got, 3)
	require.NotNil(t, got["CVE-2024-3094"].KEVAddedAt)
	require.True(t, *got["CVE-2024-3094"].KEVRansomware)
	require.InDelta(t, 0.84, *got["CVE-2024-3094"].EPSSScore, 1e-6)
	require.NotNil(t, got["CVE-2021-44228"].KEVAddedAt)
	require.Nil(t, got["CVE-2021-44228"].EPSSScore)
	require.Nil(t, got["CVE-2023-0001"].KEVAddedAt)

	// Same ETag: KEV is not stored again.
	results = enrich.Run(ctx, db, srv.Client(), srv.URL+"/kev.json", "")
	require.Len(t, results, 1)
	require.True(t, results[0].Skipped)

	// A CVE removed from KEV loses its KEV data but keeps its EPSS score.
	kev = `{"vulnerabilities":[{"cveID":"CVE-2021-44228","dateAdded":"2021-12-10"}]}`
	results = enrich.Run(ctx, db, srv.Client(), srv.URL+"/kev.json", "")
	require.NoError(t, results[0].Err)
	got = load()
	require.Nil(t, got["CVE-2024-3094"].KEVAddedAt)
	require.InDelta(t, 0.84, *got["CVE-2024-3094"].EPSSScore, 1e-6)
	require.NotNil(t, got["CVE-2021-44228"].KEVAddedAt)

	// A broken feed is recorded and leaves the stored data alone.
	results = enrich.Run(ctx, db, srv.Client(), srv.URL+"/missing", "")
	require.Error(t, results[0].Err)
	var state struct {
		LastError     string
		LastSuccessAt *time.Time
	}
	require.NoError(t, db.Raw(`SELECT last_error, last_success_at FROM vuln_feed_state WHERE feed = 'kev'`).Scan(&state).Error)
	require.Contains(t, state.LastError, "404")
	require.NotNil(t, state.LastSuccessAt)
	require.NotNil(t, load()["CVE-2021-44228"].KEVAddedAt)
}
