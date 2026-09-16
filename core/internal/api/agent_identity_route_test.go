package api_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/internal/api"
	"github.com/fortuna/core/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func writeAgentCredentialRegistry(t *testing.T, token, clusterID, agentID string) string {
	t.Helper()
	hash := sha256.Sum256([]byte(token))
	registry := map[string]any{
		"credentials": []map[string]any{{
			"id":           "route-test",
			"cluster_id":   clusterID,
			"agent_id":     agentID,
			"token_sha256": hex.EncodeToString(hash[:]),
			"expires_at":   time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		}},
	}
	data, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "agent-credentials.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAgentRegisteredRoutesUseScopedIdentityWithoutLegacyFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	scopedToken := strings.Repeat("s", 32)
	legacyToken := strings.Repeat("l", 32)
	cfg := &config.Config{
		JWTSecret:                   "x",
		AuthEnabled:                 true,
		IngestToken:                 legacyToken,
		AgentCredentialRegistryPath: writeAgentCredentialRegistry(t, scopedToken, "cluster-a", "agent-a"),
	}
	r := gin.New()
	api.SetupRoutesWithCertManager(r, db, cfg, nil, nil, nil)

	body := []byte(`{"podUid":"pod-1","clusterId":"cluster-a","namespace":"default","connections":[]}`)
	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/pod-network-connections", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Fortuna-Ingest-Token", token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := request(legacyToken); w.Code != http.StatusUnauthorized {
		t.Fatalf("legacy token must not bypass configured scoped auth: %d %s", w.Code, w.Body.String())
	}
	if w := request(scopedToken); w.Code != http.StatusOK {
		t.Fatalf("scoped credential should reach registered agent route: %d %s", w.Code, w.Body.String())
	}
}
