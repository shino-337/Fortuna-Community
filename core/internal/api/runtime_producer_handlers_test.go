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

func runtimeManifest(session string, started, reported time.Time, falcoEnabled bool) collection.RuntimeProducerManifest {
	producers := []collection.RuntimeProducerDeclaration{
		{ProducerID: "runtime-file", SourceKind: collection.RuntimeSourceFile},
		{ProducerID: "falco", SourceKind: collection.RuntimeSourceFalco, Enabled: falcoEnabled, Authoritative: falcoEnabled},
		{ProducerID: "ebpf-exec", SourceKind: collection.RuntimeSourceEBPF},
		{ProducerID: "ebpf-connect", SourceKind: collection.RuntimeSourceEBPF},
		{ProducerID: "ebpf-all", SourceKind: collection.RuntimeSourceEBPF},
	}
	collection.SortRuntimeProducerDeclarations(producers)
	return collection.RuntimeProducerManifest{
		Version: collection.RuntimeProducerManifestVersion,
		SessionID: session,
		SessionStartedAt: started,
		ReportedAt: reported,
		AgentState: collection.RuntimeAgentRunning,
		Producers: producers,
	}
}

func postRuntimeManifest(t *testing.T, db *gorm.DB, principal agentidentity.Principal, body collection.RuntimeProducerManifest) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("fortuna.agent.principal", principal)
		c.Next()
	})
	r.POST("/api/v2/runtime/producers", PostRuntimeProducerManifest(db))
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/producers", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRuntimeProducerLifecycleRestartDisableAndLease(t *testing.T) {
	db := runtimeCoverageDB(t)
	principal := agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	start1 := now.Add(-10 * time.Second)
	manifest1 := runtimeManifest(runtimeTestSession, start1, now.Add(-8*time.Second), true)

	w := postRuntimeManifest(t, db, principal, manifest1)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var producer models.RuntimeProducerState
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerStarting, producer.State)
	require.True(t, producer.Enabled)
	require.True(t, producer.Authoritative)
	require.NotNil(t, producer.GapSince)
	require.Equal(t, "startup", producer.GapReason)

	first := coverageWindow("lifecycle-coverage-0001", now.Add(-7*time.Second), now.Add(-6*time.Second))
	w = postCoverage(t, db, &principal, first)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerActive, producer.State)
	require.Nil(t, producer.GapSince)
	require.Empty(t, producer.GapReason)

	// Same-session heartbeat must preserve active state rather than returning the
	// producer to starting.
	heartbeat := manifest1
	heartbeat.ReportedAt = now.Add(-4 * time.Second)
	w = postRuntimeManifest(t, db, principal, heartbeat)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerActive, producer.State)

	var oldCoverage models.RuntimeCoverage
	require.NoError(t, db.First(&oldCoverage, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, "complete", oldCoverage.EffectiveStatus(&producer, now))

	// A new Agent execution session is an explicit evidence gap. Old coverage can
	// remain fresh by time, but cannot remain eligible because its session differs.
	session2 := "session-000000000002"
	start2 := now.Add(-3 * time.Second)
	manifest2 := runtimeManifest(session2, start2, now.Add(-2*time.Second), true)
	w = postRuntimeManifest(t, db, principal, manifest2)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerStarting, producer.State)
	require.Equal(t, session2, producer.SessionID)
	require.Equal(t, "agent_restart", producer.GapReason)
	require.NotNil(t, producer.GapSince)
	require.Equal(t, "unknown", oldCoverage.EffectiveStatus(&producer, now))
	require.False(t, oldCoverage.CoversInterval(&producer, oldCoverage.WindowStart, oldCoverage.WindowEnd, now))

	// Disabled configuration is persisted explicitly and keeps old coverage
	// ineligible even while the lifecycle lease itself is fresh.
	disabled := runtimeManifest(session2, start2, now.Add(-time.Second), false)
	w = postRuntimeManifest(t, db, principal, disabled)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerDisabled, producer.EffectiveStatus(now))
	require.Equal(t, "disabled", producer.GapReason)

	// Crash/network silence is represented by lease expiry; no explicit shutdown
	// message is required to fail closed.
	producer.Enabled = true
	producer.Authoritative = true
	producer.State = collection.RuntimeProducerActive
	producer.LastHeartbeatAt = now.Add(-collection.RuntimeProducerLeaseMaxAge - time.Second)
	require.Equal(t, "stale", producer.EffectiveStatus(now))
}

func TestRuntimeProducerStoppingManifestClosesAllLeases(t *testing.T) {
	db := runtimeCoverageDB(t)
	principal := agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	m := runtimeManifest(runtimeTestSession, now.Add(-time.Second), now, true)
	require.Equal(t, http.StatusOK, postRuntimeManifest(t, db, principal, m).Code)

	m.AgentState = collection.RuntimeAgentStopping
	m.ReportedAt = now.Add(time.Second)
	for i := range m.Producers {
		m.Producers[i].Enabled = false
		m.Producers[i].Authoritative = false
	}
	w := postRuntimeManifest(t, db, principal, m)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var states []models.RuntimeProducerState
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ?", "cluster-a", "agent-a").Find(&states).Error)
	require.Len(t, states, len(collection.RuntimeProducerRegistry))
	for _, state := range states {
		require.Equal(t, collection.RuntimeProducerStopped, state.State)
		require.False(t, state.Enabled)
		require.Equal(t, "agent_stopping", state.GapReason)
	}
}


func TestRuntimeProducerHeartbeatPersistsLeaseAndSilenceGaps(t *testing.T) {
	db := runtimeCoverageDB(t)
	principal := agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}
	now := time.Now().UTC().Truncate(time.Microsecond)
	started := now.Add(-10 * time.Second)
	manifest := runtimeManifest(runtimeTestSession, started, now.Add(-8*time.Second), true)
	require.Equal(t, http.StatusOK, postRuntimeManifest(t, db, principal, manifest).Code)

	first := coverageWindow("gap-coverage-00000001", now.Add(-7*time.Second), now.Add(-6*time.Second))
	require.Equal(t, http.StatusOK, postCoverage(t, db, &principal, first).Code)

	// Simulate a lifecycle transport/pause gap without changing process session.
	require.NoError(t, db.Model(&models.RuntimeProducerState{}).
		Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").
		Update("last_heartbeat_at", now.Add(-collection.RuntimeProducerLeaseMaxAge-time.Second)).Error)
	heartbeat := manifest
	heartbeat.ReportedAt = now
	w := postRuntimeManifest(t, db, principal, heartbeat)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var producer models.RuntimeProducerState
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerStarting, producer.State)
	require.Equal(t, "lifecycle_lease_expired", producer.GapReason)
	require.NotNil(t, producer.GapSince)

	// Even an adjacent complete window after the gap starts new continuity.
	second := coverageWindow("gap-coverage-00000002", first.WindowEnd, now.Add(-5*time.Second))
	w = postCoverage(t, db, &principal, second)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var row models.RuntimeCoverage
	require.NoError(t, db.First(&row, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.NotNil(t, row.ContinuousSince)
	require.True(t, row.ContinuousSince.Equal(second.WindowStart), "lease gap incorrectly extended old continuity")

	// Agent/config heartbeat alone cannot keep an active producer healthy if its
	// own observation windows stop arriving.
	staleEnd := time.Now().UTC().Add(-collection.RuntimeProducerLeaseMaxAge - time.Second)
	require.NoError(t, db.Model(&models.RuntimeProducerState{}).
		Where("cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").
		Updates(map[string]interface{}{
			"state": collection.RuntimeProducerActive,
			"last_coverage_end": staleEnd,
			"last_heartbeat_at": time.Now().UTC(),
		}).Error)
	heartbeat.ReportedAt = time.Now().UTC()
	w = postRuntimeManifest(t, db, principal, heartbeat)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.First(&producer, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	require.Equal(t, collection.RuntimeProducerStarting, producer.State)
	require.Equal(t, "producer_silent", producer.GapReason)
	require.NotNil(t, producer.GapSince)
}
