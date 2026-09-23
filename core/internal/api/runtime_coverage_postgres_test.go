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
		SessionID: "session-postgres-000001",
		Status: "complete",
		WindowStart: now.Add(-4 * time.Second),
		WindowEnd: now.Add(-3 * time.Second),
	}

	manifest := runtimeManifest(first.SessionID, now.Add(-5*time.Second), now.Add(-4500*time.Millisecond), true)
	require.Equal(t, http.StatusOK, postRuntimeManifest(t, db, principal, manifest).Code)

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
	require.Nil(t, accepted.ContinuousSince, "protocol-v1 Falco is non-authoritative and must not establish absence continuity")
	var producer models.RuntimeProducerState
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerActive, producer.State)
	require.False(t, producer.Authoritative)
	require.Equal(t, "source_health_unverified", producer.GapReason)
	require.Equal(t, collection.RuntimeProducerNonAuthoritative, accepted.EffectiveStatus(&producer, now))
	require.False(t, accepted.CoversInterval(&producer, first.WindowStart, first.WindowEnd, now))
	require.Equal(t, first.ID, producer.LastCoverageID)

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
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, first.ID, producer.LastCoverageID, "coverage failure advanced lifecycle state")

	require.NoError(t, db.Exec(`DROP TRIGGER deny_runtime_coverage_update ON runtime_coverages`).Error)
	code, body, err = postCoveragePostgres(db, principal, second)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, code, body)

	var recovered models.RuntimeCoverage
	require.NoError(t, db.First(&recovered, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, second.ID, recovered.CoverageID)
	require.Nil(t, recovered.ContinuousSince, "recovery must not create continuity without upstream source-health authority")
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, second.ID, producer.LastCoverageID)
	require.Equal(t, collection.RuntimeProducerActive, producer.State)
	require.False(t, producer.Authoritative)
	require.Equal(t, collection.RuntimeProducerNonAuthoritative, recovered.EffectiveStatus(&producer, now))
	require.False(t, recovered.CoversInterval(&producer, first.WindowStart, second.WindowEnd, now))

	// A new Agent session must invalidate otherwise-fresh prior coverage and create
	// a persisted restart gap before any new complete window is accepted.
	session2 := "session-postgres-000002"
	manifest2 := runtimeManifest(session2, now.Add(-time.Second), now, true)
	require.Equal(t, http.StatusOK, postRuntimeManifest(t, db, principal, manifest2).Code)
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, session2, producer.SessionID)
	require.Equal(t, collection.RuntimeProducerStarting, producer.State)
	require.Equal(t, "agent_restart", producer.GapReason)
	require.Equal(t, "unknown", recovered.EffectiveStatus(&producer, now))

	third := second
	third.ID = "postgres-coverage-0003"
	third.SessionID = session2
	third.WindowStart = now.Add(-500 * time.Millisecond)
	third.WindowEnd = now.Add(-100 * time.Millisecond)
	code, body, err = postCoveragePostgres(db, principal, third)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, code, body)
	var afterNewSession models.RuntimeCoverage
	require.NoError(t, db.First(&afterNewSession, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Nil(t, afterNewSession.ContinuousSince,
		"new session must remain non-authoritative and cannot establish absence continuity")
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerActive, producer.State)
	require.False(t, producer.Authoritative)
	require.Equal(t, collection.RuntimeProducerNonAuthoritative, afterNewSession.EffectiveStatus(&producer, now))
	require.False(t, afterNewSession.CoversInterval(&producer, third.WindowStart, third.WindowEnd, now))

	// Startup invariant reruns on populated lifecycle/evidence data without mutation.
	require.NoError(t, migrations.EnsureRuntimeCoverage(db))
	var afterRestart models.RuntimeCoverage
	require.NoError(t, db.First(&afterRestart, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, afterNewSession, afterRestart)
	var producerAfterMigration models.RuntimeProducerState
	require.NoError(t, db.First(&producerAfterMigration, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, session2, producerAfterMigration.SessionID)
	require.Equal(t, collection.RuntimeProducerActive, producerAfterMigration.State)
}
