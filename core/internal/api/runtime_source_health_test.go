package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sourcehealth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type healthFixture struct {
	path      string
	private   ed25519.PrivateKey
	principal agentidentity.Principal
	now       time.Time
}

func newHealthFixture(t *testing.T) healthFixture {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	f := healthFixture{filepath.Join(t.TempDir(), "keys.json"), private, agentidentity.Principal{CredentialID: "cred", ClusterID: "cluster-a", AgentID: "agent-a"}, now}
	registry := sourcehealth.Registry{Version: 1, Keys: []sourcehealth.Key{{ID: "sensor-key", ClusterID: f.principal.ClusterID, AgentID: f.principal.AgentID, ProducerID: "falco", PublicKey: base64.StdEncoding.EncodeToString(public), NotBefore: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}}}
	raw, err := json.Marshal(registry)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(f.path, raw, 0600))
	return f
}
func (f healthFixture) report(start, end time.Time) collection.RuntimeSourceHealth {
	return collection.RuntimeSourceHealth{Version: 1, KeyID: "sensor-key", ClusterID: f.principal.ClusterID, AgentID: f.principal.AgentID, ProducerID: "falco", SessionID: runtimeTestSession, SourceKind: "falco", SourceSessionID: "sensor-session-000001", SourceStartedAt: f.now.Add(-30 * time.Second), Sequence: 1, WindowStart: start, WindowEnd: end, ValidUntil: end.Add(time.Minute), Status: "healthy"}
}
func (f healthFixture) sign(t *testing.T, r collection.RuntimeSourceHealth) collection.SignedRuntimeSourceHealth {
	t.Helper()
	payload, err := r.SigningBytes()
	require.NoError(t, err)
	return collection.SignedRuntimeSourceHealth{Report: r, Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(f.private, payload))}
}
func postHealth(db *gorm.DB, f healthFixture, signed collection.SignedRuntimeSourceHealth) *httptest.ResponseRecorder {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("fortuna.agent.principal", f.principal); c.Next() })
	r.POST("/health", PostRuntimeSourceHealth(db, f.path))
	raw, _ := json.Marshal(signed)
	req := httptest.NewRequest(http.MethodPost, "/health", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func healthState(t *testing.T, db *gorm.DB) models.RuntimeProducerState {
	t.Helper()
	var state models.RuntimeProducerState
	require.NoError(t, db.First(&state, "cluster_id = ? AND agent_id = ? AND producer_id = ?", "cluster-a", "agent-a", "falco").Error)
	return state
}
func TestSourceHealthAuthorityReplayFailureAndRestart(t *testing.T) {
	db := runtimeCoverageDB(t)
	f := newHealthFixture(t)
	seedRuntimeProducer(t, db, &f.principal, "falco", "falco", runtimeTestSession, f.now.Add(-30*time.Second), f.now, true, false, collection.RuntimeProducerStarting)
	h := f.report(f.now.Add(-10*time.Second), f.now.Add(-5*time.Second))
	signed := f.sign(t, h)
	w := postHealth(db, f, signed)
	require.Equal(t, 200, w.Code, w.Body.String())
	c := coverageWindow("source-health-coverage-001", h.WindowStart, h.WindowEnd)
	require.Equal(t, 200, postCoverage(t, db, &f.principal, c).Code)
	state := healthState(t, db)
	var coverage models.RuntimeCoverage
	require.NoError(t, db.First(&coverage).Error)
	require.True(t, coverage.CoversInterval(&state, h.WindowStart, h.WindowEnd, time.Now().UTC()))
	before := *state.SourceHealthReceivedAt
	w = postHealth(db, f, signed)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"replay":true`)
	require.True(t, healthState(t, db).SourceHealthReceivedAt.Equal(before))
	changed := h
	changed.ValidUntil = changed.ValidUntil.Add(-time.Second)
	require.Equal(t, 409, postHealth(db, f, f.sign(t, changed)).Code)
	failed := h
	failed.Sequence = 2
	failed.WindowStart = h.WindowEnd
	failed.WindowEnd = f.now.Add(-4 * time.Second)
	failed.Status = "failed"
	failed.Reason = "sensor dropped records"
	failed.Dropped = 1
	require.Equal(t, 200, postHealth(db, f, f.sign(t, failed)).Code)
	state = healthState(t, db)
	require.False(t, coverage.CoversInterval(&state, h.WindowStart, h.WindowEnd, time.Now().UTC()))
	require.False(t, state.Authoritative)
	require.Equal(t, 200, postHealth(db, f, signed).Code)
	require.False(t, healthState(t, db).Authoritative, "old healthy replay cannot restore authority")
	recovered := h
	recovered.Sequence = 3
	recovered.WindowStart = failed.WindowEnd
	recovered.WindowEnd = f.now.Add(-2 * time.Second)
	require.Equal(t, 200, postHealth(db, f, f.sign(t, recovered)).Code)
	state = healthState(t, db)
	require.True(t, state.SourceHealthSince.Equal(recovered.WindowStart))
	c = coverageWindow("source-health-coverage-002", recovered.WindowStart, recovered.WindowEnd)
	require.Equal(t, 200, postCoverage(t, db, &f.principal, c).Code)
	require.NoError(t, db.First(&coverage).Error)
	state = healthState(t, db)
	require.True(t, coverage.CoversInterval(&state, recovered.WindowStart, recovered.WindowEnd, time.Now().UTC()))
	require.False(t, coverage.CoversInterval(&state, h.WindowStart, recovered.WindowEnd, time.Now().UTC()))
	restarted := recovered
	restarted.SourceSessionID = "sensor-session-000002"
	restarted.SourceStartedAt = recovered.WindowEnd
	restarted.Sequence = 1
	restarted.WindowStart = recovered.WindowEnd
	restarted.WindowEnd = f.now.Add(-time.Second)
	require.Equal(t, 200, postHealth(db, f, f.sign(t, restarted)).Code)
	state = healthState(t, db)
	require.False(t, coverage.CoversInterval(&state, recovered.WindowStart, recovered.WindowEnd, time.Now().UTC()))
	require.False(t, state.SourceHealthCovers(restarted.WindowStart, restarted.WindowEnd, restarted.ValidUntil))
	manifest := runtimeManifest("session-000000000002", f.now, f.now, true)
	require.Equal(t, 200, postRuntimeManifest(t, db, f.principal, manifest).Code)
	require.False(t, healthState(t, db).Authoritative)
	require.Equal(t, 409, postHealth(db, f, f.sign(t, restarted)).Code)
}
func TestSourceHealthRejectsUntrustedEvidence(t *testing.T) {
	for _, name := range []string{"signature", "foreign-cluster", "foreign-agent", "old-session", "ebpf", "expired", "future", "dropped", "registry-missing", "disabled"} {
		t.Run(name, func(t *testing.T) {
			db := runtimeCoverageDB(t)
			f := newHealthFixture(t)
			seedRuntimeProducer(t, db, &f.principal, "falco", "falco", runtimeTestSession, f.now.Add(-30*time.Second), f.now, true, false, collection.RuntimeProducerStarting)
			h := f.report(f.now.Add(-2*time.Second), f.now.Add(-time.Second))
			status := 400
			switch name {
			case "foreign-cluster":
				h.ClusterID = "cluster-b"
				status = 403
			case "foreign-agent":
				h.AgentID = "agent-b"
				status = 403
			case "old-session":
				h.SessionID = "session-000000000002"
				status = 409
			case "ebpf":
				h.ProducerID = "ebpf-exec"
				h.SourceKind = "ebpf"
			case "expired":
				h.ValidUntil = f.now.Add(-time.Second)
			case "future":
				h.WindowEnd = f.now.Add(time.Second)
			case "dropped":
				h.Dropped = 1
			case "registry-missing":
				f.path = ""
				status = 403
			case "disabled":
				require.NoError(t, db.Model(&models.RuntimeProducerState{}).Where("producer_id = ?", "falco").Update("enabled", false).Error)
				status = 409
			case "signature":
				status = 403
			}
			signed := f.sign(t, h)
			if name == "signature" {
				signed.Signature = base64.StdEncoding.EncodeToString(make([]byte, 64))
			}
			w := postHealth(db, f, signed)
			require.Equal(t, status, w.Code, w.Body.String())
			require.False(t, healthState(t, db).Authoritative)
			var count int64
			require.NoError(t, db.Model(&models.RuntimeSourceHealthReceipt{}).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestSourceHealthRegistryRotationFailsClosed(t *testing.T) {
	for _, name := range []string{"revoked", "expired", "duplicate", "invalid-key", "unknown-field", "trailing-data"} {
		t.Run(name, func(t *testing.T) {
			f := newHealthFixture(t)
			h := f.report(f.now.Add(-2*time.Second), f.now.Add(-time.Second))
			signed := f.sign(t, h)
			require.NoError(t, sourcehealth.Verify(f.path, signed, f.now))
			raw, err := os.ReadFile(f.path)
			require.NoError(t, err)
			var registry sourcehealth.Registry
			require.NoError(t, json.Unmarshal(raw, &registry))
			switch name {
			case "revoked":
				registry.Keys[0].Revoked = true
			case "expired":
				registry.Keys[0].ExpiresAt = f.now
			case "duplicate":
				registry.Keys = append(registry.Keys, registry.Keys[0])
			case "invalid-key":
				registry.Keys[0].PublicKey = "bad"
			}
			raw, err = json.Marshal(registry)
			require.NoError(t, err)
			if name == "unknown-field" {
				raw = []byte(`{"version":1,"keys":[],"unexpected":true}`)
			}
			if name == "trailing-data" {
				raw = append(raw, []byte(` {}`)...)
			}
			require.NoError(t, os.WriteFile(f.path, raw, 0600))
			require.Error(t, sourcehealth.Verify(f.path, signed, f.now))
		})
	}
}
