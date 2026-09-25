package api

import (
	"encoding/json"
	"fmt"
	"net/http"\n\t"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func sbomListPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminSQL.Close() })

	schema := fmt.Sprintf("sbom_list_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error })

	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = pool.Close() })

	require.NoError(t, db.Exec(`
		CREATE TABLE pods (
			cluster_id text NOT NULL,
			uid text NOT NULL,
			created_at timestamptz NOT NULL,
			phase text,
			deleted_at timestamptz
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE sboms (
			id bigserial PRIMARY KEY,
			cluster_id text NOT NULL,
			pod_uid text NOT NULL,
			pod_name text,
			namespace text,
			image_name text NOT NULL,
			image_tag text NOT NULL,
			image_digest text NOT NULL DEFAULT '',
			container_name text,
			generated_at timestamptz NOT NULL,
			package_count integer NOT NULL DEFAULT 0,
			sbom_source text,
			confidence text,
			go_version text,
			created_at timestamptz NOT NULL,
			deleted_at timestamptz
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE cve_matches (
			id bigserial PRIMARY KEY,
			sbom_id bigint NOT NULL,
			severity text NOT NULL,
			deleted_at timestamptz
		)
	`).Error)
	return db
}

func seedSBOMListRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, clusterID := range []string{"cluster-a", "cluster-b"} {
		require.NoError(t, db.Exec(
			"INSERT INTO pods(cluster_id, uid, created_at, phase) VALUES (?, ?, ?, ?)",
			clusterID, "same-pod", now, "Running",
		).Error)
		require.NoError(t, db.Exec(
			`INSERT INTO sboms(cluster_id, pod_uid, pod_name, namespace, image_name, image_tag, image_digest, generated_at, package_count, created_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			clusterID, "same-pod", "demo", "default", "example.invalid/app", "latest", "sha256:"+clusterID, now, 1, now,
		).Error)
	}
}

func sbomListContext(target string, user *models.User) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	c.Set("user", user)
	return c, w
}

func TestSBOMListClusterScopePostgres(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := sbomListPostgresDB(t)
	seedSBOMListRows(t, db)

	c, w := sbomListContext("/api/v1/inventory/sbom", &models.User{
		Role:      models.RoleViewer,
		ScopeJSON: `{"cluster_ids":["cluster-a"]}`,
	})
	GetSBOMList(db)(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body struct {
		SBOMs []SBOMSummaryDTO `json:"sboms"`
		Total int64            `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.EqualValues(t, 1, body.Total)
	require.Len(t, body.SBOMs, 1)
	require.Equal(t, "cluster-a", body.SBOMs[0].ClusterID)
	require.Equal(t, "same-pod", body.SBOMs[0].PodID)
	require.True(t, body.SBOMs[0].ActivePod)
}

func TestSBOMListFailClosedPostgres(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("pod metadata query failure", func(t *testing.T) {
		db := sbomListPostgresDB(t)
		seedSBOMListRows(t, db)
		require.NoError(t, db.Exec("ALTER TABLE pods RENAME TO pods_unavailable").Error)

		c, w := sbomListContext("/api/v1/inventory/sbom?includeStale=true", &models.User{Role: models.RoleAdmin})
		GetSBOMList(db)(c)
		require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, "sbom_list_pod_metadata_unavailable", body["code"])
		require.Equal(t, true, body["retryable"])
	})

	t.Run("CVE aggregation query failure", func(t *testing.T) {
		db := sbomListPostgresDB(t)
		seedSBOMListRows(t, db)
		require.NoError(t, db.Exec("ALTER TABLE cve_matches RENAME TO cve_matches_unavailable").Error)

		c, w := sbomListContext("/api/v1/inventory/sbom", &models.User{Role: models.RoleAdmin})
		GetSBOMList(db)(c)
		require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, "sbom_list_cve_summary_unavailable", body["code"])
		require.Equal(t, true, body["retryable"])
	})
}
