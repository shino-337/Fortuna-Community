package runtime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/stretchr/testify/require"
)

func TestSourceHealthRelayPreservesSignatureAndRejectsOldSession(t *testing.T) {
	now := time.Now().UTC()
	h := collection.SignedRuntimeSourceHealth{Report: collection.RuntimeSourceHealth{Version: 1, KeyID: "key", ClusterID: "cluster-a", AgentID: "agent-a", ProducerID: "falco", SessionID: "session-agent-000001", SourceKind: "falco", SourceSessionID: "session-sensor-000001", SourceStartedAt: now.Add(-time.Minute), Sequence: 1, WindowStart: now.Add(-2 * time.Second), WindowEnd: now.Add(-time.Second), ValidUntil: now.Add(30 * time.Second), Status: "healthy"}, Signature: "sensor-signature-is-opaque-to-agent"}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/api/v2/runtime/source-health", r.URL.Path)
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var got collection.SignedRuntimeSourceHealth
		require.NoError(t, json.Unmarshal(raw, &got))
		require.Equal(t, h, got)
		w.WriteHeader(200)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "health.json")
	raw, _ := json.Marshal([]collection.SignedRuntimeSourceHealth{h})
	require.NoError(t, os.WriteFile(path, raw, 0600))
	relay := SourceHealthRelay{CoreURL: server.URL, Path: path, ClusterID: h.Report.ClusterID, AgentID: h.Report.AgentID, SessionID: h.Report.SessionID}
	require.NoError(t, relay.Poll(context.Background()))
	require.NoError(t, relay.Poll(context.Background()))
	require.Equal(t, 1, calls)
	relay.SessionID = "session-agent-000002"
	require.Error(t, relay.Poll(context.Background()))
	require.Equal(t, 1, calls)
	challenge := filepath.Join(t.TempDir(), "challenge.json")
	require.NoError(t, WriteSourceHealthChallenge(challenge, relay.ClusterID, relay.AgentID, relay.SessionID, now))
	st, err := os.Stat(challenge)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), st.Mode().Perm())
}
func TestSourceHealthRelayBackoffStopsSiblingRequests(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
	}))
	defer server.Close()
	now := time.Now().UTC()
	h := collection.SignedRuntimeSourceHealth{Report: collection.RuntimeSourceHealth{Version: 1, KeyID: "key", ClusterID: "cluster-a", AgentID: "agent-a", ProducerID: "falco", SessionID: "session-agent-000001", SourceKind: "falco", SourceSessionID: "session-sensor-000001", SourceStartedAt: now.Add(-time.Minute), Sequence: 1, WindowStart: now.Add(-2 * time.Second), WindowEnd: now.Add(-time.Second), ValidUntil: now.Add(30 * time.Second), Status: "healthy"}}
	second := h
	second.Report.ProducerID = "runtime-file"
	second.Report.SourceKind = "file"
	path := filepath.Join(t.TempDir(), "health.json")
	raw, _ := json.Marshal([]collection.SignedRuntimeSourceHealth{h, second})
	require.NoError(t, os.WriteFile(path, raw, 0600))
	relay := SourceHealthRelay{CoreURL: server.URL, Path: path, ClusterID: h.Report.ClusterID, AgentID: h.Report.AgentID, SessionID: h.Report.SessionID}
	require.Error(t, relay.Poll(context.Background()))
	require.NoError(t, relay.Poll(context.Background()))
	require.Equal(t, 1, calls)
	require.True(t, relay.retryAt.After(now.Add(59*time.Second)))
}
