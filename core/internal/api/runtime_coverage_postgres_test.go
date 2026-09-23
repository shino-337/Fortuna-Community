package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func postCoveragePostgres(db *gorm.DB, principal agentidentity.Principal, body collection.RuntimeCoverage) (int, string, error) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("fortuna.agent.principal", principal)
		c.Next()
	})
	r.POST("/api/v2/runtime/coverage", PostRuntimeCoverage(db))
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, "", err
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/coverage", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String(), nil
}

func TestRuntimeCoveragePostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()

	schema := fmt.Sprintf("runtime_coverage_%d", time.Now().UnixNano())
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
	pool.SetMaxOpenConns(12)
	defer pool.Close()

	require.NoError(t, migrations.EnsureRuntimeCoverage(db))
	require.NoError(t, migrations.EnsureRuntimeCoverage(db))

	principal := agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	first := collection.RuntimeCoverage{
		Version: collection.RuntimeCoverageVersion,
		ID: "postgres-coverage-0001",
		ProducerID: "falco",
		SourceKind: collection.RuntimeSourceFalco,
		Status: "complete",
		WindowStart: now.Add(-4 * time.Second),
		WindowEnd: now.Add(-3 * time.Second),
	}

	// Exact concurrent first reports must collapse to one row; contenders are
	// idempotent replays rather than unique-key 500s.
	var wg sync.WaitGroup
	codes := make(chan int, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _, err := postCoveragePostgres(db, principal, first)
			codes <- code
			errs <- err
		}()
	}
	wg.Wait()
	close(codes)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	for code := range codes {
		require.Equal(t, http.StatusOK, code)
	}

	var count int64
	require.NoError(t, db.Model(&models.RuntimeCoverage{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	var accepted models.RuntimeCoverage
	require.NoError(t, db.First(&accepted, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, first.ID, accepted.CoverageID)
	require.NotNil(t, accepted.ContinuousSince)
	require.True(t, accepted.ContinuousSince.Equal(first.WindowStart))

	altered := first
	altered.Status = "failed"
	altered.Errors = 1
	altered.Reason = "changed replay"
	code, body, err := postCoveragePostgres(db, principal, altered)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, code, body)

	// A real SQL write failure must not advance the accepted window. Retrying the
	// exact next window after storage recovers must then succeed.
	require.NoError(t, db.Exec(`CREATE FUNCTION deny_runtime_coverage_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected runtime coverage failure'; END $$`).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER deny_runtime_coverage_update BEFORE UPDATE ON runtime_coverages FOR EACH ROW EXECUTE FUNCTION deny_runtime_coverage_update()`).Error)

	second := first
	second.ID = "postgres-coverage-0002"
	second.WindowStart = first.WindowEnd
	second.WindowEnd = now.Add(-2 * time.Second)
	code, _, err = postCoveragePostgres(db, principal, second)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, code)

	var afterFailure models.RuntimeCoverage
	require.NoError(t, db.First(&afterFailure, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, first.ID, afterFailure.CoverageID)
	require.True(t, afterFailure.WindowEnd.Equal(first.WindowEnd), "rollback changed accepted window: got=%v want=%v", afterFailure.WindowEnd, first.WindowEnd)

	require.NoError(t, db.Exec(`DROP TRIGGER deny_runtime_coverage_update ON runtime_coverages`).Error)
	code, body, err = postCoveragePostgres(db, principal, second)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, code, body)

	var recovered models.RuntimeCoverage
	require.NoError(t, db.First(&recovered, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, second.ID, recovered.CoverageID)
	require.NotNil(t, recovered.ContinuousSince)
	require.True(t, recovered.ContinuousSince.Equal(first.WindowStart))

	// Startup invariant reruns on populated data without mutating accepted evidence.
	require.NoError(t, migrations.EnsureRuntimeCoverage(db))
	var afterRestart models.RuntimeCoverage
	require.NoError(t, db.First(&afterRestart, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, recovered, afterRestart)
}
