package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// TestGetPodRiskReport_IncludesPodRuntimeInsightsAndSummary verifies pod UID-scoped insights
// and runtime_signals counts match what the dashboard uses (risk tab + events/signals APIs).
func TestGetPodRiskReport_IncludesPodRuntimeInsightsAndSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.RuntimeSignal{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	clusterID := "cl-sync"
	if err := db.Create(&models.Cluster{ID: clusterID, Name: "c"}).Error; err != nil {
		t.Fatal(err)
	}
	podUID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	if err := db.Create(&models.Pod{
		ClusterID: clusterID, UID: podUID, Name: "app", Namespace: "ns", ServiceAccount: "default",
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&models.Insight{
		ResourceType: "Pod", ResourceNamespace: "ns", ResourceName: "app", ResourceUID: podUID,
		InsightType: "runtime-behavior", Severity: "high", Title: "Runtime behavior signals observed for this pod",
		Description: "test", Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.RuntimeSignal{
		PodUID: podUID, SignalType: "NETWORK_QUEUE_ANOMALY", Category: "NETWORK",
		Evidence: "{}", CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.GET("/api/v1/risk/pods/:uid/report", GetPodRiskReport(db))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/risk/pods/"+podUID+"/report", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Insights []models.Insight       `json:"insights"`
		Summary  map[string]interface{} `json:"summary"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	found := false
	for _, ins := range body.Insights {
		if ins.ResourceUID == podUID && ins.InsightType == "runtime-behavior" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pod runtime insight in report, got %d insights", len(body.Insights))
	}
	if v, ok := body.Summary["runtimeSignals24h"].(float64); !ok || int64(v) != 1 {
		t.Fatalf("summary.runtimeSignals24h: got %v (%T)", body.Summary["runtimeSignals24h"], body.Summary["runtimeSignals24h"])
	}
	if v, ok := body.Summary["podDirectInsightCount"].(float64); !ok || int(v) != 1 {
		t.Fatalf("summary.podDirectInsightCount: got %v", body.Summary["podDirectInsightCount"])
	}
	if v, ok := body.Summary["runtimePolicyInsightCount"].(float64); !ok || int(v) != 1 {
		t.Fatalf("summary.runtimePolicyInsightCount: got %v", body.Summary["runtimePolicyInsightCount"])
	}
}

func TestGetRuntimeSignalsByPod_MatchesReportWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeSignal{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	podUID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	now := time.Now().UTC()
	if err := db.Create(&models.RuntimeSignal{
		PodUID: podUID, SignalType: "PROC_ROOT_PIVOT", Category: "ESCAPE",
		Evidence: "{}", CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	rt := r.Group("/api/v1/runtime")
	rt.GET("/pods/:uid/signals", GetRuntimeSignalsByPod(db))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/pods/"+podUID+"/signals?sinceMinutes=1440&limit=50", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		PodUID  string                `json:"podUid"`
		Signals []models.RuntimeSignal `json:"signals"`
		Count   int                   `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Count != 1 || len(out.Signals) != 1 {
		t.Fatalf("expected 1 signal, got count=%d len=%d", out.Count, len(out.Signals))
	}
	if out.Signals[0].SignalType != "PROC_ROOT_PIVOT" {
		t.Fatalf("signal type: %s", out.Signals[0].SignalType)
	}
}
