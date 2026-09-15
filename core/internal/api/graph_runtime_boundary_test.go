package api

import (
	"crypto/sha256"
	"fmt"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLegacyGraphFailsBeforeGlobalQuery(t *testing.T) {
	db, _ := serviceAccountScopeFixture(t)
	handlers := []gin.HandlerFunc{GetBlastRadius(db), GetShortestPath(db), GetAccessibleResources(db), ExecuteGraphQuery(db), GetServiceAccountPermissionsGraph(db), GetRiskyPods(db)}
	for i, handler := range handlers {
		for _, tc := range []struct {
			scope, query string
			missing      bool
			status       int
		}{
			{`{"cluster_ids":["a"]}`, "", false, 403}, {`{}`, "?cluster_id=a", false, 400}, {`{}`, "", true, 401},
		} {
			c, w := testGraphContext(tc.query)
			if !tc.missing {
				role := models.RoleOperator
				if tc.scope == `{}` {
					role = models.RoleAdmin
				}
				c.Set("user", &models.User{Role: role, ScopeJSON: tc.scope})
			}
			handler(c)
			if w.Code != tc.status {
				t.Errorf("handler %d %+v: %d %s", i, tc, w.Code, w.Body)
			}
		}
	}
	c, w := testGraphContext("?cluster_id=a&clusterId=b")
	c.Set("user", &models.User{Role: models.RoleAdmin})
	if _, err := graphClusterIDOrDefault(db, c); err == nil || w.Code != 400 {
		t.Fatal("graph conflicting aliases accepted")
	}
	for _, query := range []string{"?cluster=b", "?clusterId=b"} {
		c, w := testGraphContext(query)
		c.Set("user", &models.User{Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["a"]}`})
		if _, ok := scopeRuntimeQuery(db, c, db); ok || w.Code != 403 {
			t.Fatal("runtime forbidden alias accepted")
		}
	}
}
func testGraphContext(query string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test"+query, nil)
	return c, w
}
func TestNetworkServiceCacheSeparatesClustersAndCredentials(t *testing.T) {
	db, _ := serviceAccountScopeFixture(t)
	if err := db.Model(&models.Cluster{}).Where("id IN ?", []string{"a", "b"}).Update("kubeconfig", "invalid-test-config").Error; err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("a:%x", sha256.Sum256([]byte("invalid-test-config")))
	networkActivityServiceCache.Lock()
	networkActivityServiceCache.entries[key] = networkServiceCacheEntry{expiresAt: time.Now().Add(time.Minute), byIP: map[string]networkActivityServiceRef{"10.0.0.1": {Name: "only-a"}}}
	networkActivityServiceCache.Unlock()
	t.Cleanup(func() {
		networkActivityServiceCache.Lock()
		delete(networkActivityServiceCache.entries, key)
		networkActivityServiceCache.Unlock()
	})
	c, _ := testGraphContext("")
	if got := getNetworkActivityServiceCache(c.Request.Context(), db, "a"); got["10.0.0.1"].Name != "only-a" {
		t.Fatal("expected own cached service")
	}
	if got := getNetworkActivityServiceCache(c.Request.Context(), db, "b"); len(got) != 0 {
		t.Fatal("foreign cluster cache reused")
	}
	if err := db.Model(&models.Cluster{}).Where("id = ?", "a").Update("kubeconfig", "rotated-invalid-config").Error; err != nil {
		t.Fatal(err)
	}
	if got := getNetworkActivityServiceCache(c.Request.Context(), db, "a"); len(got) != 0 {
		t.Fatal("cache reused after credential rotation")
	}
}
