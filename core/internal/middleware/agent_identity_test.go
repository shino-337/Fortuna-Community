package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/gin-gonic/gin"
)

func writeAgentRegistry(t *testing.T, token, path string) {
	t.Helper()
	h := sha256.Sum256([]byte(token))
	data := `{"credentials":[{"id":"cred-a","cluster_id":"cluster-a","agent_id":"agent-a","token_sha256":"` + hex.EncodeToString(h[:]) + `","expires_at":"` + time.Now().UTC().Add(time.Hour).Format(time.RFC3339) + `"}]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestScopedAgentIdentityMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("a", 32)
	path := filepath.Join(t.TempDir(), "agents.json")
	writeAgentRegistry(t, token, path)
	store := agentidentity.Store{Path: path}

	request := func(header, value string) *httptest.ResponseRecorder {
		r := gin.New()
		r.POST("/agent", RequireAgentIdentity(store), func(c *gin.Context) {
			p, ok := AgentPrincipal(c)
			if !ok {
				c.Status(http.StatusInternalServerError)
				return
			}
			c.JSON(http.StatusOK, gin.H{"cluster": p.ClusterID, "agent": p.AgentID})
		})
		req := httptest.NewRequest(http.MethodPost, "/agent", nil)
		if header != "" {
			req.Header.Set(header, value)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	for _, tc := range []struct {
		name, header, value string
		want                int
	}{
		{"ingest-header", ingestTokenHeader, token, http.StatusOK},
		{"bearer", "Authorization", "Bearer " + token, http.StatusOK},
		{"missing", "", "", http.StatusUnauthorized},
		{"wrong", ingestTokenHeader, strings.Repeat("b", 32), http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := request(tc.header, tc.value); got.Code != tc.want {
				t.Fatalf("status=%d body=%s", got.Code, got.Body.String())
			}
		})
	}

	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := request(ingestTokenHeader, token); got.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid registry must fail closed: %d %s", got.Code, got.Body.String())
	}
}

func TestAgentPrincipalRejectsUntrustedContextValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set(agentPrincipalKey, "cluster-a")
	if _, ok := AgentPrincipal(ctx); ok {
		t.Fatal("non-principal context value accepted")
	}
}
