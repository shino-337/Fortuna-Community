package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
)


func useRuntimeAgentPrincipal(r *gin.Engine, clusterID string) {
	r.Use(func(c *gin.Context) {
		c.Set("fortuna.agent.principal", agentidentity.Principal{
			CredentialID: "test-runtime-credential",
			ClusterID: clusterID,
			AgentID: "agent-a",
		})
		c.Next()
	})
}

func TestPostRuntimeEventsV2_RejectsProcessableEventWithoutSourceRecordID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.RuntimeEvent{}, &models.RuntimeSignal{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}, &models.PodRiskProfile{}, &models.PodCapability{}, &models.CapabilityMetadata{}, &models.PromotionRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	useRuntimeAgentPrincipal(r, "c1")
	r.POST("/api/v2/runtime/events", requireScopedRuntimeOwnership(db), PostRuntimeEventsV2Scoped(db))
	if err := db.Create(&models.Pod{UID: "pod-no-record-id", ClusterID: "c1", Namespace: "ns", Name: "demo"}).Error; err != nil {
		t.Fatal(err)
	}
	payload := []map[string]interface{}{{
		"pod": map[string]interface{}{"uid": "pod-no-record-id", "namespace": "ns"},
		"syscall": "execve",
		"target": "/bin/sh",
		"confidence": 0.9,
	}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing source_record_id status=%d body=%s", w.Code, w.Body.String())
	}
	var count int64
	if err := db.Model(&models.RuntimeEvent{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("missing source_record_id persisted %d runtime events", count)
	}
}


func TestPostRuntimeEventsV2_ExactReplaySkipsDownstreamEffects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.RuntimeEvent{}, &models.RuntimeSignal{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}, &models.PodRiskProfile{}, &models.PodCapability{}, &models.CapabilityMetadata{}, &models.PromotionRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_runtime_event_agent_source_record_identity
		ON runtime_events(cluster_id, agent_id, source_record_id)
		WHERE source_record_id IS NOT NULL AND source_record_id <> ''
	`).Error; err != nil {
		t.Fatalf("idempotency index: %v", err)
	}

	if err := db.Create(&models.Pod{UID: "pod-replay", ClusterID: "c1", Namespace: "ns", Name: "demo"}).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	useRuntimeAgentPrincipal(r, "c1")
	r.POST("/api/v2/runtime/events", requireScopedRuntimeOwnership(db), PostRuntimeEventsV2Scoped(db))

	payload := []map[string]interface{}{{
		"event_id":         "same-second-event-id",
		"source_record_id": "abababababababababababababababababababababababababababababababab",
		"payload_hash":     "same-payload-hash",
		"payload_json":     map[string]interface{}{"kind": "open"},
		"pod":              map[string]interface{}{"uid": "pod-replay", "namespace": "ns"},
		"syscall":          "open",
		"target":           "/proc/1/root",
		"confidence":       0.9,
	}}
	body, _ := json.Marshal(payload)

	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/events", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	first := post()
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	second := post()
	if second.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", second.Code, second.Body.String())
	}
	var replayResp runtimeEventResponse
	if err := json.Unmarshal(second.Body.Bytes(), &replayResp); err != nil {
		t.Fatal(err)
	}
	if replayResp.Processed != 0 || replayResp.Duplicates != 1 {
		t.Fatalf("unexpected replay response: %+v", replayResp)
	}

	var eventCount int64
	if err := db.Model(&models.RuntimeEvent{}).Count(&eventCount).Error; err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("exact replay persisted %d events", eventCount)
	}
	var signal models.RuntimeSignal
	if err := db.Where("cluster_id = ? AND pod_uid = ?", "c1", "pod-replay").First(&signal).Error; err != nil {
		t.Fatal(err)
	}
	if signal.Count != 1 {
		t.Fatalf("exact replay incremented signal count to %d", signal.Count)
	}
	var profile models.PodRiskProfile
	if err := db.Where("cluster_id = ? AND pod_uid = ?", "c1", "pod-replay").First(&profile).Error; err != nil {
		t.Fatal(err)
	}
	if profile.RuntimeScore != 90 {
		t.Fatalf("exact replay changed runtime score: %d", profile.RuntimeScore)
	}
}

func TestPostRuntimeEventsV2_PersistsCanonicalFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.RuntimeEvent{}, &models.RuntimeSignal{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}, &models.PodRiskProfile{}, &models.PodCapability{}, &models.CapabilityMetadata{}, &models.PromotionRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	useRuntimeAgentPrincipal(r, "c1")
	r.POST("/api/v2/runtime/events", requireScopedRuntimeOwnership(db), PostRuntimeEventsV2Scoped(db))

	if err := db.Create(&models.Pod{UID: "pod-v2-1", ClusterID: "c1", Namespace: "ns", Name: "demo"}).Error; err != nil { t.Fatal(err) }
	payload := []map[string]interface{}{{
		"event_id":         "evt-1",
		"source_record_id": "1111111111111111111111111111111111111111111111111111111111111111",
		"observed_at":      "2026-03-26T00:00:00Z",
		"ingested_at":      "2026-03-26T00:00:01Z",
		"resolution_state": "resolved",
		"source": map[string]interface{}{
			"kind":      "falco",
			"sensor_id": "falco-1",
			"rule":      "Contact K8s API Server From Container",
		},
		"payload_hash": "sha256:abc",
		"payload_json": map[string]interface{}{"k": "v"},
		"pod": map[string]interface{}{
			"uid":       "pod-v2-1",
			"namespace": "ns",
			"name":      "demo",
			"node":      "n1",
		},
		"syscall":    "open",
		"target":     "/proc/1/root",
		"confidence": 0.9,
	}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/events", bytes.NewReader(body))
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
	if events[0].AgentID != "agent-a" || events[0].EventID != "evt-1" || events[0].ResolutionState != "resolved" || events[0].SourceKind != "falco" || events[0].PayloadHash != "sha256:abc" {
		t.Fatalf("event canonical fields missing: %+v", events[0])
	}
	if events[0].ObservedAt == nil || events[0].IngestedAt == nil {
		t.Fatalf("expected observed_at/ingested_at set: %+v", events[0])
	}
}

func TestPostRuntimeEventsV2_AcceptsFlattenedSourceFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.RuntimeEvent{}, &models.RuntimeSignal{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}, &models.PodRiskProfile{}, &models.PodCapability{}, &models.CapabilityMetadata{}, &models.PromotionRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	useRuntimeAgentPrincipal(r, "c1")
	r.POST("/api/v2/runtime/events", requireScopedRuntimeOwnership(db), PostRuntimeEventsV2Scoped(db))

	if err := db.Create(&models.Pod{UID: "pod-v2-2", ClusterID: "c1", Namespace: "ns", Name: "demo2"}).Error; err != nil { t.Fatal(err) }
	payload := []map[string]interface{}{{
		"event_id":         "evt-2",
		"source_record_id": "2222222222222222222222222222222222222222222222222222222222222222",
		"observed_at":      "2026-03-26T00:00:00Z",
		"ingested_at":      "2026-03-26T00:00:01Z",
		"resolution_state": "unresolved",

		// Flattened source fields (agent compatibility)
		"source_kind":      "falco",
		"source_sensor_id": "node-1",
		"source_rule":      "Some Falco rule",

		"payload_hash": "sha256:def",
		"payload_json": map[string]interface{}{"k": "v"},
		"pod": map[string]interface{}{
			"uid":       "pod-v2-2",
			"namespace": "ns",
			"name":      "demo2",
			"node":      "n1",
		},
		"syscall":    "connect",
		"target":     "dst=8.8.8.8:53 proto=udp dport=53",
		"confidence": 0.9,
	}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/events", bytes.NewReader(body))
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
	if events[0].EventID != "evt-2" || events[0].SourceKind != "falco" || events[0].SourceSensorID != "node-1" || events[0].SourceRule != "Some Falco rule" {
		t.Fatalf("flattened source canonical fields missing: %+v", events[0])
	}
}

func TestPostRuntimeEventsV2_PreservesPartialResolutionState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.RuntimeEvent{}, &models.RuntimeSignal{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}, &models.PodRiskProfile{}, &models.PodCapability{}, &models.CapabilityMetadata{}, &models.PromotionRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	useRuntimeAgentPrincipal(r, "c1")
	r.POST("/api/v2/runtime/events", requireScopedRuntimeOwnership(db), PostRuntimeEventsV2Scoped(db))

	if err := db.Create(&models.Pod{UID: "pod-v2-3", ClusterID: "c1", Namespace: "ns", Name: "demo3"}).Error; err != nil { t.Fatal(err) }
	payload := []map[string]interface{}{{
		"event_id":         "evt-3",
		"source_record_id": "3333333333333333333333333333333333333333333333333333333333333333",
		"observed_at":      "2026-03-26T01:00:00Z",
		"ingested_at":      "2026-03-26T01:00:01Z",
		"resolution_state": "partial",
		"source_kind":      "agent",
		"source_sensor_id": "sensor-1",
		"source_rule":      "rule-x",
		"payload_hash":     "sha256:xyz",
		"payload_json":     map[string]interface{}{"kind": "runtime.exec"},
		"pod": map[string]interface{}{
			"uid":       "pod-v2-3",
			"namespace": "ns",
			"name":      "demo3",
			"node":      "n1",
		},
		"syscall":    "execve",
		"target":     "/bin/sh",
		"confidence": 0.9,
	}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	var ev models.RuntimeEvent
	if err := db.Where("event_id = ?", "evt-3").First(&ev).Error; err != nil {
		t.Fatalf("query event: %v", err)
	}
	if ev.ResolutionState != "partial" {
		t.Fatalf("expected resolution_state=partial, got %q", ev.ResolutionState)
	}
}
