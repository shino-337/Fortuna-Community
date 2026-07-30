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
