package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestSBOMListDuplicateUIDClusterIsolationPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()

	schema := fmt.Sprintf("sbom_list_scope_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")

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
	defer pool.Close()

	require.NoError(t, db.Exec(`
		CREATE TABLE pods (
			id BIGSERIAL PRIMARY KEY,
			cluster_id VARCHAR(255) NOT NULL,
			uid VARCHAR(255) NOT NULL,
			phase VARCHAR(64),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);
		CREATE TABLE sboms (
			id BIGSERIAL PRIMARY KEY,
			cluster_id VARCHAR(255) NOT NULL,
			pod_uid VARCHAR(255) NOT NULL,
			pod_name VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			image_name VARCHAR(255) NOT NULL,
			image_tag VARCHAR(255) NOT NULL,
			image_digest VARCHAR(255),
			container_name VARCHAR(255),
			generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			package_count INTEGER NOT NULL DEFAULT 0,
			sbom_source VARCHAR(255),
			confidence VARCHAR(64),
			go_version VARCHAR(255),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);
		CREATE TABLE cve_matches (
			id BIGSERIAL PRIMARY KEY,
			sbom_id BIGINT NOT NULL,
			severity VARCHAR(64),
			deleted_at TIMESTAMPTZ
		);
	`).Error)

	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, clusterID := range []string{"cluster-a", "cluster-b"} {
		require.NoError(t, db.Exec(
			"INSERT INTO pods(cluster_id, uid, phase, created_at) VALUES (?, ?, ?, ?)",
			clusterID, "same-pod", "Running", now,
		).Error)
		require.NoError(t, db.Exec(
			`INSERT INTO sboms(cluster_id, pod_uid, pod_name, namespace, image_name, image_tag, container_name, generated_at, package_count, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			clusterID, "same-pod", "same-name", "default", "image-"+clusterID, "1", "app", now, 1, now,
		).Error)
	}

	call := func(user *models.User, target string) (*httptest.ResponseRecorder, map[string]interface{}) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, target, nil)
		c.Set("user", user)
		GetSBOMList(db)(c)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		return w, body
	}

	gin.SetMode(gin.TestMode)
	restricted := &models.User{Role: models.RoleViewer, ScopeJSON: `{"cluster_ids":["cluster-a"]}`}
	w, body := call(restricted, "/api/v1/inventory/sbom?podName=same-name&namespace=default")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.EqualValues(t, 1, body["total"])
	rows := body["sboms"].([]interface{})
	require.Len(t, rows, 1)
	row := rows[0].(map[string]interface{})
	require.Equal(t, "cluster-a", row["clusterId"])
	require.Equal(t, "same-pod", row["podId"])

	unrestricted := &models.User{Role: models.RoleAdmin}
	w, body = call(unrestricted, "/api/v1/inventory/sbom?podName=same-name&namespace=default")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.EqualValues(t, 2, body["total"])
	rows = body["sboms"].([]interface{})
	require.Len(t, rows, 2)
	seen := map[string]bool{}
	for _, raw := range rows {
		seen[raw.(map[string]interface{})["clusterId"].(string)] = true
	}
	require.True(t, seen["cluster-a"])
	require.True(t, seen["cluster-b"])
}
