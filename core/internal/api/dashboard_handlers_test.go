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
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Agent{}, &models.Insight{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	old := now.Add(-2 * ActiveClusterCutoff)
	if err := db.Create([]models.Cluster{
		{ID: "cluster-active", Name: "active", Source: "env", Status: "active", LastSync: now},
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
		{ResourceType: "Pod", ResourceUID: "pod-active", ResourceName: "api", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "active", Description: "active", Status: "active", DetectedAt: now},
		{ResourceType: "Pod", ResourceUID: "pod-stale-cluster", ResourceName: "stale-api", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "stale", Description: "stale", Status: "active", DetectedAt: now},
		{ResourceType: "Pod", ResourceUID: "pod-deleted", ResourceName: "old-api", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "deleted", Description: "deleted", Status: "active", DetectedAt: now},
		{ResourceType: "Pod", ResourceUID: "pod-missing", ResourceName: "missing", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "missing", Description: "missing", Status: "active", DetectedAt: now},
		{ResourceType: "Pod", ResourceUID: "pod-active", ResourceName: "api", ResourceNamespace: "default", InsightType: "vulnerability", Severity: "critical", Title: "inactive", Description: "inactive", Status: "resolved", DetectedAt: now},
	}).Error; err != nil {
		t.Fatalf("seed insights: %v", err)
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
	if resp.RunningPods != 2 {
		t.Fatalf("running pods=%d", resp.RunningPods)
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
