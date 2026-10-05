package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDashboardStatsAffectedPodCountUsesActiveInventoryScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Agent{}, &models.Insight{}, &models.RiskScore{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	old := now.Add(-2 * ActiveClusterCutoff)
	if err := db.Create([]models.Cluster{
		{ID: "cluster-active", Name: "active", Source: "env", Status: "active", LastSync: now},
		{ID: "cluster-empty", Name: "empty", Source: "env", Status: "active", LastSync: now},
		{ID: "cluster-stale", Name: "stale", Source: "env", Status: "active", LastSync: old},
	}).Error; err != nil {
		t.Fatalf("seed clusters: %v", err)
	}
	if err := db.Create([]models.Pod{
		{UID: "pod-active", ClusterID: "cluster-active", Name: "api", Namespace: "default", ServiceAccount: "default"},
		{UID: "pod-stale-cluster", ClusterID: "cluster-stale", Name: "stale-api", Namespace: "default", ServiceAccount: "default"},
		{UID: "pod-deleted", ClusterID: "cluster-active", Name: "old-api", Namespace: "default", ServiceAccount: "default", DeletedAt: gorm.DeletedAt{Time: now, Valid: true}},
	}).Error; err != nil {
		t.Fatalf("seed pods: %v", err)
	}
	if err := db.Create([]models.Insight{
		{ClusterID: "cluster-active", ResourceType: "Pod", ResourceUID: "pod-active", ResourceName: "api", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "active", Description: "active", Status: "active", DetectedAt: now},
		{ClusterID: "cluster-stale", ResourceType: "Pod", ResourceUID: "pod-stale-cluster", ResourceName: "stale-api", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "stale", Description: "stale", Status: "active", DetectedAt: now},
		{ClusterID: "cluster-active", ResourceType: "Pod", ResourceUID: "pod-deleted", ResourceName: "old-api", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "deleted", Description: "deleted", Status: "active", DetectedAt: now},
		{ClusterID: "cluster-active", ResourceType: "Pod", ResourceUID: "pod-missing", ResourceName: "missing", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "missing", Description: "missing", Status: "active", DetectedAt: now},
		{ClusterID: "cluster-active", ResourceType: "Pod", ResourceUID: "pod-active", ResourceName: "api", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "inactive", Description: "inactive", Status: "resolved", DetectedAt: now},
	}).Error; err != nil {
		t.Fatalf("seed insights: %v", err)
	}

	// Critical counts the critical risk level (score >= 70), not the rule severity.
	for _, uid := range []string{"pod-active", "pod-stale-cluster", "pod-deleted", "pod-missing"} {
		clusterID := "cluster-active"
		if uid == "pod-stale-cluster" {
			clusterID = "cluster-stale"
		}
		db.Create(&models.RiskScore{ClusterID: clusterID, ResourceType: "Pod", ResourceUID: uid, TotalScore: 80, ScorerVersion: "v3", CalculatedAt: now})
	}

	router := gin.New()
	router.GET("/api/v1/dashboard/stats", GetDashboardStats(db))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats?byType=all", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp DashboardStatsDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TotalClusters != 2 {
		t.Fatalf("total clusters=%d, want 2 active inventory clusters including the empty cluster", resp.TotalClusters)
	}
	if resp.RunningPods != 1 {
		t.Fatalf("running pods=%d, want 1 active-cluster pod", resp.RunningPods)
	}
	if resp.AffectedPodCount != 1 {
		t.Fatalf("affected pod count=%d, want 1", resp.AffectedPodCount)
	}
	if resp.TotalRisks != 1 {
		t.Fatalf("total risks=%d, want 1", resp.TotalRisks)
	}
	if resp.CriticalRisks != 1 {
		t.Fatalf("critical risks=%d, want 1", resp.CriticalRisks)
	}
}

func TestClusterInventoryIncludesActiveClusterWithoutPods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Cluster{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create([]models.Cluster{
		{ID: "cluster-without-pods", Name: "empty", Source: "env", LastSync: now},
		{ID: "cluster-stale", Name: "stale", Source: "env", LastSync: now.Add(-2 * ActiveClusterCutoff)},
		{ID: "legacy", Name: "legacy", Source: "", LastSync: now},
	}).Error; err != nil {
		t.Fatal(err)
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	clusters, err := getClustersForAPI(db, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(clusters) != 1 || clusters[0].ID != "cluster-without-pods" {
		t.Fatalf("active cluster inventory should not depend on Pod rows: %+v", clusters)
	}
}

func TestDashboardStatsKeepsAcknowledgedRisks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Agent{}, &models.Insight{}, &models.RiskScore{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	db.Create(&models.Cluster{ID: "cluster-a", Name: "a", Source: "env", Status: "active", LastSync: now})
	db.Create(&models.Pod{UID: "pod-a", ClusterID: "cluster-a", Name: "a", Namespace: "default"})
	db.Create(&models.Insight{ClusterID: "cluster-a", ResourceType: "Pod", ResourceUID: "pod-a", ResourceName: "a", InsightType: "vulnerability", Severity: "critical", Title: "review", Description: "review", Status: "acknowledged", DetectedAt: now})
	db.Create(&models.RiskScore{ClusterID: "cluster-a", ResourceType: "Pod", ResourceUID: "pod-a", TotalScore: 75, ScorerVersion: "v3", CalculatedAt: now})
	r := gin.New()
	r.GET("/stats", GetDashboardStats(db))
	for _, query := range []string{"?byType=all", "?byType=all&clusterId=cluster-a", "?clusterId=cluster-a"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/stats"+query, nil))
		var stats DashboardStatsDTO
		if err = json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || stats.TotalRisks != 1 || stats.CriticalRisks != 1 || stats.AffectedPodCount != 1 {
			t.Fatalf("%s: %d %s", query, w.Code, w.Body.String())
		}
	}
}
