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

func governanceFixture(t *testing.T) (*gorm.DB, func(string, string, string, string) *httptest.ResponseRecorder) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	sqlDB, err := db.DB()
	if err != nil { t.Fatal(err) }
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.Pod{}, &models.PodAttackStep{}, &models.ExceptionPolicy{}, &models.Insight{}, &models.AuditLog{}, &models.SecurityActivityLog{}); err != nil {
		t.Fatal(err)
	}
	for i, uid := range []string{"a", "b", "a-history"} {
		cluster := "a"
		if uid == "b" { cluster = "b" }
		for _, row := range []any{
			&models.Pod{UID: uid, Name: "shared", Namespace: "shared", ClusterID: cluster},
			&models.PodAttackStep{PodUID: uid, StepID: "escape", Category: "escape", Confidence: float64(i+1)/4},
			&models.ExceptionPolicy{ResourceUID: uid, CVEID: "CVE-test", InsightType: "vulnerability", Reason: uid},
		} {
			if err := db.Create(row).Error; err != nil { t.Fatal(err) }
		}
	}
	if err := db.Where("uid = ?", "a-history").Delete(&models.Pod{}).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&models.ExceptionPolicy{ResourceUID: "orphan", CVEID: "CVE-test", InsightType: "vulnerability"}).Error; err != nil { t.Fatal(err) }
	r := gin.New()
	group := r.Group("/api/v1")
	group.Use(func(c *gin.Context) {
		who := c.GetHeader("Test-User")
		perms := []authorization.Permission{authorization.PermissionFindingsRead, authorization.PermissionRiskEvaluate, authorization.PermissionFindingsExceptionCreate, authorization.PermissionFindingsExceptionDelete}
		if who == "viewer" { perms = []authorization.Permission{authorization.PermissionFindingsRead} }
		c.Set(middleware.CtxPermissions, perms)
		if who == "missing" { return }
		u := &models.User{Role: models.RoleOperator}
		if who == "admin" {
			u.Role = models.RoleAdmin
		} else if who != "unrestricted" && who != "viewer" {
			u.ScopeJSON = `{"cluster_ids":["` + who + `"]}`
		}
		c.Set("user", u)
	})
	registerRiskRoutes(group, db)
	return db, func(method, path, who, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v1/risk"+path, strings.NewReader(body))
		req.Header.Set("Test-User", who)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
}

func TestRiskGovernanceAggregateScope(t *testing.T) {
	_, request := governanceFixture(t)
	for _, path := range []string{"/attack-steps/summary", "/exceptions"} {
		for _, alias := range []string{"cluster", "clusterId"} {
			w := request("GET", path+"?"+alias+"=b", "a", "")
			if w.Code != 403 { t.Fatalf("%s: %d %s", path, w.Code, w.Body) }
		}
		if w := request("GET", path+"?cluster=a&clusterId=b", "admin", ""); w.Code != 400 { t.Fatalf("conflict: %d", w.Code) }
		if w := request("GET", path, "missing", ""); w.Code != 401 { t.Fatalf("missing principal: %d", w.Code) }
	}
	var summary struct { Summary []struct { Count int; AvgConfidence float64 } }
	w := request("GET", "/attack-steps/summary", "a", "")
	if w.Code != 200 { t.Fatal(w.Body.String()) }
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil { t.Fatal(err) }
	if len(summary.Summary) != 1 || summary.Summary[0].Count != 1 || summary.Summary[0].AvgConfidence != .25 { t.Fatalf("summary=%+v", summary) }
	w = request("GET", "/attack-steps/summary", "admin", "")
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil { t.Fatal(err) }
	if w.Code != 200 || summary.Summary[0].Count != 2 { t.Fatalf("global: %s", w.Body) }
	var exceptions struct { Exceptions []models.ExceptionPolicy; Total int }
	for _, tc := range []struct { path, who string; total int }{
		{"/exceptions", "a", 2},
		{"/exceptions?clusterId=a", "admin", 2},
		{"/exceptions?resource_uid=b", "a", 0},
		{"/exceptions", "admin", 4},
	} {
		w = request("GET", tc.path, tc.who, "")
		if err := json.Unmarshal(w.Body.Bytes(), &exceptions); err != nil { t.Fatal(err) }
		if w.Code != 200 || exceptions.Total != tc.total { t.Fatalf("%+v: %d %s", tc, w.Code, w.Body) }
	}
}

func TestRiskExceptionsMutationsRespectOwnership(t *testing.T) {
	db, request := governanceFixture(t)
	body := func(uid string) string { return "{\"resourceUid\":\""+uid+"\",\"cveId\":\"CVE-new\",\"insightType\":\"vulnerability\",\"reason\":\"test\"}" }
	for _, uid := range []string{"b", "orphan"} {
		if w := request("POST", "/exceptions", "a", body(uid)); w.Code != 403 { t.Fatalf("create %s: %d %s", uid, w.Code, w.Body) }
	}
	if w := request("DELETE", "/exceptions/2", "a", ""); w.Code != 403 { t.Fatalf("delete foreign: %d", w.Code) }
	if w := request("DELETE", "/exceptions/not-a-number", "a", ""); w.Code != 400 { t.Fatalf("bad ID: %d", w.Code) }
	if w := request("POST", "/exceptions", "viewer", body("a")); w.Code != 403 { t.Fatalf("viewer create: %d", w.Code) }
	var count int64
	if err := db.Model(&models.ExceptionPolicy{}).Count(&count).Error; err != nil { t.Fatal(err) }
	if count != 4 { t.Fatalf("denied writes changed count: %d", count) }
	if w := request("POST", "/exceptions", "a", body("a-history")); w.Code != 201 { t.Fatalf("historical create: %d %s", w.Code, w.Body) }
	if w := request("DELETE", "/exceptions/3", "a", ""); w.Code != 200 { t.Fatalf("historical delete: %d %s", w.Code, w.Body) }
}

func TestGlobalEvaluationRejectsScopeBeforeWork(t *testing.T) {
	db, request := governanceFixture(t)
	// Worker resource tables are intentionally absent. Any attempted global work
	// would return 500 rather than the expected authorization/filter response.
	for _, path := range []string{"/insights/evaluate", "/insights/evaluate/historical"} {
		for _, tc := range []struct { who, suffix string; status int }{
			{"a", "", 403}, {"a", "?clusterId=a", 403},
			{"admin", "?cluster=a", 400}, {"viewer", "", 403}, {"missing", "", 401},
			{"admin", "", 500}, {"unrestricted", "", 500},
		} {
			w := request("POST", path+tc.suffix, tc.who, "")
			if w.Code != tc.status { t.Fatalf("%s %+v: %d %s", path, tc, w.Code, w.Body) }
		}
	}
	if err := db.Migrator().DropTable(&models.ExceptionPolicy{}, &models.PodAttackStep{}); err != nil { t.Fatal(err) }
	for _, path := range []string{"/exceptions", "/attack-steps/summary"} {
		w := request("GET", path, "a", "")
		if w.Code != 500 || strings.Contains(w.Body.String(), "no such table") { t.Fatalf("%s: %d %s", path, w.Code, w.Body) }
	}
}
