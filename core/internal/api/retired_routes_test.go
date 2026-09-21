package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRetiredRoutesAreNotRegistered(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		h := newRuntimeRouteHarness(t, scoped)
		retired := []struct{ method, pattern, path string }{
			{"POST", "/api/v1/runtime/events", "/api/v1/runtime/events"},
			{"POST", "/api/v1/bulk/serviceaccounts/disable", "/api/v1/bulk/serviceaccounts/disable"},
			{"DELETE", "/api/v1/bulk/serviceaccounts/delete", "/api/v1/bulk/serviceaccounts/delete"},
			{"GET", "/api/v1/monitoring/agents", "/api/v1/monitoring/agents"},
			{"GET", "/api/v1/policy/rules/:id", "/api/v1/policy/rules/old-rule"},
			{"PUT", "/api/v1/policy/rules/:id", "/api/v1/policy/rules/old-rule"},
			{"DELETE", "/api/v1/policy/rules/:id", "/api/v1/policy/rules/old-rule"},
			{"POST", "/api/v1/policy/rules/:id/test", "/api/v1/policy/rules/old-rule/test"},
			{"GET", "/api/v1/policy/rules/:id/metrics", "/api/v1/policy/rules/old-rule/metrics"},
			{"GET", "/api/v1/policy/rules/:id/matches", "/api/v1/policy/rules/old-rule/matches"},
			{"GET", "/api/v1/graph/blast-radius/:uid", "/api/v1/graph/blast-radius/pod-a"},
			{"GET", "/api/v1/graph/shortest-path", "/api/v1/graph/shortest-path"},
			{"GET", "/api/v1/graph/accessible/:uid", "/api/v1/graph/accessible/sa-a"},
			{"POST", "/api/v1/graph/query", "/api/v1/graph/query"},
			{"GET", "/api/v1/graph/permissions/:uid", "/api/v1/graph/permissions/sa-a"},
			{"GET", "/api/v1/graph/risky-pods", "/api/v1/graph/risky-pods"},
		}
		routes := h.router.Routes()
		for _, old := range retired {
			for _, route := range routes {
				if route.Method == old.method && route.Path == old.pattern {
					t.Fatalf("retired route registered: %s %s (scoped=%v)", old.method, old.pattern, scoped)
				}
			}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(old.method, old.path, nil)
			req.Header.Set("X-Fortuna-Ingest-Token", h.scopedToken)
			h.router.ServeHTTP(w, req)
			if w.Code != http.StatusNotFound {
				t.Fatalf("retired route %s %s returned %d", old.method, old.path, w.Code)
			}
		}
		// Positive control: the supported routes must not disappear with the old ones.
		for _, want := range []struct{ method, path string }{
			{"POST", "/api/v2/runtime/events"},
			{"GET", "/api/v1/agents/status"},
			{"GET", "/api/v1/policy/rules/uid/:uid"},
			{"PUT", "/api/v1/policy/rules/uid/:uid"},
			{"DELETE", "/api/v1/policy/rules/uid/:uid"},
			{"POST", "/api/v1/policy/rules/uid/:uid/test"},
			{"GET", "/api/v1/policy/rules/uid/:uid/metrics"},
			{"GET", "/api/v1/policy/rules/uid/:uid/matches"},
			{"POST", "/api/v1/inventory/serviceaccounts/bulk/disable"},
			{"POST", "/api/v1/inventory/serviceaccounts/bulk/delete"},
			{"GET", "/api/v1/runtime/pods/:uid/signals"},
			{"GET", "/api/v1/graph"},
			{"GET", "/api/v1/graph/attack-paths/bundle"},
		} {
			found := false
			for _, route := range routes {
				if route.Method == want.method && route.Path == want.path {
					found = true
				}
			}
			if !found {
				t.Fatalf("supported route missing: %s %s", want.method, want.path)
			}
		}
	}
}
