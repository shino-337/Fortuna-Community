package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

func TestRequireClusterScope_AllowsWhenUnrestricted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.AutoMigrate(&models.AuditLog{}, &models.SecurityActivityLog{})

	r := gin.New()
	r.GET("/cluster/:id/x", func(c *gin.Context) {
		c.Set("user", &models.User{ID: 2, Username: "op", Role: models.RoleOperator, ScopeJSON: "{}"})
		c.Set(middleware.CtxPermissions, authorization.PermissionsForRole(models.RoleOperator))
	}, middleware.RequireClusterScope(db, "id"), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cluster/99/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
}

func TestRequireClusterScope_DeniesOutOfScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.AutoMigrate(&models.AuditLog{}, &models.SecurityActivityLog{})

	r := gin.New()
	r.GET("/cluster/:id/x", func(c *gin.Context) {
		c.Set("user", &models.User{
			ID: 2, Username: "op", Role: models.RoleOperator,
			ScopeJSON: `{"cluster_ids":["1","2"]}`,
		})
		c.Set(middleware.CtxPermissions, authorization.PermissionsForRole(models.RoleOperator))
	}, middleware.RequireClusterScope(db, "id"), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cluster/99/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRequirePodUIDClusterScope_AllowsInScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.AuditLog{}, &models.SecurityActivityLog{})
	_ = db.Create(&models.Cluster{ID: "c1", Name: "cluster"}).Error
	_ = db.Create(&models.Pod{UID: "pod-1", Name: "p", Namespace: "ns", ClusterID: "c1", ServiceAccount: "default"}).Error

	r := gin.New()
	r.GET("/pods/:uid", func(c *gin.Context) {
		c.Set("user", &models.User{
			ID: 2, Username: "op", Role: models.RoleOperator,
			ScopeJSON: `{"clusters":["c1"]}`,
		})
		c.Set(middleware.CtxPermissions, authorization.PermissionsForRole(models.RoleOperator))
	}, middleware.RequirePodUIDClusterScope(db, "uid"), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/pods/pod-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRequirePodUIDClusterScope_DeniesOutOfScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.AuditLog{}, &models.SecurityActivityLog{})
	_ = db.Create(&models.Cluster{ID: "c1", Name: "cluster"}).Error
	_ = db.Create(&models.Pod{UID: "pod-1", Name: "p", Namespace: "ns", ClusterID: "c1", ServiceAccount: "default"}).Error

	r := gin.New()
	r.GET("/pods/:uid", func(c *gin.Context) {
		c.Set("user", &models.User{
			ID: 2, Username: "op", Role: models.RoleOperator,
			ScopeJSON: `{"clusters":["c2"]}`,
		})
		c.Set(middleware.CtxPermissions, authorization.PermissionsForRole(models.RoleOperator))
	}, middleware.RequirePodUIDClusterScope(db, "uid"), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/pods/pod-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d body=%s", w.Code, w.Body.String())
	}
}

// Scope validation must finish before handlers can read or mutate a resource.
func TestScopedPermissionChecksBeforeHandler(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.AutoMigrate(&models.AuditLog{}, &models.SecurityActivityLog{})
	for _, tc := range []struct {
		name, cluster string
		grant         bool
		want          int
	}{
		{"allowed", "allowed", true, 200},
		{"wrong cluster", "other", true, 403},
		{"missing permission", "allowed", false, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			r := gin.New()
			r.GET("/clusters/:id", func(c *gin.Context) {
				c.Set("user", &models.User{Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["allowed"]}`})
				if tc.grant {
					c.Set(middleware.CtxPermissions, []authorization.Permission{authorization.PermissionInventoryRead})
				}
			}, middleware.RequireScopedPermission(db, "id", authorization.PermissionInventoryRead), func(c *gin.Context) { called = true; c.Status(200) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/clusters/"+tc.cluster, nil))
			if w.Code != tc.want || called != (tc.want == 200) {
				t.Fatalf("status=%d handler_called=%v", w.Code, called)
			}
		})
	}
}

func TestPodScopeRetainedUnknownAndLookupFailure(t *testing.T) {
	for _, tc := range []struct {
		name, uid string
		broken    bool
		want      int
	}{
		{"retained allowed", "allowed", false, 200},
		{"retained denied", "other", false, 403},
		{"unknown", "unknown", false, 403},
		{"database failure", "allowed", true, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err = db.AutoMigrate(&models.Pod{}, &models.AuditLog{}, &models.SecurityActivityLog{}); err != nil {
				t.Fatal(err)
			}
			for _, uid := range []string{"allowed", "other"} {
				pod := models.Pod{UID: uid, ClusterID: uid, Name: uid, Namespace: "default"}
				if err = db.Create(&pod).Error; err != nil {
					t.Fatal(err)
				}
				if err = db.Delete(&pod).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.broken {
				if err = db.Migrator().DropTable(&models.Pod{}); err != nil {
					t.Fatal(err)
				}
			}
			called := false
			r := gin.New()
			r.GET("/pods/:uid", func(c *gin.Context) {
				c.Set("user", &models.User{Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["allowed"]}`})
			}, middleware.RequirePodUIDClusterScope(db, "uid"), func(c *gin.Context) { called = true; c.Status(200) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/pods/"+tc.uid, nil))
			if w.Code != tc.want || called != (tc.want == 200) {
				t.Fatalf("status=%d handler_called=%v", w.Code, called)
			}
		})
	}
}
