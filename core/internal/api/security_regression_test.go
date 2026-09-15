package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api"
	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/internal/sessions"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

const securityRegressionJWTSecret = "security-regression-secret-32-bytes!!"

func newSecurityRegressionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	// SQLite :memory: databases belong to a connection. Insight actions start
	// asynchronous scoring, so an unrestricted pool can open a second, empty
	// database and make the next authenticated request lose its seeded user.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.AutoMigrate(&models.User{}, &models.UserSession{}, &models.ServiceAccount{}, &models.Pod{}, &models.ClusterRole{}, &models.Insight{}, &models.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func createSecurityUser(t *testing.T, db *gorm.DB, username, role, scopeJSON, password string) models.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := models.User{
		Username:  username,
		Email:     username + "@test.local",
		Password:  hash,
		Role:      role,
		ScopeJSON: scopeJSON,
		Active:    true,
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func tokenForUser(t *testing.T, db *gorm.DB, u models.User) (string, string) {
	t.Helper()
	sid, err := sessions.CreateLoginSession(db, u.ID, 24, "test", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	perms := authorization.ToStrings(authorization.PermissionsForUser(u.Role))
	tok, err := auth.GenerateToken(u.ID, u.Username, authorization.NormalizeRole(u.Role), perms, sid, securityRegressionJWTSecret, 24)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return tok, sid
}

func TestBulkServiceAccountDeleteHonorsClusterScope(t *testing.T) {
	db := newSecurityRegressionDB(t)
	user := createSecurityUser(t, db, "scoped-op", models.RoleOperator, `{"cluster_ids":["cluster-a"]}`, "OldPassword123!")
	token, _ := tokenForUser(t, db, user)

	saA := models.ServiceAccount{ClusterID: "cluster-a", Name: "sa-a", Namespace: "default", UID: "sa-a"}
	saB := models.ServiceAccount{ClusterID: "cluster-b", Name: "sa-b", Namespace: "default", UID: "sa-b"}
	if err := db.Create(&saA).Error; err != nil {
		t.Fatalf("create sa-a: %v", err)
	}
	if err := db.Create(&saB).Error; err != nil {
		t.Fatalf("create sa-b: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthMiddleware(db, securityRegressionJWTSecret))
	r.POST("/bulk-delete", api.BulkDeleteServiceAccounts(db))

	body, _ := json.Marshal(map[string]any{"ids": []uint{saA.ID, saB.ID}})
	req := httptest.NewRequest(http.MethodPost, "/bulk-delete", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bulk delete: want 403 got %d body=%s", w.Code, w.Body.String())
	}

	var countA, countB int64
	db.Unscoped().Model(&models.ServiceAccount{}).Where("id = ?", saA.ID).Count(&countA)
	db.Unscoped().Model(&models.ServiceAccount{}).Where("id = ?", saB.ID).Count(&countB)
	if countA != 1 {
		t.Fatalf("expected rejected batch to preserve in-scope service account, remaining count %d", countA)
	}
	if countB != 1 {
		t.Fatalf("expected out-of-scope service account preserved, remaining count %d", countB)
	}
}

func TestBulkServiceAccountDisableRejectsOversizedRequest(t *testing.T) {
	db := newSecurityRegressionDB(t)
	user := createSecurityUser(t, db, "bulk-op", models.RoleAdmin, `{}`, "OldPassword123!")
	token, _ := tokenForUser(t, db, user)

	ids := make([]uint, 101)
	for i := range ids {
		ids[i] = uint(i + 1)
	}
	body, _ := json.Marshal(map[string]any{"ids": ids})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthMiddleware(db, securityRegressionJWTSecret))
	r.POST("/bulk-disable", api.BulkDisableServiceAccounts(db))

	req := httptest.NewRequest(http.MethodPost, "/bulk-disable", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oversized bulk disable: want 400 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestInsightActionHonorsClusterScope(t *testing.T) {
	db := newSecurityRegressionDB(t)
	user := createSecurityUser(t, db, "cluster-admin-a", models.RoleClusterAdmin, `{"cluster_ids":["cluster-a"]}`, "OldPassword123!")
	token, _ := tokenForUser(t, db, user)

	podA := models.Pod{ClusterID: "cluster-a", Name: "pod-a", Namespace: "default", UID: "pod-a", Containers: "[]", ImageDigests: "[]"}
	podB := models.Pod{ClusterID: "cluster-b", Name: "pod-b", Namespace: "default", UID: "pod-b", Containers: "[]", ImageDigests: "[]"}
	if err := db.Create(&podA).Error; err != nil {
		t.Fatalf("create pod-a: %v", err)
	}
	if err := db.Create(&podB).Error; err != nil {
		t.Fatalf("create pod-b: %v", err)
	}
	insA := models.Insight{ResourceType: "Pod", ResourceNamespace: "default", ResourceName: "pod-a", ResourceUID: "pod-a", InsightType: "misconfiguration", Severity: "high", Title: "a", Description: "a"}
	insB := models.Insight{ResourceType: "Pod", ResourceNamespace: "default", ResourceName: "pod-b", ResourceUID: "pod-b", InsightType: "misconfiguration", Severity: "high", Title: "b", Description: "b"}
	if err := db.Create(&insA).Error; err != nil {
		t.Fatalf("create insight-a: %v", err)
	}
	if err := db.Create(&insB).Error; err != nil {
		t.Fatalf("create insight-b: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthMiddleware(db, securityRegressionJWTSecret))
	r.POST("/insights/:id/acknowledge", api.AcknowledgeInsight(db))

	req := httptest.NewRequest(http.MethodPost, "/insights/"+strconv.FormatUint(uint64(insA.ID), 10)+"/acknowledge", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("in-scope acknowledge: want 200 got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/insights/"+strconv.FormatUint(uint64(insB.ID), 10)+"/acknowledge", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("out-of-scope acknowledge: want 403 got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "cluster_scope") {
		t.Fatalf("expected cluster_scope reason, got %s", w.Body.String())
	}
}

func TestInsightReadHonorsClusterScope(t *testing.T) {
	db := newSecurityRegressionDB(t)
	user := createSecurityUser(t, db, "scoped-viewer", models.RoleViewer, `{"cluster_ids":["cluster-a"]}`, "OldPassword123!")
	token, _ := tokenForUser(t, db, user)

	podA := models.Pod{ClusterID: "cluster-a", Name: "pod-a", Namespace: "default", UID: "read-pod-a", Containers: "[]", ImageDigests: "[]"}
	podB := models.Pod{ClusterID: "cluster-b", Name: "pod-b", Namespace: "default", UID: "read-pod-b", Containers: "[]", ImageDigests: "[]"}
	if err := db.Create(&podA).Error; err != nil {
		t.Fatalf("create pod-a: %v", err)
	}
	if err := db.Create(&podB).Error; err != nil {
		t.Fatalf("create pod-b: %v", err)
	}
	insA := models.Insight{ResourceType: "Pod", ResourceNamespace: "default", ResourceName: "pod-a", ResourceUID: "read-pod-a", InsightType: "misconfiguration", Severity: "high", Title: "a", Description: "a"}
	insB := models.Insight{ResourceType: "Pod", ResourceNamespace: "default", ResourceName: "pod-b", ResourceUID: "read-pod-b", InsightType: "misconfiguration", Severity: "high", Title: "b", Description: "b"}
	if err := db.Create(&insA).Error; err != nil {
		t.Fatalf("create insight-a: %v", err)
	}
	if err := db.Create(&insB).Error; err != nil {
		t.Fatalf("create insight-b: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthMiddleware(db, securityRegressionJWTSecret))
	r.GET("/insights/:id", api.GetInsight(db))
	r.GET("/insights/:id/context", api.GetInsightContext(db))

	req := httptest.NewRequest(http.MethodGet, "/insights/"+strconv.FormatUint(uint64(insA.ID), 10), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("in-scope insight read: want 200 got %d body=%s", w.Code, w.Body.String())
	}

	for _, path := range []string{
		"/insights/" + strconv.FormatUint(uint64(insB.ID), 10),
		"/insights/" + strconv.FormatUint(uint64(insB.ID), 10) + "/context",
	} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("out-of-scope %s: want 403 got %d body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestResourcesHonorsClusterScope(t *testing.T) {
	db := newSecurityRegressionDB(t)
	user := createSecurityUser(t, db, "resources-viewer", models.RoleViewer, `{"cluster_ids":["cluster-a"]}`, "OldPassword123!")
	token, _ := tokenForUser(t, db, user)

	podA := models.Pod{ClusterID: "cluster-a", Name: "pod-a", Namespace: "default", UID: "resources-pod-a", Containers: "[]", ImageDigests: "[]"}
	podB := models.Pod{ClusterID: "cluster-b", Name: "pod-b", Namespace: "default", UID: "resources-pod-b", Containers: "[]", ImageDigests: "[]"}
	roleB := models.ClusterRole{ClusterID: "cluster-b", Name: "cluster-b-role", UID: "resources-cr-b", Rules: "[]"}
	for _, row := range []any{&podA, &podB, &roleB} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed resource: %v", err)
		}
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthMiddleware(db, securityRegressionJWTSecret))
	r.GET("/resources", api.GetResources(db))
	r.GET("/resources/:kind/:uid", api.GetResourceDetail(db))

	req := httptest.NewRequest(http.MethodGet, "/resources?kind=Pod&cluster=cluster-a", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "resources-pod-a") {
		t.Fatalf("in-scope resources: got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/resources?kind=Pod&cluster=cluster-b", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("out-of-scope resources list: want 403 got %d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/resources/ClusterRole/resources-cr-b", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("out-of-scope resource detail: want 403 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestInsightActionResolvesSoftDeletedPodClusterScope(t *testing.T) {
	db := newSecurityRegressionDB(t)
	user := createSecurityUser(t, db, "soft-delete-cluster-admin", models.RoleClusterAdmin, `{"cluster_ids":["cluster-a"]}`, "OldPassword123!")
	token, _ := tokenForUser(t, db, user)

	pod := models.Pod{ClusterID: "cluster-a", Name: "old-pod", Namespace: "default", UID: "soft-pod-a", Containers: "[]", ImageDigests: "[]"}
	if err := db.Create(&pod).Error; err != nil {
		t.Fatalf("create pod: %v", err)
	}
	if err := db.Delete(&pod).Error; err != nil {
		t.Fatalf("soft delete pod: %v", err)
	}
	ins := models.Insight{ResourceType: "Pod", ResourceNamespace: "default", ResourceName: "old-pod", ResourceUID: "soft-pod-a", InsightType: "misconfiguration", Severity: "high", Title: "old", Description: "old"}
	if err := db.Create(&ins).Error; err != nil {
		t.Fatalf("create insight: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthMiddleware(db, securityRegressionJWTSecret))
	r.POST("/insights/:id/acknowledge", api.AcknowledgeInsight(db))

	req := httptest.NewRequest(http.MethodPost, "/insights/"+strconv.FormatUint(uint64(ins.ID), 10)+"/acknowledge", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("soft-deleted pod insight acknowledge: want 200 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestChangePasswordInvalidatesOtherSessions(t *testing.T) {
	db := newSecurityRegressionDB(t)
	user := createSecurityUser(t, db, "password-op", models.RoleOperator, `{}`, "OldPassword123!")
	currentToken, currentSID := tokenForUser(t, db, user)
	_, otherSID := tokenForUser(t, db, user)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.AuthMiddleware(db, securityRegressionJWTSecret))
	r.POST("/change-password", api.ChangePassword(db))

	body := strings.NewReader(`{"oldPassword":"OldPassword123!","newPassword":"NewPassword123!"}`)
	req := httptest.NewRequest(http.MethodPost, "/change-password", body)
	req.Header.Set("Authorization", "Bearer "+currentToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("change password: want 200 got %d body=%s", w.Code, w.Body.String())
	}

	if _, err := sessions.ValidateActiveSession(db, user.ID, otherSID); err == nil {
		t.Fatal("expected other session to be invalid after password change")
	}
	if _, err := sessions.ValidateActiveSession(db, user.ID, currentSID); err != nil {
		t.Fatalf("expected current session to remain valid, got %v", err)
	}
}
