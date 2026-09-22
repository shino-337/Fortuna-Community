package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func runtimeCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.RuntimeCoverage{}))
	return db
}

func postCoverage(t *testing.T, db *gorm.DB, principal *agentidentity.Principal, body collection.RuntimeCoverage) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if principal != nil {
		r.Use(func(c *gin.Context) {
			c.Set("fortuna.agent.principal", *principal)
			c.Next()
		})
	}
	r.POST("/api/v2/runtime/coverage", PostRuntimeCoverage(db))
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/coverage", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func coverageWindow(id string, start, end time.Time) collection.RuntimeCoverage {
	return collection.RuntimeCoverage{
		Version: collection.RuntimeCoverageVersion,
		ID: id,
		ProducerID: "falco",
		SourceKind: collection.RuntimeSourceFalco,
		Status: "complete",
		WindowStart: start,
		WindowEnd: end,
	}
}

func TestRuntimeCoverageScopedContinuityAndReplay(t *testing.T) {
	db := runtimeCoverageDB(t)
	principal := &agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	first := coverageWindow("coverage-000000000001", now.Add(-3*time.Second), now.Add(-2*time.Second))

	w := postCoverage(t, db, principal, first)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var row models.RuntimeCoverage
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.NotNil(t, row.ContinuousSince)
	require.True(t, row.ContinuousSince.Equal(first.WindowStart))

	// Exact replay is idempotent.
	w = postCoverage(t, db, principal, first)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"replay":true`)

	// Same ID with changed payload is a conflict.
	changed := first
	changed.Errors = 1
	changed.Status = "failed"
	changed.Reason = "changed"
	w = postCoverage(t, db, principal, changed)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	// Adjacent clean window preserves continuity.
	second := coverageWindow("coverage-000000000002", first.WindowEnd, now.Add(-time.Second))
	w = postCoverage(t, db, principal, second)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.NotNil(t, row.ContinuousSince)
	require.True(t, row.ContinuousSince.Equal(first.WindowStart))

	// Failure breaks continuity.
	failed := coverageWindow("coverage-000000000003", second.WindowEnd, now)
	failed.Status = "failed"
	failed.Emitted = 1
	failed.Errors = 1
	failed.Reason = "delivery failed"
	w = postCoverage(t, db, principal, failed)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Nil(t, row.ContinuousSince)
	require.Equal(t, "failed", row.EffectiveStatus(time.Now().UTC()))

	// Later success starts a new continuity interval.
	recovered := coverageWindow("coverage-000000000004", failed.WindowEnd, now.Add(time.Second))
	w = postCoverage(t, db, principal, recovered)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.NotNil(t, row.ContinuousSince)
	require.True(t, row.ContinuousSince.Equal(recovered.WindowStart))
}

func TestRuntimeCoverageRejectsUnsafeWindows(t *testing.T) {
	principal := &agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)

	t.Run("identity-required", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		w := postCoverage(t, db, nil, coverageWindow("coverage-000000000010", now.Add(-time.Second), now))
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("complete-with-error", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		c := coverageWindow("coverage-000000000011", now.Add(-time.Second), now)
		c.Errors = 1
		w := postCoverage(t, db, principal, c)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("overlap", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		first := coverageWindow("coverage-000000000012", now.Add(-4*time.Second), now.Add(-2*time.Second))
		require.Equal(t, http.StatusOK, postCoverage(t, db, principal, first).Code)
		overlap := coverageWindow("coverage-000000000013", now.Add(-3*time.Second), now.Add(-time.Second))
		w := postCoverage(t, db, principal, overlap)
		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("source-kind-rebind", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		first := coverageWindow("coverage-000000000014", now.Add(-4*time.Second), now.Add(-2*time.Second))
		require.Equal(t, http.StatusOK, postCoverage(t, db, principal, first).Code)
		next := coverageWindow("coverage-000000000015", first.WindowEnd, now.Add(-time.Second))
		next.SourceKind = collection.RuntimeSourceFile
		w := postCoverage(t, db, principal, next)
		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("gap-restarts-continuity", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		first := coverageWindow("coverage-000000000016", now.Add(-5*time.Second), now.Add(-4*time.Second))
		require.Equal(t, http.StatusOK, postCoverage(t, db, principal, first).Code)
		gap := coverageWindow("coverage-000000000017", now.Add(-2*time.Second), now.Add(-time.Second))
		require.Equal(t, http.StatusOK, postCoverage(t, db, principal, gap).Code)
		var row models.RuntimeCoverage
		require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
		require.NotNil(t, row.ContinuousSince)
		require.True(t, row.ContinuousSince.Equal(gap.WindowStart))
	})
}
