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

func setupInsightContextTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.RiskRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestGetInsightContext_PodWithRule verifies context endpoint returns pod, cluster and rules when available.
func TestGetInsightContext_PodWithRule(t *testing.T) {
	db := setupInsightContextTestDB(t)
	now := time.Now()

	// Seed cluster, pod, rule and insight
	if err := db.Create(&models.Cluster{ID: "c1", Name: "cluster1"}).Error; err != nil {
		t.Fatalf("seed cluster: %v", err)
	}
	pod := models.Pod{
		UID:       "pod-uid-1",
		Name:      "pod-1",
		Namespace: "default",
		ClusterID: "c1",
	}
	if err := db.Create(&pod).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}
	rule := models.RiskRule{
		RuleID:      "esc-priv-pod",
		Name:        "Privileged pod",
		Category:    "pce",
		Severity:    "high",
		Description: "Pod is running privileged",
		Conditions:  "[]",
	}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	violatedJSON := `[{ "ruleId": "esc-priv-pod", "status": "triggered" }]`
	insight := models.Insight{
		ResourceType:      "Pod",
		ResourceUID:       pod.UID,
		ResourceName:      pod.Name,
		ResourceNamespace: pod.Namespace,
		InsightType:       "capability",
		Severity:          "high",
		Title:             "Privileged pod detected",
		Description:       "Pod is running privileged",
		Status:            "active",
		DetectedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
		ViolatedRules:     violatedJSON,
	}
	if err := db.Create(&insight).Error; err != nil {
		t.Fatalf("seed insight: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(insight.ID), 10)}}

	GetInsightContext(db)(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp InsightContextResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse json: %v", err)
	}

	if resp.Insight.ID != insight.ID {
		t.Errorf("expected insight id=%d, got %d", insight.ID, resp.Insight.ID)
	}
	if len(resp.Pods) != 1 || resp.Pods[0].UID != pod.UID {
		t.Errorf("expected 1 pod with uid=%s, got %+v", pod.UID, resp.Pods)
	}
	if resp.Cluster == nil || resp.Cluster.ID != "c1" {
		t.Errorf("expected cluster c1, got %#v", resp.Cluster)
	}
	if len(resp.Rules) != 1 || resp.Rules[0].RuleID != "esc-priv-pod" {
		t.Errorf("expected 1 rule esc-priv-pod, got %+v", resp.Rules)
	}
}

// TestGetInsightContext_NotFound verifies 404 when insight does not exist.
func TestGetInsightContext_NotFound(t *testing.T) {
	db := setupInsightContextTestDB(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "9999"}}

	GetInsightContext(db)(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

