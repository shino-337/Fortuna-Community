package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fortuna/core/internal/api"
	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type runtimeRouteHarness struct {
	router      *gin.Engine
	db          *gorm.DB
	scopedToken string
	legacyToken string
	registry    string
}

func newRuntimeRouteHarness(t *testing.T, scoped bool) runtimeRouteHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "runtime-route.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.RuntimeEvent{}); err != nil {
		t.Fatal(err)
	}
	for _, pod := range []models.Pod{
		{UID: "pod-a", ClusterID: "cluster-a", Namespace: "team-a", Name: "pod-a", ServiceAccount: "default"},
		{UID: "pod-b", ClusterID: "cluster-b", Namespace: "team-b", Name: "pod-b", ServiceAccount: "default"},
	} {
		if err := db.Create(&pod).Error; err != nil {
			t.Fatal(err)
		}
	}

	scopedToken := strings.Repeat("s", 32)
	legacyToken := strings.Repeat("l", 32)
	registry := ""
	if scoped {
		registry = writeAgentCredentialRegistry(t, scopedToken, "cluster-a", "agent-a")
	}
	cfg := &config.Config{
		JWTSecret:                   "x",
		AuthEnabled:                 true,
		IngestToken:                 legacyToken,
		AgentCredentialRegistryPath: registry,
	}
	r := gin.New()
	api.SetupRoutesWithCertManager(r, db, cfg, nil, nil, nil)
	return runtimeRouteHarness{router: r, db: db, scopedToken: scopedToken, legacyToken: legacyToken, registry: registry}
}

func (h runtimeRouteHarness) post(t *testing.T, route, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Fortuna-Ingest-Token", token)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

func assertNoRuntimeEvents(t *testing.T, db *gorm.DB) {
	t.Helper()
	var count int64
	if err := db.Model(&models.RuntimeEvent{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("runtime ownership rejection allowed %d persisted events", count)
	}
}

func TestRuntimeRegisteredRoutesRequireScopedIdentityAndOwnership(t *testing.T) {
	h := newRuntimeRouteHarness(t, true)
	for _, route := range []string{"/api/v1/runtime/events", "/api/v2/runtime/events"} {
		t.Run(strings.ReplaceAll(route, "/", "_"), func(t *testing.T) {
			validForA := `[{"pod":{"uid":"pod-a","namespace":"team-a"},"syscall":"execve","confidence":1}]`
			if w := h.post(t, route, h.legacyToken, validForA); w.Code != http.StatusUnauthorized {
				t.Fatalf("legacy token bypassed scoped runtime auth on %s: %d %s", route, w.Code, w.Body.String())
			}

			foreign := `[{"pod":{"uid":"pod-b","namespace":"team-b"},"syscall":"execve","confidence":1}]`
			if w := h.post(t, route, h.scopedToken, foreign); w.Code != http.StatusForbidden {
				t.Fatalf("cluster-a credential was not blocked from cluster-b pod on %s: %d %s", route, w.Code, w.Body.String())
			}
			assertNoRuntimeEvents(t, h.db)
		})
	}
}

func TestRuntimeRegisteredRoutesRejectMixedBatchBeforeEffects(t *testing.T) {
	h := newRuntimeRouteHarness(t, true)
	mixed := `[` +
		`{"pod":{"uid":"pod-a","namespace":"team-a"},"syscall":"execve","confidence":1},` +
		`{"pod":{"uid":"pod-b","namespace":"team-b"},"syscall":"connect","confidence":1}]`

	for _, route := range []string{"/api/v1/runtime/events", "/api/v2/runtime/events"} {
		t.Run(strings.ReplaceAll(route, "/", "_"), func(t *testing.T) {
			if w := h.post(t, route, h.scopedToken, mixed); w.Code != http.StatusForbidden {
				t.Fatalf("mixed cluster batch was not rejected on %s: %d %s", route, w.Code, w.Body.String())
			}
			assertNoRuntimeEvents(t, h.db)
		})
	}
}

func TestRuntimeRegisteredRoutesApplyRevocationAndRegistryFailureImmediately(t *testing.T) {
	h := newRuntimeRouteHarness(t, true)
	body := `[{"pod":{"uid":"pod-a","namespace":"team-a"},"syscall":"execve","confidence":1}]`

	data, err := os.ReadFile(h.registry)
	if err != nil {
		t.Fatal(err)
	}
	var registry map[string]any
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatal(err)
	}
	credentials := registry["credentials"].([]any)
	credential := credentials[0].(map[string]any)
	credential["revoked"] = true
	updated, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.registry, updated, 0600); err != nil {
		t.Fatal(err)
	}
	if w := h.post(t, "/api/v1/runtime/events", h.scopedToken, body); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked runtime credential accepted: %d %s", w.Code, w.Body.String())
	}
	assertNoRuntimeEvents(t, h.db)

	if err := os.WriteFile(h.registry, []byte(`{"credentials":[{"broken":true}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if w := h.post(t, "/api/v2/runtime/events", h.scopedToken, body); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("invalid runtime credential registry must fail unavailable: %d %s", w.Code, w.Body.String())
	}
	assertNoRuntimeEvents(t, h.db)
}

func TestRuntimeRegisteredRoutesPreserveExplicitLegacyMode(t *testing.T) {
	h := newRuntimeRouteHarness(t, false)
	// No scoped principal exists in explicit legacy mode, so the ownership guard
	// intentionally preserves compatibility. The handler receives an event that
	// lacks syscall and therefore performs no write while proving route reachability.
	body := `[{"pod":{"uid":"pod-b","namespace":"team-b"}}]`
	for _, route := range []string{"/api/v1/runtime/events", "/api/v2/runtime/events"} {
		if w := h.post(t, route, h.legacyToken, body); w.Code != http.StatusOK {
			t.Fatalf("legacy runtime mode unexpectedly rejected on %s: %d %s", route, w.Code, w.Body.String())
		}
	}
	assertNoRuntimeEvents(t, h.db)
}
