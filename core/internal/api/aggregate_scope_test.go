package api

import (
	"encoding/json"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAggregateCacheIsolation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.RiskScore{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, cluster := range []string{"a", "b"} {
		for _, row := range []interface{}{
			&models.Cluster{ID: cluster, Name: cluster, Source: "env", LastSync: now},
			&models.Pod{UID: "pod-" + cluster, ClusterID: cluster, Name: cluster, Namespace: "default"},
			&models.Insight{ResourceType: "Pod", ResourceUID: "pod-" + cluster, ResourceName: cluster, InsightType: "vulnerability", Severity: "critical", Title: cluster, Description: cluster, Status: "acknowledged", DetectedAt: now.Add(-2 * time.Hour)},
			&models.RiskScore{ResourceType: "Pod", ResourceUID: "pod-" + cluster, ClusterID: cluster, TotalScore: 100, ScorerVersion: "v3", CalculatedAt: now},
		} {
			if err = db.Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	// Historical scores must not multiply histogram counts.
	if err = db.Create(&models.RiskScore{ResourceType: "Pod", ResourceUID: "pod-a", ClusterID: "a", TotalScore: 30, ScorerVersion: "v3", CalculatedAt: now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	oldCache := defaultRisksCache
	defaultRisksCache = NewMemoryRisksCache(time.Minute)
	t.Cleanup(func() { defaultRisksCache = oldCache })
	r := gin.New()
	r.Use(func(c *gin.Context) {
		scope := c.GetHeader("Test-Scope")
		u := &models.User{Role: models.RoleAdmin}
		if scope != "admin" {
			u.Role = models.RoleViewer
			u.ScopeJSON = `{"cluster_ids":["` + scope + `"]}`
		}
		c.Set("user", u)
	})
	r.GET("/summary", GetInsightsSummaryCached(db))
	r.GET("/list", GetInsightsListCached(db))
	r.GET("/global", GetInsightsSummaryGlobalCached(db))
	r.GET("/clusters", GetInsightsSummaryByCluster(db))
	r.GET("/histogram", GetRiskHistogram(db))
	r.GET("/dashboard", GetDashboardStats(db))
	request := func(path, scope string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Test-Scope", scope)
		r.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"/summary?sinceMinutes=0", "/global?sinceMinutes=0", "/histogram?sinceMinutes=0", "/dashboard"} {
		for _, scope := range []string{"admin", "a", "b", "a", "admin"} {
			w := request(path, scope)
			if w.Code != 200 {
				t.Fatalf("%s %s: %d %s", path, scope, w.Code, w.Body.String())
			}
			var body map[string]interface{}
			if err = json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			field := "total"
			if strings.HasPrefix(path, "/histogram") {
				field = "totalFindings"
			}
			if path == "/dashboard" {
				field = "totalRisks"
			}
			want := float64(1)
			if scope == "admin" {
				want = 2
			}
			if body[field] != want {
				t.Fatalf("%s %s: want %v got %s", path, scope, want, w.Body.String())
			}
			if strings.HasPrefix(path, "/histogram") {
				bins := body["bins"].([]interface{})
				if bins[9].(map[string]interface{})["count"] != want {
					t.Fatal("100-point scores missing from last bin")
				}
			}
		}
	}
	for _, path := range []string{"/summary", "/global", "/histogram", "/dashboard", "/clusters"} {
		// Warm the foreign cluster cache before checking authorization.
		request(path+"?clusterId=b", "admin")
		if w := request(path+"?clusterId=b", "a"); w.Code != 403 {
			t.Fatalf("foreign cache accepted: %s %d", path, w.Code)
		}
	}
	w := request("/clusters", "a")
	if w.Code != 200 || strings.Contains(w.Body.String(), `"clusterId":"b"`) {
		t.Fatalf("cluster rows leaked: %s", w.Body.String())
	}
	// An absent scoreBin and explicit scoreBin=0 must not share a cache entry.
	for _, tc := range []struct {
		query string
		want  float64
	}{
		{"/list?status=all&view=instance", 1},
		{"/list?status=all&view=instance&scoreBin=0", 0},
		{"/list?status=all&view=instance", 1},
	} {
		w := request(tc.query, "a")
		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || body["total"] != tc.want {
			t.Fatalf("list cache filter mismatch: %s %d %s", tc.query, w.Code, w.Body.String())
		}
	}
	// An error must not be cached as a successful empty summary.
	defaultRisksCache.ClearByPrefix("insights:summary:")
	if err = db.Migrator().DropTable(&models.Insight{}); err != nil {
		t.Fatal(err)
	}
	if w = request("/summary", "a"); w.Code != 500 {
		t.Fatalf("expected query error, got %d", w.Code)
	}
}
