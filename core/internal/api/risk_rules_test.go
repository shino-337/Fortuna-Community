package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// TestGetRiskRulesList_EmptyDir verifies GET /risk-rules returns empty list when FORTUNA_RULES_DIR is unset.
func TestGetRiskRulesList_EmptyDir(t *testing.T) {
	orig := os.Getenv("FORTUNA_RULES_DIR")
	os.Unsetenv("FORTUNA_RULES_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("FORTUNA_RULES_DIR", orig)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risk-rules", nil)

	GetRiskRulesList(nil)(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Rules []interface{} `json:"rules"`
		Total int           `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if out.Total != 0 || len(out.Rules) != 0 {
		t.Errorf("expected empty rules when dir unset, got total=%d len=%d", out.Total, len(out.Rules))
	}
}

// TestGetRiskRulesList_WithYAML verifies GET /risk-rules returns rules when FORTUNA_RULES_DIR points to a dir with YAML.
func TestGetRiskRulesList_WithYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "rule.yaml")
	const yaml = `
id: test-rule-1
name: Test Rule
severity: high
description: A test rule for API
category: rbac
enabled: true
conditions:
  - type: expression
    expression: "true"
aggregation: AND
base_score: 7.0
`
	if err := os.WriteFile(f, []byte(yaml), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	orig := os.Getenv("FORTUNA_RULES_DIR")
	os.Setenv("FORTUNA_RULES_DIR", dir)
	defer func() {
		os.Setenv("FORTUNA_RULES_DIR", orig)
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risk-rules", nil)

	GetRiskRulesList(nil)(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Rules []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Severity    string `json:"severity"`
			Description string `json:"description"`
			Category    string `json:"category"`
			File        string `json:"file"`
			Enabled     bool   `json:"enabled"`
		} `json:"rules"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	if out.Total != 1 || len(out.Rules) != 1 {
		t.Fatalf("expected 1 rule, got total=%d len=%d", out.Total, len(out.Rules))
	}
	if out.Rules[0].ID != "test-rule-1" || out.Rules[0].Name != "Test Rule" || out.Rules[0].Severity != "high" {
		t.Errorf("expected id=test-rule-1 name=Test Rule severity=high, got id=%q name=%q severity=%q",
			out.Rules[0].ID, out.Rules[0].Name, out.Rules[0].Severity)
	}
	if !out.Rules[0].Enabled {
		t.Error("expected enabled true")
	}
}

// TestRiskRulesCRUD_DB verifies CRUD with risk_rules table (list from DB, create, get, update, delete).
func TestRiskRulesCRUD_DB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RiskRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)

	// List empty
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risk-rules", nil)
	GetRiskRulesList(db)(c)
	if w.Code != http.StatusOK {
		t.Errorf("list empty: expected 200, got %d", w.Code)
	}
	var listOut struct {
		Rules  []interface{} `json:"rules"`
		Total  int           `json:"total"`
		Source string        `json:"source"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listOut)
	if listOut.Source != "db" || listOut.Total != 0 {
		t.Errorf("expected source=db total=0, got source=%s total=%d", listOut.Source, listOut.Total)
	}

	// Create
	body := []byte(`{"id":"crud-rule-1","name":"CRUD Rule","severity":"high","description":"Test","category":"rbac","enabled":true,"conditions":[{"type":"expression","expression":"true"}],"aggregation":"AND","base_score":7}`)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/risk-rules", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	CreateRiskRule(db)(c)
	if w.Code != http.StatusCreated {
		t.Errorf("create: expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	// List one
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risk-rules", nil)
	GetRiskRulesList(db)(c)
	_ = json.Unmarshal(w.Body.Bytes(), &listOut)
	if listOut.Total != 1 {
		t.Errorf("expected 1 rule after create, got %d", listOut.Total)
	}

	// Get by id
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risk-rules/crud-rule-1", nil)
	c.Params = gin.Params{{Key: "id", Value: "crud-rule-1"}}
	GetRiskRuleByID(db)(c)
	if w.Code != http.StatusOK {
		t.Errorf("get by id: expected 200, got %d", w.Code)
	}

	// Update
	body = []byte(`{"id":"crud-rule-1","name":"CRUD Rule Updated","severity":"medium","description":"Updated","category":"rbac","enabled":false,"conditions":[{"type":"expression","expression":"true"}],"aggregation":"AND","base_score":5}`)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/risk-rules/crud-rule-1", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "crud-rule-1"}}
	UpdateRiskRule(db)(c)
	if w.Code != http.StatusOK {
		t.Errorf("update: expected 200, got %d", w.Code)
	}

	// Delete
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/v1/risk-rules/crud-rule-1", nil)
	c.Params = gin.Params{{Key: "id", Value: "crud-rule-1"}}
	DeleteRiskRule(db)(c)
	if w.Code != http.StatusOK {
		t.Errorf("delete: expected 200, got %d", w.Code)
	}

	// List empty again
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risk-rules", nil)
	GetRiskRulesList(db)(c)
	_ = json.Unmarshal(w.Body.Bytes(), &listOut)
	if listOut.Total != 0 {
		t.Errorf("expected 0 rules after delete, got %d", listOut.Total)
	}
}
