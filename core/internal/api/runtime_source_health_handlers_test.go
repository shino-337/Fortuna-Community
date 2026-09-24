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
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func postSourceHealth(t *testing.T, db *gorm.DB, principal *agentidentity.Principal, body collection.RuntimeSourceHealth) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if principal != nil {
		r.Use(func(c *gin.Context) {
			c.Set("fortuna.agent.principal", *principal)
			c.Next()
		})
	}
	r.POST("/api/v2/runtime/source-health", PostRuntimeSourceHealth(db))
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/source-health", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func sourceHealth(id, session, instance string, observed time.Time, status string) collection.RuntimeSourceHealth {
	h := collection.RuntimeSourceHealth{
		Version:          collection.RuntimeSourceHealthVersion,
		ID:               id,
		ProducerID:       "falco",
		SourceKind:       collection.RuntimeSourceFalco,
		SessionID:        session,
		ProbeKind:        collection.RuntimeSourceHealthProbeFalcoK8sReadiness,
		SourceInstanceID: instance,
		Status:           status,
		ObservedAt:       observed,
	}
	if status == collection.RuntimeSourceHealthUnhealthy {
		h.Reason = "falco readiness failed"
	}
	return h
}

func TestRuntimeSourceHealthEstablishesBoundedCoverageAuthority(t *testing.T) {
	db := runtimeCoverageDB(t)
	principal := agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	started := now.Add(-10 * time.Second)
	manifest := runtimeManifest(runtimeTestSession, started, now.Add(-9*time.Second), true)
	require.Equal(t, http.StatusOK, postRuntimeManifest(t, db, principal, manifest).Code)

	h1 := sourceHealth("source-health-00000001", runtimeTestSession, "falco-pod-uid-a", now.Add(-4*time.Second), collection.RuntimeSourceHealthHealthy)
	w := postSourceHealth(t, db, &principal, h1)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Exact replay is idempotent and does not append history.
	w = postSourceHealth(t, db, &principal, h1)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"replay":true`)

	changed := h1
	changed.Reason = "changed"
	w = postSourceHealth(t, db, &principal, changed)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	h2 := sourceHealth("source-health-00000002", runtimeTestSession, "falco-pod-uid-a", now.Add(-2*time.Second), collection.RuntimeSourceHealthHealthy)
	require.Equal(t, http.StatusOK, postSourceHealth(t, db, &principal, h2).Code)

	var producer models.RuntimeProducerState
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.True(t, producer.Authoritative)
	require.NotNil(t, producer.SourceHealthHealthySince)
	require.True(t, producer.SourceHealthHealthySince.Equal(h1.ObservedAt))
	require.True(t, producer.SourceHealthCovers(now.Add(-3500*time.Millisecond), now.Add(-2500*time.Millisecond), now))
	require.False(t, producer.SourceHealthCovers(now.Add(-3500*time.Millisecond), now.Add(-time.Second), now),
		"health observed only through h2 must not authorize a later interval")

	window := coverageWindow("health-covered-0000001", now.Add(-3500*time.Millisecond), now.Add(-2500*time.Millisecond))
	w = postCoverage(t, db, &principal, window)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var coverage models.RuntimeCoverage
	require.NoError(t, db.First(&coverage, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.NotNil(t, coverage.ContinuousSince)
	require.True(t, coverage.ContinuousSince.Equal(window.WindowStart))
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerActive, coverage.EffectiveStatus(&producer, now))
	require.True(t, coverage.CoversInterval(&producer, window.WindowStart, window.WindowEnd, now))

	var receipts int64
	require.NoError(t, db.Model(&models.RuntimeSourceHealthReceipt{}).Count(&receipts).Error)
	require.EqualValues(t, 2, receipts, "exact replay must not duplicate source-health history")
}

func TestRuntimeSourceHealthFailureAndRestartInvalidateCoverage(t *testing.T) {
	db := runtimeCoverageDB(t)
	principal := agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	started := now.Add(-12 * time.Second)
	require.Equal(t, http.StatusOK, postRuntimeManifest(t, db, principal, runtimeManifest(runtimeTestSession, started, now.Add(-11*time.Second), true)).Code)

	h1 := sourceHealth("source-health-reset-001", runtimeTestSession, "falco-pod-uid-a", now.Add(-6*time.Second), collection.RuntimeSourceHealthHealthy)
	h2 := sourceHealth("source-health-reset-002", runtimeTestSession, "falco-pod-uid-a", now.Add(-4*time.Second), collection.RuntimeSourceHealthHealthy)
	require.Equal(t, http.StatusOK, postSourceHealth(t, db, &principal, h1).Code)
	require.Equal(t, http.StatusOK, postSourceHealth(t, db, &principal, h2).Code)

	window := coverageWindow("source-health-reset-coverage", now.Add(-5500*time.Millisecond), now.Add(-4500*time.Millisecond))
	require.Equal(t, http.StatusOK, postCoverage(t, db, &principal, window).Code)

	var coverage models.RuntimeCoverage
	require.NoError(t, db.First(&coverage, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.NotNil(t, coverage.ContinuousSince)

	failed := sourceHealth("source-health-reset-003", runtimeTestSession, "", now.Add(-3*time.Second), collection.RuntimeSourceHealthUnhealthy)
	require.Equal(t, http.StatusOK, postSourceHealth(t, db, &principal, failed).Code)
	require.NoError(t, db.First(&coverage, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Nil(t, coverage.ContinuousSince)

	var producer models.RuntimeProducerState
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.False(t, producer.Authoritative)
	require.Equal(t, "source_health_failed", producer.GapReason)
	require.Equal(t, collection.RuntimeProducerNonAuthoritative, producer.EffectiveStatus(now))

	recovered := sourceHealth("source-health-reset-004", runtimeTestSession, "falco-pod-uid-a", now.Add(-2*time.Second), collection.RuntimeSourceHealthHealthy)
	require.Equal(t, http.StatusOK, postSourceHealth(t, db, &principal, recovered).Code)
	restarted := sourceHealth("source-health-reset-005", runtimeTestSession, "falco-pod-uid-b", now.Add(-time.Second), collection.RuntimeSourceHealthHealthy)
	require.Equal(t, http.StatusOK, postSourceHealth(t, db, &principal, restarted).Code)
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.True(t, producer.Authoritative)
	require.Equal(t, "falco-pod-uid-b", producer.SourceInstanceID)
	require.Equal(t, "source_restart", producer.GapReason)
	require.NotNil(t, producer.SourceHealthHealthySince)
	require.True(t, producer.SourceHealthHealthySince.Equal(restarted.ObservedAt))
}

func TestRuntimeSourceHealthRejectsWrongIdentitySessionAndUnsupportedProof(t *testing.T) {
	db := runtimeCoverageDB(t)
	principal := agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.Equal(t, http.StatusOK, postRuntimeManifest(t, db, principal, runtimeManifest(runtimeTestSession, now.Add(-3*time.Second), now.Add(-2*time.Second), true)).Code)

	h := sourceHealth("source-health-scope-0001", runtimeTestSession, "falco-pod-uid-a", now.Add(-time.Second), collection.RuntimeSourceHealthHealthy)
	require.Equal(t, http.StatusUnauthorized, postSourceHealth(t, db, nil, h).Code)

	wrongSession := h
	wrongSession.ID = "source-health-scope-0002"
	wrongSession.SessionID = "session-000000000099"
	require.Equal(t, http.StatusConflict, postSourceHealth(t, db, &principal, wrongSession).Code)

	unsupported := h
	unsupported.ID = "source-health-scope-0003"
	unsupported.ProducerID = "runtime-file"
	unsupported.SourceKind = collection.RuntimeSourceFile
	require.Equal(t, http.StatusBadRequest, postSourceHealth(t, db, &principal, unsupported).Code)
}

func TestRuntimeProducerAuthorityBitWithoutSourceHealthIsInsufficient(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	producer := models.RuntimeProducerState{
		ClusterID: "cluster-a",
		AgentID: "agent-a",
		ProducerID: "falco",
		SourceKind: collection.RuntimeSourceFalco,
		SessionID: runtimeTestSession,
		SessionStartedAt: now.Add(-time.Minute),
		Enabled: true,
		Authoritative: true,
		State: collection.RuntimeProducerActive,
		LastHeartbeatAt: now,
		LastCoverageEnd: func() *time.Time { t := now.Add(-time.Second); return &t }(),
	}
	coverage := models.RuntimeCoverage{
		ClusterID: "cluster-a",
		AgentID: "agent-a",
		ProducerID: "falco",
		SessionID: runtimeTestSession,
		Status: "complete",
		WindowStart: now.Add(-2*time.Second),
		WindowEnd: now.Add(-time.Second),
		ContinuousSince: func() *time.Time { t := now.Add(-2*time.Second); return &t }(),
	}
	require.False(t, producer.SourceHealthFresh(now))
	require.Equal(t, collection.RuntimeProducerNonAuthoritative, producer.EffectiveStatus(now))
	require.False(t, coverage.CoversInterval(&producer, coverage.WindowStart, coverage.WindowEnd, now))
}
