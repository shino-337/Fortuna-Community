package api

import (
	"bytes"
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

// TestPostRuntimeEvents_AgentPayloadCreatesSemanticSignal verifies agent→core contract:
// POST /api/v1/runtime/events with pod.uid + syscall + target persists runtime_events and runtime_signals (REP).
func TestPostRuntimeEvents_AgentPayloadCreatesSemanticSignal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeSignal{}, &models.PodRiskProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	r.POST("/api/v1/runtime/events", PostRuntimeEvents(db))

	payload := []map[string]interface{}{{
		"pod": map[string]interface{}{
			"uid":       "cccccccc-cccc-cccc-cccc-cccccccccccc",
			"namespace": "ns",
			"name":      "work",
		},
		"syscall":    "open",
		"target":     "/proc/1/root",
		"confidence": 0.9,
	}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	var events []models.RuntimeEvent
	if err := db.Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("runtime_events: want 1 got %d", len(events))
	}
	if events[0].PodUID != "cccccccc-cccc-cccc-cccc-cccccccccccc" || events[0].Syscall != "open" {
		t.Fatalf("event: %+v", events[0])
	}

	var signals []models.RuntimeSignal
	if err := db.Find(&signals).Error; err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("runtime_signals: want 1 got %d", len(signals))
	}
	if signals[0].SignalType != "PROC_ROOT_PIVOT" {
		t.Fatalf("expected PROC_ROOT_PIVOT, got %q", signals[0].SignalType)
	}
}

// TestRuntimeEventPayload_UnmarshalAgentShape mirrors agent/internal/runtime.Event JSON shape.
func TestRuntimeEventPayload_UnmarshalAgentShape(t *testing.T) {
	raw := `{
		"event_type": "runtime.exec",
		"pod": {"uid": "ddd", "namespace": "n", "name": "p"},
		"syscall": "openat",
		"target": "/proc/1/root",
		"timestamp": 1700000000
	}`
	var p runtimeEventPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	if p.Pod.UID != "ddd" || p.Syscall != "openat" || p.Target != "/proc/1/root" {
		t.Fatalf("unmarshal: %+v", p)
	}
	if p.Timestamp != 1700000000 {
		t.Fatalf("timestamp: %d", p.Timestamp)
	}
	// Core accepts timestamp 0 as "now" path — sanity only
	_ = time.Unix(p.Timestamp, 0)
}

// TestPostRuntimeEvents_EBPFExecTrace_IngestsAndMapsSignal locks the R6/R9 e2e contract:
// flat pod_uid + execve + EBPF_EXEC_TRACE -> processed>=1 and runtime_signals.signal_type EBPF_EXEC_ACTIVITY.
func TestPostRuntimeEvents_EBPFExecTrace_IngestsAndMapsSignal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeEvent{}, &models.RuntimeSignal{}, &models.PodRiskProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	r.POST("/api/v1/runtime/events", PostRuntimeEvents(db))

	podUID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	body := []byte(`[{
		"pod_uid": "` + podUID + `",
		"namespace": "fortuna",
		"syscall": "execve",
		"target_path": "/bin/sh-e2e-test",
		"capability": "EBPF_EXEC_TRACE",
		"confidence": 0.9,
		"timestamp": 1700000001
	}]`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response json: %v", err)
	}
	if int(resp["processed"].(float64)) < 1 {
		t.Fatalf("expected processed>=1, got %v", resp["processed"])
	}

	var signals []models.RuntimeSignal
	if err := db.Where("pod_uid = ? AND signal_type = ?", podUID, "EBPF_EXEC_ACTIVITY").Find(&signals).Error; err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("runtime_signals: want 1 EBPF_EXEC_ACTIVITY, got %d", len(signals))
	}
}
