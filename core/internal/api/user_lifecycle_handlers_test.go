package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api"
	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

const userLifecycleSecret = "user-lifecycle-test-secret-key-32b!!"

func setupUserLifecycleDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	// SQLite :memory: databases are connection-local. Keep one connection
	// so GORM tests cannot lose the migrated schema between operations.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.AutoMigrate(&models.User{}, &models.Cluster{}, &models.Pod{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	hash, err := auth.HashPassword("UnitTestPass12!")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	seed := []models.User{
		{Username: "admin1", Email: "admin1@test.local", Password: hash, Role: models.RoleAdmin, Active: true},
		{Username: "uadmin1", Email: "uadmin1@test.local", Password: hash, Role: models.RoleUserAdmin, Active: true},
		{Username: "op1", Email: "op1@test.local", Password: hash, Role: models.RoleOperator, Active: true},
		{Username: "viewer1", Email: "viewer1@test.local", Password: hash, Role: models.RoleViewer, Active: true},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	if err := db.Create(&models.Cluster{ID: "c1", Name: "c1"}).Error; err != nil {
		t.Fatalf("seed cluster: %v", err)
	}
	pod := models.Pod{
		ClusterID: "c1", Name: "p1", Namespace: "ns1", ServiceAccount: "default", UID: "uid-p1",
		Containers: "[]", ImageDigests: "[]",
	}
	if err := db.Create(&pod).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}
	return db
}

func userIDByUsername(t *testing.T, db *gorm.DB, username string) uint {
	t.Helper()
	var u models.User
	if err := db.Where("username = ?", username).First(&u).Error; err != nil {
		t.Fatalf("lookup %s: %v", username, err)
	}
	return u.ID
}

func routerUserLifecycleV1(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}

	authG := r.Group("/api/v1/auth")
	authG.POST("/register",
		middleware.AuthMiddleware(db, userLifecycleSecret),
		p(authorization.PermissionAuthRegister),
		api.Register(db, userLifecycleSecret),
	)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, userLifecycleSecret))
	{
		v1.GET("/users", p(authorization.PermissionUsersRead), api.GetUsers(db))
		v1.PATCH("/users/:id", middleware.RequireAnyPermission(nil, authorization.PermissionUsersUpdate, authorization.PermissionUsersRoleAssign), api.PatchUser(db))
		v1.DELETE("/users/:id", p(authorization.PermissionUsersDelete), api.DeleteUser(db))
		v1.GET("/resources", p(authorization.PermissionInventoryRead), api.GetResources(db))
	}
	return r
}

func routerPatchUserWithPermissions(t *testing.T, db *gorm.DB, actorUsername string, perms ...authorization.Permission) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/api/v1/users/:id", func(c *gin.Context) {
		var actor models.User
		if err := db.Where("username = ?", actorUsername).First(&actor).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Set("user", &actor)
		c.Set(middleware.CtxPermissions, perms)
		c.Next()
	}, api.PatchUser(db))
	return r
}

func TestUserLifecycle_AdminRegistersNewUser(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")

	body := map[string]string{
		"username": "newop",
		"email":    "newop@test.local",
		"password": "AnotherPass12!",
		"role":     "operator",
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register: want 201 got %d %s", w.Code, w.Body.String())
	}
	var u models.User
	if err := db.Where("username = ?", "newop").First(&u).Error; err != nil {
		t.Fatalf("new user row: %v", err)
	}
	if authorization.NormalizeRole(u.Role) != models.RoleOperator {
		t.Fatalf("role got %q", u.Role)
	}
}

func TestUserLifecycle_AdminRegistersNewUserWithClusterScope(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")

	body := map[string]string{
		"username":  "scopedviewer",
		"email":     "scopedviewer@test.local",
		"password":  "AnotherPass12!",
		"role":      "viewer",
		"scopeJson": `{"clusters":["c1"]}`,
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register scoped: want 201 got %d %s", w.Code, w.Body.String())
	}
	var u models.User
	if err := db.Where("username = ?", "scopedviewer").First(&u).Error; err != nil {
		t.Fatalf("new scoped user row: %v", err)
	}
	if u.ScopeJSON != `{"clusters":["c1"]}` {
		t.Fatalf("scope got %q", u.ScopeJSON)
	}
}

func TestUserLifecycle_AdminRegistersClusterAdminWithClusterScope(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")

	body := map[string]string{
		"username":  "clusteradmin1",
		"email":     "clusteradmin1@test.local",
		"password":  "AnotherPass12!",
		"role":      "cluster_admin",
		"scopeJson": `{"clusters":["c1"]}`,
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register cluster_admin: want 201 got %d %s", w.Code, w.Body.String())
	}
	var u models.User
	if err := db.Where("username = ?", "clusteradmin1").First(&u).Error; err != nil {
		t.Fatalf("new cluster_admin user row: %v", err)
	}
	if authorization.NormalizeRole(u.Role) != models.RoleClusterAdmin {
		t.Fatalf("role got %q", u.Role)
	}
	if u.ScopeJSON != `{"clusters":["c1"]}` {
		t.Fatalf("scope got %q", u.ScopeJSON)
	}
}

func TestUserLifecycle_UserAdminCannotRegisterWithClusterScope(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")

	body := map[string]string{
		"username":  "badscope",
		"email":     "badscope@test.local",
		"password":  "AnotherPass12!",
		"role":      "viewer",
		"scopeJson": `{"clusters":["c1"]}`,
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_UserAdminCannotRegisterClusterAdmin(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")

	body := map[string]string{
		"username": "badclusteradmin",
		"email":    "badclusteradmin@test.local",
		"password": "AnotherPass12!",
		"role":     "cluster_admin",
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_UserAdminCannotRegisterAdmin(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")

	body := map[string]string{
		"username": "badadmin",
		"email":    "badadmin@test.local",
		"password": "AnotherPass12!",
		"role":     "admin",
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_OperatorCannotRegister(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "op1")

	body := map[string]string{
		"username": "x",
		"email":    "x@test.local",
		"password": "AnotherPass12!",
		"role":     "viewer",
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_AdminPatchesUserRole(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")
	opID := userIDByUsername(t, db, "op1")

	w := httptest.NewRecorder()
	patch := map[string]string{"role": "viewer"}
	b, _ := json.Marshal(patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+strconv.FormatUint(uint64(opID), 10), bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("patch: want 200 got %d %s", w.Code, w.Body.String())
	}
	var u models.User
	_ = db.First(&u, opID).Error
	if authorization.NormalizeRole(u.Role) != models.RoleViewer {
		t.Fatalf("role after patch: %q", u.Role)
	}
}

func TestUserLifecycle_AdminPatchesUserClusterScope(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")
	opID := userIDByUsername(t, db, "op1")

	w := httptest.NewRecorder()
	patch := map[string]string{"scopeJson": `{"clusters":["c1"]}`}
	b, _ := json.Marshal(patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+strconv.FormatUint(uint64(opID), 10), bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("patch scope: want 200 got %d %s", w.Code, w.Body.String())
	}
	var u models.User
	if err := db.First(&u, opID).Error; err != nil {
		t.Fatalf("user after patch: %v", err)
	}
	if u.ScopeJSON != `{"clusters":["c1"]}` {
		t.Fatalf("scope after patch: %q", u.ScopeJSON)
	}
}

func TestUserLifecycle_UserAdminCannotPatchClusterScope(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")
	opID := userIDByUsername(t, db, "op1")

	w := httptest.NewRecorder()
	patch := map[string]string{"scopeJson": `{"clusters":["c1"]}`}
	b, _ := json.Marshal(patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+strconv.FormatUint(uint64(opID), 10), bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_UpdatePermissionCannotPatchRole(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerPatchUserWithPermissions(t, db, "admin1", authorization.PermissionUsersUpdate)
	tok := ""
	opID := userIDByUsername(t, db, "op1")

	w := httptest.NewRecorder()
	patch := map[string]string{"role": "viewer"}
	b, _ := json.Marshal(patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+strconv.FormatUint(uint64(opID), 10), bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_RoleAssignPermissionCannotPatchActive(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerPatchUserWithPermissions(t, db, "admin1", authorization.PermissionUsersRoleAssign)
	tok := ""
	opID := userIDByUsername(t, db, "op1")

	w := httptest.NewRecorder()
	patch := map[string]bool{"active": false}
	b, _ := json.Marshal(patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+strconv.FormatUint(uint64(opID), 10), bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_UserAdminCannotPatchAdmin(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")
	adminID := userIDByUsername(t, db, "admin1")

	w := httptest.NewRecorder()
	patch := map[string]bool{"active": false}
	b, _ := json.Marshal(patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+strconv.FormatUint(uint64(adminID), 10), bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_UserAdminPatchesNonAdminRole(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")
	opID := userIDByUsername(t, db, "op1")

	w := httptest.NewRecorder()
	patch := map[string]string{"role": "viewer"}
	b, _ := json.Marshal(patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+strconv.FormatUint(uint64(opID), 10), bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_AdminDeletesUser(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")
	vID := userIDByUsername(t, db, "viewer1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+strconv.FormatUint(uint64(vID), 10), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: want 200 got %d %s", w.Code, w.Body.String())
	}
	var cnt int64
	db.Model(&models.User{}).Where("id = ? AND deleted_at IS NULL", vID).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("expected soft-deleted user id=%d still active count=%d", vID, cnt)
	}
}

func TestUserLifecycle_UserAdminCannotDeleteAdmin(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")
	adminID := userIDByUsername(t, db, "admin1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+strconv.FormatUint(uint64(adminID), 10), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_CannotDeleteSelf(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "admin1")
	selfID := userIDByUsername(t, db, "admin1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+strconv.FormatUint(uint64(selfID), 10), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_ViewerCannotListUsers(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "viewer1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_UserAdminCanListUsers(t *testing.T) {
	db := setupUserLifecycleDB(t)
	opID := userIDByUsername(t, db, "op1")
	if err := db.Model(&models.User{}).Where("id = ?", opID).Update("scope_json", `{"clusters":["c1"]}`).Error; err != nil {
		t.Fatalf("seed scope: %v", err)
	}
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	users, _ := resp["users"].([]interface{})
	if len(users) < 3 {
		t.Fatalf("expected users list, got %d", len(users))
	}
	var found map[string]interface{}
	for _, row := range users {
		m, _ := row.(map[string]interface{})
		if m["username"] == "op1" {
			found = m
			break
		}
	}
	if found == nil {
		t.Fatalf("op1 not found in users response: %s", w.Body.String())
	}
	if found["scopeJson"] != `{"clusters":["c1"]}` {
		t.Fatalf("scopeJson missing from users response: %#v", found["scopeJson"])
	}
	if _, ok := found["permissions"].([]interface{}); !ok {
		t.Fatalf("permissions missing from users response: %#v", found["permissions"])
	}
	scope, _ := found["operationalScope"].(map[string]interface{})
	if scope == nil || scope["restricted"] != true {
		t.Fatalf("operationalScope restricted missing: %#v", found["operationalScope"])
	}
}

func TestUserLifecycle_OperatorCanQueryResources(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "op1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources?kind=Pod&cluster=c1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	arr, _ := resp["resources"].([]interface{})
	if len(arr) != 1 {
		t.Fatalf("resources len want 1 got %d body=%s", len(arr), w.Body.String())
	}
}

func TestUserLifecycle_UserAdminCannotQueryResources(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources?kind=Pod", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_ViewerCanQueryResources(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "viewer1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources?kind=Pod&cluster=c1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("viewer resources: want 200 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_UserAdminCannotPatchOperatorToAdmin(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")
	opID := userIDByUsername(t, db, "op1")
	_ = db.Model(&models.User{}).Where("id = ?", opID).Update("role", models.RoleOperator).Error

	w := httptest.NewRecorder()
	patch := map[string]string{"role": "admin"}
	b, _ := json.Marshal(patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+strconv.FormatUint(uint64(opID), 10), bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestUserLifecycle_UserAdminCannotPatchOperatorToClusterAdmin(t *testing.T) {
	db := setupUserLifecycleDB(t)
	r := routerUserLifecycleV1(t, db)
	tok := loginToken(t, db, userLifecycleSecret, "uadmin1")
	opID := userIDByUsername(t, db, "op1")
	_ = db.Model(&models.User{}).Where("id = ?", opID).Update("role", models.RoleOperator).Error

	w := httptest.NewRecorder()
	patch := map[string]string{"role": "cluster_admin"}
	b, _ := json.Marshal(patch)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+strconv.FormatUint(uint64(opID), 10), bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}
