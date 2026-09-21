package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

func TestRequirePodUIDClusterScopeRejectsAmbiguousUIDAndPublishesResolvedCluster(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.AuditLog{}, &models.SecurityActivityLog{}); err != nil {
		t.Fatal(err)
	}

	seed := []models.Pod{
		{ClusterID: "cluster-a", UID: "dup", Name: "a", Namespace: "ns", ServiceAccount: "default"},
		{ClusterID: "cluster-b", UID: "dup", Name: "b", Namespace: "ns", ServiceAccount: "default"},
		{ClusterID: "cluster-a", UID: "unique", Name: "u", Namespace: "ns", ServiceAccount: "default"},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	r := gin.New()
	r.GET("/pods/:uid", func(c *gin.Context) {
		c.Set("user", &models.User{Role: models.RoleAdmin})
	}, middleware.RequirePodUIDClusterScope(db, "uid"), func(c *gin.Context) {
		clusterID, ok := middleware.ResolvedPodClusterID(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "missing resolved cluster"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"clusterId": clusterID})
	})

	ambiguous := httptest.NewRecorder()
	r.ServeHTTP(ambiguous, httptest.NewRequest(http.MethodGet, "/pods/dup", nil))
	if ambiguous.Code != http.StatusConflict {
		t.Fatalf("ambiguous UID status=%d body=%s", ambiguous.Code, ambiguous.Body.String())
	}

	resolved := httptest.NewRecorder()
	r.ServeHTTP(resolved, httptest.NewRequest(http.MethodGet, "/pods/unique", nil))
	if resolved.Code != http.StatusOK || resolved.Body.String() != "{\"clusterId\":\"cluster-a\"}" {
		t.Fatalf("resolved UID status=%d body=%s", resolved.Code, resolved.Body.String())
	}
}

func TestRequirePodUIDClusterScopeDoesNotDiscloseAmbiguousClustersToRestrictedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.AuditLog{}, &models.SecurityActivityLog{}); err != nil {
		t.Fatal(err)
	}
	for _, clusterID := range []string{"cluster-a", "cluster-b"} {
		pod := models.Pod{ClusterID: clusterID, UID: "dup", Name: clusterID, Namespace: "ns", ServiceAccount: "default"}
		if err := db.Create(&pod).Error; err != nil {
			t.Fatal(err)
		}
	}

	r := gin.New()
	r.GET("/pods/:uid", func(c *gin.Context) {
		c.Set("user", &models.User{Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["cluster-a"]}`})
	}, middleware.RequirePodUIDClusterScope(db, "uid"), func(c *gin.Context) {
		t.Fatal("ambiguous UID reached handler")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/pods/dup", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body == "" || body == `{"required_cluster_id":"cluster-a"}` || body == `{"required_cluster_id":"cluster-b"}` {
		t.Fatalf("unexpected disclosure body=%s", body)
	}
}
