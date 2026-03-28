package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func TestBuildInsightEvidenceRefs_ParsesEvidenceAndRules(t *testing.T) {
	in := models.Insight{
		Evidence: `{
			"event_id": 123,
			"evidence_fact_ids": ["f1","f2"],
			"signal_type": "EXTERNAL_EGRESS",
			"incident_type": "RECON_BURST",
			"capability_id": "ESC_RUNTIME_ACTIVE"
		}`,
		ViolatedRules: `[{"ruleId":"runtime-signals-recent"}]`,
	}
	refs := buildInsightEvidenceRefs(in)
	if len(refs.EventIDs) != 1 || refs.EventIDs[0] != "123" {
		t.Fatalf("event refs mismatch: %+v", refs.EventIDs)
	}
	if len(refs.FactIDs) != 2 {
		t.Fatalf("fact refs mismatch: %+v", refs.FactIDs)
	}
	if len(refs.SignalTypes) != 1 || refs.SignalTypes[0] != "EXTERNAL_EGRESS" {
		t.Fatalf("signal refs mismatch: %+v", refs.SignalTypes)
	}
	if len(refs.IncidentTypes) != 1 || refs.IncidentTypes[0] != "RECON_BURST" {
		t.Fatalf("incident refs mismatch: %+v", refs.IncidentTypes)
	}
	if len(refs.CapabilityIDs) != 1 || refs.CapabilityIDs[0] != "ESC_RUNTIME_ACTIVE" {
		t.Fatalf("capability refs mismatch: %+v", refs.CapabilityIDs)
	}
	if len(refs.RuleIDs) != 1 || refs.RuleIDs[0] != "runtime-signals-recent" {
		t.Fatalf("rule refs mismatch: %+v", refs.RuleIDs)
	}
}

func TestGetInsight_IncludesEvidenceRefs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Insight{}, &models.Pod{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	ins := models.Insight{
		ResourceType:      "Pod",
		ResourceUID:       "pod-1",
		ResourceName:      "p1",
		ResourceNamespace: "ns",
		InsightType:       "runtime",
		Severity:          "high",
		Title:             "runtime finding",
		Description:       "desc",
		Status:            "active",
		DetectedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
		Evidence:          `{"signal_type":"EXTERNAL_EGRESS","evidence_fact_ids":["f1"]}`,
		ViolatedRules:     `[{"ruleId":"runtime-signals-recent"}]`,
	}
	if err := db.Create(&ins).Error; err != nil {
		t.Fatalf("seed insight: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risk/insights/"+strconv.FormatUint(uint64(ins.ID), 10), nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(ins.ID), 10)}}
	GetInsight(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	er, ok := out["evidence_refs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected evidence_refs object, got: %#v", out["evidence_refs"])
	}
	if _, ok := er["signalTypes"]; !ok {
		t.Fatalf("missing signalTypes in evidence_refs: %#v", er)
	}
	chain, ok := out["explanation_chain"].([]interface{})
	if !ok || len(chain) < 2 {
		t.Fatalf("expected explanation_chain with multiple layers, got: %#v", out["explanation_chain"])
	}
}

func TestGetInsight_EnrichQueryLoadsFacts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Insight{}, &models.Pod{}, &models.RuntimeBehaviorFact{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	podUID := "pod-enr-1"
	if err := db.Create(&models.Pod{
		UID: podUID, Name: "p", Namespace: "ns", ClusterID: "c1",
		ServiceAccount: "default", Containers: "[]",
	}).Error; err != nil {
		t.Fatal(err)
	}
	ins := models.Insight{
		ResourceType: "Pod", ResourceUID: podUID, ResourceName: "p", ResourceNamespace: "ns",
		InsightType: "runtime", Severity: "high", Title: "t", Status: "active", DetectedAt: now,
		Evidence: `{"evidence_fact_ids":["fact-a"]}`,
	}
	if err := db.Create(&ins).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.RuntimeBehaviorFact{
		FactID: "fact-a", PodUID: podUID, Namespace: "ns", FactType: "NETWORK_CONNECT",
		Domain: "network", Attributes: "{}", SourceRef: "{}", ObservedAt: now, CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risk/insights/"+strconv.FormatUint(uint64(ins.ID), 10)+"?enrich=1", nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(ins.ID), 10)}}
	GetInsight(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d %s", w.Code, w.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	enr, ok := out["enriched_refs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected enriched_refs: %#v", out["enriched_refs"])
	}
	facts, ok := enr["facts"].([]interface{})
	if !ok || len(facts) != 1 {
		t.Fatalf("expected one enriched fact: %#v", enr)
	}
}

func TestBuildExplanationChain_FromRefs(t *testing.T) {
	refs := buildInsightEvidenceRefs(models.Insight{
		Evidence:      `{"event_id":1,"evidence_fact_ids":["f1"],"signal_type":"S","incident_type":"I","capability_id":"C"}`,
		ViolatedRules: `[{"ruleId":"r1"}]`,
	})
	ch := buildExplanationChain(refs)
	if len(ch) < 5 {
		t.Fatalf("expected chain steps, got %+v", ch)
	}
	if ch[0].Layer != "runtime_event" || ch[len(ch)-1].Layer != "risk_rule" {
		t.Fatalf("unexpected order: %+v", ch)
	}
}

