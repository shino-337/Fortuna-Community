package api

import (
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"testing"
)

func TestRiskScoreRoutesDenyForeignPod(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Pod{}, &models.AuditLog{}, &models.SecurityActivityLog{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.Pod{UID: "foreign-pod", ClusterID: "other", Name: "foreign", Namespace: "default"}).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	group := r.Group("/api/v1")
	group.Use(func(c *gin.Context) {
		c.Set("user", &models.User{Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["allowed"]}`})
		c.Set(middleware.CtxPermissions, []authorization.Permission{authorization.PermissionFindingsRead, authorization.PermissionRiskEvaluate})
	})
	registerRiskRoutes(group, db)
	for _, req := range []struct{ method, path string }{{"GET", "/risk/scores/foreign-pod"}, {"POST", "/risk/scores/foreign-pod/calculate"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(req.method, "/api/v1"+req.path, nil))
		if w.Code != 403 {
			t.Fatalf("%s %s: %d %s", req.method, req.path, w.Code, w.Body.String())
		}
	}
}
