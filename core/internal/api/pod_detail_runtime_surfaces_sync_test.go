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

func TestPodDetailRuntimeSurfaces_ReturnDataAcrossLayers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.RuntimeEvent{},
		&models.RuntimeSignal{},
		&models.RuntimeBehaviorFact{},
		&models.RuntimeIncident{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	podUID := "pod-ui-sync-1"
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	ev := &models.RuntimeEvent{
		EventID:    "evt-ui-1",
		PodUID:     podUID,
		PodName:    "ui-pod",
		Namespace:  "default",
		Runtime:    "falco",
		EventType:  "exec",
		Signal:     "SUSPICIOUS_EXEC_FROM_SNAPSHOT",
		Severity:   "high",
		Syscall:    "execve",
		TargetPath: "/bin/sh",
		CreatedAt:  now,
		ObservedAt: &now,
		IngestedAt: &now,
		SourceKind: "falco",
		SourceRule: "Terminal shell in container",
	}
	if err := db.Create(ev).Error; err != nil {
		t.Fatalf("seed runtime event: %v", err)
	}
	if err := db.Create(&models.RuntimeSignal{
		PodUID:       podUID,
		SignalType:   "SUSPICIOUS_EXEC_FROM_SNAPSHOT",
		Category:     "EXECUTION",
		Confidence:   0.8,
		Evidence:     `{"syscall":"execve"}`,
		EvidenceRefs: `{"eventIds":["evt-ui-1"]}`,
		Count:        1,
		FirstSeenAt:  &nowStr,
		LastSeenAt:   &nowStr,
		CreatedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed runtime signal: %v", err)
	}
	if err := db.Create(&models.RuntimeBehaviorFact{
		FactID:     "fact-ui-1",
		PodUID:     podUID,
		Namespace:  "default",
		FactType:   "INTERACTIVE_SHELL",
		Domain:     "EXECUTION",
		ObservedAt: now,
		EventID:    ev.ID,
		Attributes: `{"shell":"sh"}`,
		SourceRef:  `{"eventIds":["evt-ui-1"]}`,
	}).Error; err != nil {
		t.Fatalf("seed runtime fact: %v", err)
	}
	if err := db.Create(&models.RuntimeIncident{
		IncidentID:   "inc-ui-1",
		PodUID:       podUID,
		Namespace:    "default",
		IncidentType: "POST_EXPLOIT_EXEC_CHAIN",
		SeverityHint: "high",
		Confidence:   0.9,
		FirstSeenAt:  now,
		LastSeenAt:   now,
		Window:       "15m",
		EvidenceRefs: `{"eventIds":["evt-ui-1"],"factIds":["fact-ui-1"]}`,
	}).Error; err != nil {
		t.Fatalf("seed runtime incident: %v", err)
	}

	r := gin.New()
	r.GET("/api/v1/risk/pods/:uid/runtime/events", GetPodRuntimeEvents(db))
	rt := r.Group("/api/v1/runtime")
	rt.GET("/pods/:uid/signals", GetRuntimeSignalsByPod(db))
	r.GET("/api/v2/runtime/pods/:uid/facts", GetPodRuntimeBehaviorFacts(db))
	r.GET("/api/v2/runtime/pods/:uid/incidents", GetPodRuntimeIncidents(db))

	assertCount := func(path string, key string) {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
		}
		var out map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s unmarshal: %v", path, err)
		}
		list, ok := out[key].([]interface{})
		if !ok || len(list) == 0 {
			t.Fatalf("%s expected non-empty %q, body=%v", path, key, out)
		}
	}

	assertCount("/api/v1/risk/pods/"+podUID+"/runtime/events?limit=20", "events")
	assertCount("/api/v1/runtime/pods/"+podUID+"/signals?limit=20", "signals")
	assertCount("/api/v2/runtime/pods/"+podUID+"/facts?limit=20", "facts")
	assertCount("/api/v2/runtime/pods/"+podUID+"/incidents?limit=20", "incidents")
}
