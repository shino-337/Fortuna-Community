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
	require.NoError(t, db.AutoMigrate(&models.RuntimeCoverage{}, &models.RuntimeCoverageReceipt{}, &models.RuntimeProducerState{}, &models.RuntimeSourceHealthReceipt{}))
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

const runtimeTestSession = "session-000000000001"

func coverageWindow(id string, start, end time.Time) collection.RuntimeCoverage {
	return collection.RuntimeCoverage{
		Version: collection.RuntimeCoverageVersion,
		ID: id,
		ProducerID: "falco",
		SourceKind: collection.RuntimeSourceFalco,
		SessionID: runtimeTestSession,
		Status: "complete",
		WindowStart: start,
		WindowEnd: end,
	}
}


func seedRuntimeProducer(t *testing.T, db *gorm.DB, principal *agentidentity.Principal, producerID, sourceKind, sessionID string, sessionStartedAt, heartbeat time.Time, enabled, authoritative bool, state string) models.RuntimeProducerState {
	t.Helper()
	row := models.RuntimeProducerState{
		ClusterID: principal.ClusterID,
		AgentID: principal.AgentID,
		ProducerID: producerID,
		SourceKind: sourceKind,
		SessionID: sessionID,
		SessionStartedAt: sessionStartedAt,
		Enabled: enabled,
		Authoritative: authoritative,
		State: state,
		LastManifestAt: heartbeat,
		LastHeartbeatAt: heartbeat,
	}
	gap := sessionStartedAt
	if state != collection.RuntimeProducerActive {
		row.GapSince = &gap
		row.GapReason = "test"
	}
	require.NoError(t, db.Create(&row).Error)
	return row
}

func TestRuntimeCoverageScopedContinuityAndReplay(t *testing.T) {
	db := runtimeCoverageDB(t)
	principal := &agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	first := coverageWindow("coverage-000000000001", now.Add(-3*time.Second), now.Add(-2*time.Second))
	seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, runtimeTestSession, now.Add(-4*time.Second), now, true, true, collection.RuntimeProducerStarting)

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
	var failedRow models.RuntimeCoverage
	require.NoError(t, db.First(&failedRow, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Nil(t, failedRow.ContinuousSince)
	var failedProducer models.RuntimeProducerState
	require.NoError(t, db.First(&failedProducer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerDegraded, failedRow.EffectiveStatus(&failedProducer, time.Now().UTC()))

	// Later success starts a new continuity interval.
	recovered := coverageWindow("coverage-000000000004", failed.WindowEnd, now.Add(time.Second))
	w = postCoverage(t, db, principal, recovered)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var recoveredRow models.RuntimeCoverage
	require.NoError(t, db.First(&recoveredRow, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.NotNil(t, recoveredRow.ContinuousSince)
	require.True(t, recoveredRow.ContinuousSince.Equal(recovered.WindowStart))
}


func TestFailedCoverageGapStartsAtLastAcceptedCoverageEnd(t *testing.T) {
	db := runtimeCoverageDB(t)
	principal := &agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)

	first := coverageWindow("gap-anchor-coverage-0001", now.Add(-10*time.Second), now.Add(-8*time.Second))
	seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, runtimeTestSession, now.Add(-12*time.Second), now, true, true, collection.RuntimeProducerStarting)
	require.Equal(t, http.StatusOK, postCoverage(t, db, principal, first).Code)

	// Leave an explicit silent interval before the failed window.
	failed := coverageWindow("gap-anchor-coverage-0002", now.Add(-4*time.Second), now.Add(-2*time.Second))
	failed.Status = "failed"
	failed.Emitted = 1
	failed.Delivered = 0
	failed.Errors = 1
	failed.Reason = "delivery failed"
	require.Equal(t, http.StatusOK, postCoverage(t, db, principal, failed).Code)

	var producer models.RuntimeProducerState
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.NotNil(t, producer.GapSince)
	require.True(t, producer.GapSince.Equal(first.WindowEnd),
		"failed window gap must include the silent interval since last accepted coverage")
	require.Equal(t, "coverage_failed", producer.GapReason)
}

func TestRuntimeCoverageRejectsUnsafeWindows(t *testing.T) {
	principal := &agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)

	t.Run("identity-required", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		w := postCoverage(t, db, nil, coverageWindow("coverage-000000000010", now.Add(-time.Second), now))
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("historical-window-accepted-but-stale", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		oldStart := now.Add(-2 * time.Hour)
		oldEnd := oldStart.Add(time.Minute)
		c := coverageWindow("coverage-000000000099", oldStart, oldEnd)
		seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, runtimeTestSession, oldStart.Add(-time.Minute), now, true, true, collection.RuntimeProducerStarting)
		w := postCoverage(t, db, principal, c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var row models.RuntimeCoverage
		require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
		var producer models.RuntimeProducerState
		require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
		require.Equal(t, "stale", row.EffectiveStatus(&producer, now))
	})

	t.Run("producer-source-mismatch", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		c := coverageWindow("coverage-000000000098", now.Add(-time.Second), now)
		c.ProducerID = "runtime-file"
		w := postCoverage(t, db, principal, c)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("zero-duration", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		c := coverageWindow("coverage-000000000097", now, now)
		w := postCoverage(t, db, principal, c)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("lifecycle-required", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		c := coverageWindow("coverage-000000000090", now.Add(-time.Second), now)
		w := postCoverage(t, db, principal, c)
		require.Equal(t, http.StatusConflict, w.Code)
		require.Contains(t, w.Body.String(), "runtime_lifecycle_required")
	})

	t.Run("disabled-producer", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		c := coverageWindow("coverage-000000000091", now.Add(-time.Second), now)
		seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, runtimeTestSession, now.Add(-2*time.Second), now, false, false, collection.RuntimeProducerDisabled)
		w := postCoverage(t, db, principal, c)
		require.Equal(t, http.StatusConflict, w.Code)
		require.Contains(t, w.Body.String(), "runtime_producer_inactive")
	})

	t.Run("stale-lifecycle-lease", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		c := coverageWindow("coverage-000000000092", now.Add(-time.Second), now)
		seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, runtimeTestSession, now.Add(-2*time.Minute), now.Add(-2*collection.RuntimeProducerLeaseMaxAge), true, true, collection.RuntimeProducerActive)
		w := postCoverage(t, db, principal, c)
		require.Equal(t, http.StatusConflict, w.Code)
		require.Contains(t, w.Body.String(), "runtime_producer_inactive")
	})

	t.Run("stale-observation-recovers-with-fresh-lifecycle", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		c := coverageWindow("coverage-000000000094", now.Add(-time.Second), now)
		producer := seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, runtimeTestSession, now.Add(-2*time.Minute), now, true, true, collection.RuntimeProducerActive)
		staleEnd := now.Add(-collection.RuntimeProducerLeaseMaxAge - time.Second)
		require.NoError(t, db.Model(&producer).Update("last_coverage_end", staleEnd).Error)
		require.Equal(t, "stale", producer.EffectiveStatus(now))

		w := postCoverage(t, db, principal, c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
		require.Equal(t, collection.RuntimeProducerActive, producer.State)
		require.NotNil(t, producer.LastCoverageEnd)
		require.True(t, producer.LastCoverageEnd.Equal(c.WindowEnd))
	})

	t.Run("session-mismatch", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		c := coverageWindow("coverage-000000000093", now.Add(-time.Second), now)
		seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, "session-000000000002", now.Add(-2*time.Second), now, true, true, collection.RuntimeProducerStarting)
		w := postCoverage(t, db, principal, c)
		require.Equal(t, http.StatusConflict, w.Code)
		require.Contains(t, w.Body.String(), "runtime_producer_inactive")
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
		seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, runtimeTestSession, first.WindowStart.Add(-time.Second), now, true, true, collection.RuntimeProducerStarting)
		require.Equal(t, http.StatusOK, postCoverage(t, db, principal, first).Code)
		overlap := coverageWindow("coverage-000000000013", now.Add(-3*time.Second), now.Add(-time.Second))
		w := postCoverage(t, db, principal, overlap)
		require.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("source-kind-rebind", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		first := coverageWindow("coverage-000000000014", now.Add(-4*time.Second), now.Add(-2*time.Second))
		seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, runtimeTestSession, first.WindowStart.Add(-time.Second), now, true, true, collection.RuntimeProducerStarting)
		require.Equal(t, http.StatusOK, postCoverage(t, db, principal, first).Code)
		next := coverageWindow("coverage-000000000015", first.WindowEnd, now.Add(-time.Second))
		next.SourceKind = collection.RuntimeSourceFile
		w := postCoverage(t, db, principal, next)
		// The fixed producer/source registry rejects this before it can become a
		// persistence-level rebind conflict.
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("gap-restarts-continuity", func(t *testing.T) {
		db := runtimeCoverageDB(t)
		first := coverageWindow("coverage-000000000016", now.Add(-5*time.Second), now.Add(-4*time.Second))
		seedRuntimeProducer(t, db, principal, "falco", collection.RuntimeSourceFalco, runtimeTestSession, first.WindowStart.Add(-time.Second), now, true, true, collection.RuntimeProducerStarting)
		require.Equal(t, http.StatusOK, postCoverage(t, db, principal, first).Code)
		gap := coverageWindow("coverage-000000000017", now.Add(-2*time.Second), now.Add(-time.Second))
		require.Equal(t, http.StatusOK, postCoverage(t, db, principal, gap).Code)
		var row models.RuntimeCoverage
		require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
		require.NotNil(t, row.ContinuousSince)
		require.True(t, row.ContinuousSince.Equal(gap.WindowStart))
	})
}
