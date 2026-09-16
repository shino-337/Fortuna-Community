package api

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func scopedAgentContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("fortuna.agent.principal", agentidentity.Principal{CredentialID: "cred-a", ClusterID: "cluster-a", AgentID: "agent-a"})
	return c
}

func TestValidateScopedSyncClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name      string
		legacyID  string
		cluster   *ClusterPayload
		agent     *AgentPayload
		mismatch  bool
		wantError bool
	}{
		{"legacy-cluster-alias", "cluster-a", nil, &AgentPayload{AgentID: "agent-a"}, false, false},
		{"cluster-object", "", &ClusterPayload{ID: "cluster-a"}, &AgentPayload{AgentID: "agent-a"}, false, false},
		{"matching-both-aliases", "cluster-a", &ClusterPayload{ID: "cluster-a"}, &AgentPayload{AgentID: "agent-a"}, false, false},
		{"conflicting-aliases", "cluster-a", &ClusterPayload{ID: "cluster-b"}, &AgentPayload{AgentID: "agent-a"}, false, true},
		{"foreign-cluster", "cluster-b", nil, &AgentPayload{AgentID: "agent-a"}, true, true},
		{"foreign-agent", "cluster-a", nil, &AgentPayload{AgentID: "agent-b"}, true, true},
		{"missing-agent", "cluster-a", nil, nil, false, true},
		{"missing-cluster", "", nil, &AgentPayload{AgentID: "agent-a"}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateScopedSyncClaims(scopedAgentContext(), tc.legacyID, tc.cluster, tc.agent)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v wantError=%v", err, tc.wantError)
			}
			if tc.mismatch != errors.Is(err, agentidentity.ErrIdentityMismatch) {
				t.Fatalf("identity mismatch=%v err=%v", tc.mismatch, err)
			}
		})
	}

	legacy, _ := gin.CreateTestContext(httptest.NewRecorder())
	if err := validateScopedSyncClaims(legacy, "legacy-cluster", nil, nil); err != nil {
		t.Fatalf("legacy migration mode changed unexpectedly: %v", err)
	}
}

func TestScopedSyncRejectsForeignClaimsBeforeDatabaseEffects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("fortuna.agent.principal", agentidentity.Principal{CredentialID: "cred-a", ClusterID: "cluster-a", AgentID: "agent-a"})
		c.Next()
	})
	r.POST("/sync", SyncDataFromAgent(db, nil))

	body := []byte(`{"clusterId":"cluster-b","agent":{"agentId":"agent-a"},"data":{"isFullSync":false}}`)
	req := httptest.NewRequest(http.MethodPost, "/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("foreign cluster should be rejected before DB access: %d %s", w.Code, w.Body.String())
	}
}
