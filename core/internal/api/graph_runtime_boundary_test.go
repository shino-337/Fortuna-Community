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

func TestGraphAndRuntimeClusterAliases(t *testing.T) {
	db, _ := serviceAccountScopeFixture(t)
	c, w := testGraphContext("?cluster_id=a&clusterId=b")
	c.Set("user", &models.User{Role: models.RoleAdmin})
	if _, err := graphClusterIDOrDefault(db, c); err == nil || w.Code != 400 {
		t.Fatal("graph conflicting aliases accepted")
	}
	for _, query := range []string{"?cluster=b", "?clusterId=b"} {
		c, w := testGraphContext(query)
		c.Set("user", &models.User{Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["a"]}`})
		if _, _, ok := scopeRuntimeQuery(db, c, db); ok || w.Code != 403 {
			t.Fatal("runtime forbidden alias accepted")
		}
	}
}
func TestRuntimePodFilterUsesExplicitClusterToDisambiguateDuplicateUID(t *testing.T) {
	db, _ := serviceAccountScopeFixture(t)
	for _, clusterID := range []string{"a", "b"} {
		pod := models.Pod{ClusterID: clusterID, UID: "dup-runtime", Name: "dup-" + clusterID, Namespace: "default"}
		if err := db.Create(&pod).Error; err != nil {
			t.Fatal(err)
		}
	}

	qualified, w := testGraphContext("?podUid=dup-runtime&clusterId=a")
	qualified.Set("user", &models.User{Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["a"]}`})
	_, clusterID, ok := scopeRuntimeQuery(db, qualified, db.Model(&models.RuntimeEvent{}))
	if !ok || w.Code >= 400 || clusterID != "a" {
		t.Fatalf("qualified duplicate runtime Pod was not resolved: ok=%v status=%d cluster=%q body=%s", ok, w.Code, clusterID, w.Body.String())
	}

	ambiguous, w := testGraphContext("?podUid=dup-runtime")
	ambiguous.Set("user", &models.User{Role: models.RoleAdmin})
	if _, _, ok := scopeRuntimeQuery(db, ambiguous, db.Model(&models.RuntimeEvent{})); ok || w.Code != 409 {
		t.Fatalf("UID-only duplicate runtime Pod should remain ambiguous: ok=%v status=%d body=%s", ok, w.Code, w.Body.String())
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
