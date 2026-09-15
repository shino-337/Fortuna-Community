package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func serviceAccountScopeFixture(t *testing.T) (*gorm.DB, func(string, string, string, string) *httptest.ResponseRecorder) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.Cluster{}, &models.ServiceAccount{}, &models.Role{}, &models.ClusterRole{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}, &models.AuditLog{}, &models.SecurityActivityLog{}); err != nil {
		t.Fatal(err)
	}
	for _, cluster := range []string{"a", "b"} {
		for _, row := range []any{&models.Cluster{ID: cluster, Name: cluster}, &models.ServiceAccount{UID: "sa-" + cluster, ClusterID: cluster, Namespace: "shared", Name: "shared", Labels: `{"keep":"yes"}`}} {
			if err := db.Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	r := gin.New()
	group := r.Group("/api/v1")
	group.Use(func(c *gin.Context) {
		who := c.GetHeader("Test-User")
		perms := []authorization.Permission{authorization.PermissionInventoryRead, authorization.PermissionInventoryModify, authorization.PermissionInventoryDelete, authorization.PermissionInventoryBulk}
		if who == "bulk-only" {
			perms = []authorization.Permission{authorization.PermissionInventoryBulk}
		}
		if who == "viewer" {
			perms = []authorization.Permission{authorization.PermissionInventoryRead}
		}
		c.Set(middleware.CtxPermissions, perms)
		if who == "missing" {
			return
		}
		u := &models.User{Role: models.RoleOperator, ScopeJSON: `{"cluster_ids":["a"]}`}
		if who == "admin" {
			u.Role = models.RoleAdmin
		}
		c.Set("user", u)
	})
	registerInventoryRoutes(group, db, nil)
	// Legacy handlers are also protected if reused by older route wiring.
	group.GET("/legacy/:id", GetServiceAccount(db))
	group.PUT("/legacy/:id", UpdateServiceAccount(db))
	group.DELETE("/legacy/:id", DeleteServiceAccount(db))
	group.GET("/legacy/:id/permissions", GetServiceAccountPermissions(db))
	return db, func(method, path, who, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Test-User", who)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
}

func TestServiceAccountInventoryScope(t *testing.T) {
	_, request := serviceAccountScopeFixture(t)
	for _, tc := range []struct {
		path, who string
		total     int
	}{
		{"/inventory/serviceaccounts", "a", 1}, {"/inventory/serviceaccounts?pageSize=-1", "a", 1}, {"/inventory/serviceaccounts", "admin", 2}, {"/inventory/serviceaccounts?clusterId=a", "admin", 1},
	} {
		w := request("GET", tc.path, tc.who, "")
		var out struct {
			ServiceAccounts []models.ServiceAccount
			Total           int
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || out.Total != tc.total || len(out.ServiceAccounts) != tc.total {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body)
		}
	}
	for _, path := range []string{"/inventory/serviceaccounts/sa-b", "/inventory/serviceaccounts/sa-b/permissions", "/legacy/2", "/legacy/2/permissions"} {
		w := request("GET", path, "a", "")
		if w.Code != 403 {
			t.Fatalf("foreign %s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, suffix := range []string{"?cluster=b", "?clusterId=b"} {
		if w := request("GET", "/inventory/serviceaccounts"+suffix, "a", ""); w.Code != 403 {
			t.Fatalf("filter %s: %d", suffix, w.Code)
		}
	}
	if w := request("GET", "/inventory/serviceaccounts?cluster=a&clusterId=b", "admin", ""); w.Code != 400 {
		t.Fatalf("conflict: %d", w.Code)
	}
	if w := request("GET", "/inventory/serviceaccounts", "missing", ""); w.Code != 401 {
		t.Fatalf("missing auth: %d", w.Code)
	}
	if w := request("GET", "/inventory/serviceaccounts?page=9223372036854775807", "a", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"serviceAccounts":[]`) {
		t.Fatalf("large page: %d %s", w.Code, w.Body)
	}
	if w := request("GET", "/inventory/serviceaccounts/sa-a/permissions", "a", ""); w.Code != 200 {
		t.Fatalf("own permissions: %d %s", w.Code, w.Body)
	}
}

func TestServiceAccountMutationsProtectIdentity(t *testing.T) {
	db, request := serviceAccountScopeFixture(t)
	for _, path := range []string{"/inventory/serviceaccounts/sa-b", "/legacy/2"} {
		for _, method := range []string{"PUT", "DELETE"} {
			if w := request(method, path, "a", `{"labels":{}}`); w.Code != 403 {
				t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
			}
		}
	}
	for _, body := range []string{`{"cluster_id":"b"}`, `{"uid":"sa-b"}`, `{"id":2}`, `{"deleted_at":"2026-01-01"}`, `{"labels":{},"secrets":"[]"}`, `{"labels":null}`, `{"labels":{"x":1}}`} {
		if w := request("PUT", "/inventory/serviceaccounts/sa-a", "a", body); w.Code != 400 {
			t.Fatalf("body %s: %d %s", body, w.Code, w.Body)
		}
	}
	if w := request("PUT", "/inventory/serviceaccounts/sa-a", "viewer", `{"labels":{}}`); w.Code != 403 {
		t.Fatalf("viewer write: %d", w.Code)
	}
	for _, body := range []string{`{"labels":{"owner":"team-a"}}`, `{"labels":"{\"owner\":\"team-a\"}"}`} {
		if w := request("PUT", "/inventory/serviceaccounts/sa-a", "a", body); w.Code != 200 {
			t.Fatalf("labels: %d %s", w.Code, w.Body)
		}
	}
	// No configured target cluster: never fall back to the server's default cluster.
	if w := request("DELETE", "/inventory/serviceaccounts/sa-a", "a", ""); w.Code != 503 {
		t.Fatalf("unconfigured deletion: %d %s", w.Code, w.Body)
	}
	if err := db.Model(&models.Cluster{}).Where("id = ?", "a").Update("kubeconfig", "invalid-kubeconfig").Error; err != nil {
		t.Fatal(err)
	}
	if w := request("DELETE", "/inventory/serviceaccounts/sa-a", "a", ""); w.Code != 502 {
		t.Fatalf("invalid cluster credentials: %d %s", w.Code, w.Body)
	}
	var rows []models.ServiceAccount
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].UID != "sa-a" || rows[0].ClusterID != "a" || rows[0].Labels != `{"owner":"team-a"}` || rows[1].Labels != `{"keep":"yes"}` {
		t.Fatalf("mutated identity or foreign row: %+v", rows)
	}
	if err := db.Migrator().DropTable(&models.ServiceAccount{}); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/inventory/serviceaccounts", "a", ""); w.Code != 500 {
		t.Fatalf("query error: %d", w.Code)
	}
}
