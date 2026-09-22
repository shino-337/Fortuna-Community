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

func runtimeCoverageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.RuntimeCoverage{}))
	return db
}

func runtimeCoverageRouter(db *gorm.DB, scoped bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if scoped {
		r.Use(func(c *gin.Context) {
			c.Set("fortuna.agent.principal", agentidentity.Principal{
				CredentialID: "cred-a", ClusterID: "cluster-a", AgentID: "agent-a",
			})
			c.Next()
		})
	}
	r.POST("/coverage", PostRuntimeCoverage(db))
	return r
}

func postRuntimeCoverage(t *testing.T, r http.Handler, c collection.RuntimeCoverage) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/coverage", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func coverageFixture(id, status string, start, end time.Time) collection.RuntimeCoverage {
	c := collection.RuntimeCoverage{
		Version: collection.RuntimeCoverageVersion,
		ID: id, ProducerID: "runtime-file", SourceKind: "file",
		Status: status, WindowStart: start, WindowEnd: end,
	}
	if status == "failed" {
		c.Emitted = 1
		c.Delivered = 0
		c.Dropped = 1
		c.Reason = "delivery_failed"
	}
	return c
}

func TestRuntimeCoverageRequiresScopedAgentIdentity(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	w := postRuntimeCoverage(t, runtimeCoverageRouter(runtimeCoverageTestDB(t), false),
		coverageFixture("coverage-00000001", "complete", now.Add(-time.Minute), now))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRuntimeCoverageContinuityGapAndReplay(t *testing.T) {
	db := runtimeCoverageTestDB(t)
	r := runtimeCoverageRouter(db, true)
	now := time.Now().UTC().Truncate(time.Microsecond)

	first := coverageFixture("coverage-00000001", "complete", now.Add(-4*time.Minute), now.Add(-3*time.Minute))
	require.Equal(t, http.StatusOK, postRuntimeCoverage(t, r, first).Code)
	var row models.RuntimeCoverage
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "runtime-file").Error)
	require.NotNil(t, row.ContinuousSince)
	require.True(t, row.ContinuousSince.Equal(first.WindowStart))

	adjacent := coverageFixture("coverage-00000002", "complete", first.WindowEnd, now.Add(-2*time.Minute))
	require.Equal(t, http.StatusOK, postRuntimeCoverage(t, r, adjacent).Code)
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "runtime-file").Error)
	require.NotNil(t, row.ContinuousSince)
	require.True(t, row.ContinuousSince.Equal(first.WindowStart), "adjacent complete windows must preserve continuity")

	// Exact replay is idempotent.
	replay := postRuntimeCoverage(t, r, adjacent)
	require.Equal(t, http.StatusOK, replay.Code)
	require.Contains(t, replay.Body.String(), `"replay":true`)

	// Same coverage ID with altered content is rejected.
	altered := adjacent
	altered.Reason = "changed"
	require.Equal(t, http.StatusConflict, postRuntimeCoverage(t, r, altered).Code)

	// A positive time gap resets the continuous watermark.
	gapped := coverageFixture("coverage-00000003", "complete", now.Add(-90*time.Second), now.Add(-60*time.Second))
	require.Equal(t, http.StatusOK, postRuntimeCoverage(t, r, gapped).Code)
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "runtime-file").Error)
	require.NotNil(t, row.ContinuousSince)
	require.True(t, row.ContinuousSince.Equal(gapped.WindowStart))
	require.False(t, row.CoversSince(now.Add(-2*time.Minute), now))
	require.True(t, row.CoversSince(now.Add(-75*time.Second), now))
}

func TestRuntimeCoverageFailureCannotBeErasedByOverlappingRecovery(t *testing.T) {
	db := runtimeCoverageTestDB(t)
	r := runtimeCoverageRouter(db, true)
	now := time.Now().UTC().Truncate(time.Microsecond)

	first := coverageFixture("coverage-00000011", "complete", now.Add(-5*time.Minute), now.Add(-4*time.Minute))
	require.Equal(t, http.StatusOK, postRuntimeCoverage(t, r, first).Code)

	failed := coverageFixture("coverage-00000012", "failed", first.WindowEnd, now.Add(-3*time.Minute))
	require.Equal(t, http.StatusOK, postRuntimeCoverage(t, r, failed).Code)
	var row models.RuntimeCoverage
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "runtime-file").Error)
	require.Equal(t, "failed", row.Status)
	require.Nil(t, row.ContinuousSince)

	// A successful window that backdates into the failed span cannot claim that
	// span. Core clamps continuity to the end of the known failure.
	recovery := coverageFixture("coverage-00000013", "complete", now.Add(-210*time.Second), now.Add(-2*time.Minute))
	require.Equal(t, http.StatusOK, postRuntimeCoverage(t, r, recovery).Code)
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "runtime-file").Error)
	require.Equal(t, "complete", row.Status)
	require.NotNil(t, row.ContinuousSince)
	require.True(t, row.ContinuousSince.Equal(failed.WindowEnd))
	require.False(t, row.CoversSince(now.Add(-190*time.Second), now), "known failed interval must remain outside continuous coverage")
	require.True(t, row.CoversSince(now.Add(-150*time.Second), now))
}

func TestRuntimeCoverageRejectsLossAsComplete(t *testing.T) {
	db := runtimeCoverageTestDB(t)
	r := runtimeCoverageRouter(db, true)
	now := time.Now().UTC().Truncate(time.Microsecond)
	c := coverageFixture("coverage-00000021", "complete", now.Add(-time.Minute), now)
	c.Emitted = 2
	c.Delivered = 1
	c.Dropped = 1
	require.Equal(t, http.StatusBadRequest, postRuntimeCoverage(t, r, c).Code)
	var count int64
	require.NoError(t, db.Model(&models.RuntimeCoverage{}).Count(&count).Error)
	require.Zero(t, count)
}
