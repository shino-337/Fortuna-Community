package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api"
	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/internal/sessions"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

func setupAuthzIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.UserSession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	hash, err := auth.HashPassword("testpass12345")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	for _, u := range []models.User{
		{Username: "admin1", Email: "a@test.local", Password: hash, Role: models.RoleAdmin, Active: true},
		{Username: "uadmin1", Email: "ua@test.local", Password: hash, Role: models.RoleUserAdmin, Active: true},
		{Username: "viewer1", Email: "v@test.local", Password: hash, Role: models.RoleViewer, Active: true},
		{Username: "operator1", Email: "o@test.local", Password: hash, Role: models.RoleOperator, Active: true},
	} {
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	return db
}

func loginToken(t *testing.T, db *gorm.DB, secret string, username string) string {
	t.Helper()
	var u models.User
	if err := db.Where("username = ?", username).First(&u).Error; err != nil {
		t.Fatalf("user: %v", err)
	}
	norm := authorization.NormalizeRole(u.Role)
	perms := authorization.ToStrings(authorization.PermissionsForUser(u.Role))
	sid := ""
	if sessions.TableExists(db) {
		var err2 error
		sid, err2 = sessions.CreateLoginSession(db, u.ID, 24, "integration", "127.0.0.1", "test-agent")
		if err2 != nil {
			t.Fatalf("session: %v", err2)
		}
	}
	tok, err := auth.GenerateToken(u.ID, u.Username, norm, perms, sid, secret, 24)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return tok
}

func TestAuthorizationIntegration_ViewerCannotEvaluate(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, secret))
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}
	v1.POST("/risk/insights/evaluate", p(authorization.PermissionRiskEvaluate), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tok := loginToken(t, db, secret, "viewer1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/risk/insights/evaluate", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer evaluate: want 403 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthorizationIntegration_OperatorCanEvaluate(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, secret))
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}
	v1.POST("/risk/insights/evaluate", p(authorization.PermissionRiskEvaluate), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tok := loginToken(t, db, secret, "operator1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/risk/insights/evaluate", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("operator evaluate: want 204 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthorizationIntegration_GraphQueryAdvancedOnly(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, secret))
	v1.POST("/graph/query", middleware.RequirePermission(db, authorization.PermissionGraphQueryAdvanced), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	operatorToken := loginToken(t, db, secret, "operator1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/graph/query", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+operatorToken)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("operator graph query: want 403 got %d body=%s", w.Code, w.Body.String())
	}

	adminToken := loginToken(t, db, secret, "admin1")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/graph/query", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("admin graph query: want 204 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthorizationIntegration_AttackPathsEnforceClusterScope(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}); err != nil {
		t.Fatalf("migrate graph scope models: %v", err)
	}
	if err := db.Create(&models.Cluster{ID: "allowed", Name: "Allowed"}).Error; err != nil {
		t.Fatalf("seed allowed cluster: %v", err)
	}
	if err := db.Create(&models.Cluster{ID: "denied", Name: "Denied"}).Error; err != nil {
		t.Fatalf("seed denied cluster: %v", err)
	}
	if err := db.Create(&models.Pod{
		ClusterID:      "denied",
		Name:           "api",
		Namespace:      "default",
		ServiceAccount: "default",
		UID:            "pod-denied",
	}).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}
	if err := db.Model(&models.User{}).
		Where("username = ?", "viewer1").
		Update("scope_json", `{"cluster_ids":["allowed"]}`).Error; err != nil {
		t.Fatalf("scope viewer: %v", err)
	}

	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api.SetupRoutesWithCertManager(r, db, &config.Config{JWTSecret: secret, AuthEnabled: true}, nil, nil, nil)

	tok := loginToken(t, db, secret, "viewer1")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph/attack-paths/bundle?cluster_id=denied", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("bundle outside scope: want 403 got %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/graph/attack-paths/pod-denied", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("pod paths outside scope: want 403 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthorizationIntegration_ViewerInsightContextRedactsPods(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	_ = db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.RiskRule{})
	_ = db.Create(&models.Cluster{ID: "c1", Name: "c"}).Error
	pod := models.Pod{
		UID: "u1", Name: "p", Namespace: "ns", ClusterID: "c1", ServiceAccount: "default",
		Containers: `[{"name":"c1","env":[{"name":"E","value":"SECRET"}]}]`,
	}
	_ = db.Create(&pod).Error
	ins := models.Insight{
		ResourceType: "Pod", ResourceUID: pod.UID, ResourceName: pod.Name, ResourceNamespace: pod.Namespace,
		InsightType: "capability", Severity: "high", Title: "t", Description: "d", Status: "active",
		DetectedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	_ = db.Create(&ins).Error

	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, secret))
	p := func(perm authorization.Permission) gin.HandlerFunc {
		return middleware.RequirePermission(db, perm)
	}
	v1.GET("/risk/insights/:id/context", p(authorization.PermissionFindingsRead), api.GetInsightContext(db))

	tok := loginToken(t, db, secret, "viewer1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/risk/insights/"+strconv.FormatUint(uint64(ins.ID), 10)+"/context", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	pods, _ := body["pods"].([]interface{})
	if len(pods) != 1 {
		t.Fatalf("pods len %d", len(pods))
	}
	pm := pods[0].(map[string]interface{})
	if pm["containers"] != "[REDACTED]" {
		t.Fatalf("viewer should not see raw containers: %v", pm["containers"])
	}
}

func TestAuthorizationIntegration_AuditLogsViewerGlobalForbidden(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	_ = db.AutoMigrate(&models.AuditLog{})
	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, secret))
	v1.GET("/audit/logs", middleware.RequirePermission(db, authorization.PermissionSystemAuditRead), api.GetAuditLogs(db))

	tok := loginToken(t, db, secret, "viewer1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs?page=1&pageSize=10", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer global audit: want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestAuthorizationIntegration_AuditLogsViewerInsightWithoutResourceIDForbidden(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	_ = db.AutoMigrate(&models.AuditLog{}, &models.Insight{}, &models.Cluster{}, &models.Pod{})
	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, secret))
	v1.GET("/audit/logs", middleware.RequirePermission(db, authorization.PermissionSystemAuditRead), api.GetAuditLogs(db))

	tok := loginToken(t, db, secret, "viewer1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs?resource=insight&page=1&pageSize=10", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer insight audit without resource_id: want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestAuthorizationIntegration_AuditLogsViewerInsightWithResourceIDForbidden(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	_ = db.AutoMigrate(&models.AuditLog{}, &models.Insight{}, &models.Cluster{}, &models.Pod{})
	_ = db.Create(&models.Cluster{ID: "c1", Name: "c"}).Error
	pod := models.Pod{
		UID: "u-audit", Name: "p", Namespace: "ns", ClusterID: "c1", ServiceAccount: "default",
	}
	_ = db.Create(&pod).Error
	ins := models.Insight{
		ResourceType: "Pod", ResourceUID: pod.UID, ResourceName: pod.Name, ResourceNamespace: pod.Namespace,
		InsightType: "capability", Severity: "high", Title: "t", Description: "d", Status: "active",
		DetectedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	_ = db.Create(&ins).Error
	_ = db.Create(&models.AuditLog{
		Action: "view", Resource: "insight", ResourceID: strconv.FormatUint(uint64(ins.ID), 10),
		User: "viewer1", Details: "{}",
	}).Error

	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, secret))
	v1.GET("/audit/logs", middleware.RequirePermission(db, authorization.PermissionSystemAuditRead), api.GetAuditLogs(db))

	tok := loginToken(t, db, secret, "viewer1")
	w := httptest.NewRecorder()
	url := "/api/v1/audit/logs?resource=insight&resource_id=" + strconv.FormatUint(uint64(ins.ID), 10) + "&page=1&pageSize=10"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer insight audit with resource_id: want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestAuthorizationIntegration_AuditLogsAdminGlobalOK(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	_ = db.AutoMigrate(&models.AuditLog{})
	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, secret))
	v1.GET("/audit/logs", middleware.RequirePermission(db, authorization.PermissionSystemAuditRead), api.GetAuditLogs(db))

	tok := loginToken(t, db, secret, "admin1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs?page=1&pageSize=10", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin global audit: want 200 got %d %s", w.Code, w.Body.String())
	}
}

func TestAuthorizationIntegration_AuditLogsUserAdminRouteForbidden(t *testing.T) {
	db := setupAuthzIntegrationDB(t)
	_ = db.AutoMigrate(&models.AuditLog{})
	const secret = "integration-test-secret-key-32b!!"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(db, secret))
	v1.GET("/audit/logs", middleware.RequirePermission(db, authorization.PermissionSystemAuditRead), api.GetAuditLogs(db))

	tok := loginToken(t, db, secret, "uadmin1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs?resource=insight&page=1&pageSize=10", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("user_admin audit route: want 403 got %d %s", w.Code, w.Body.String())
	}
}
