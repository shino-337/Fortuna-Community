package policy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func newPolicyTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.PolicyTemplate{}, &models.PolicyInstance{}); err != nil {
		t.Fatal(err)
	}
	h := NewPolicyHandler(db)
	r := gin.New()
	r.POST("/templates", h.CreateTemplate)
	r.PUT("/templates/:templateId/:version", h.UpdateTemplate)
	r.DELETE("/templates/:templateId/:version", h.DeleteTemplate)
	r.POST("/instances", h.CreateInstance)
	r.PUT("/instances/:instanceName", h.UpdateInstance)
	r.DELETE("/instances/:instanceName", h.DeleteInstance)
	return r, db
}

func doJSON(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var testTemplate = map[string]any{
	"templateId": "t1", "version": "1.0.0", "name": "T1", "category": "security",
	"defaultSeverity": "high", "celExpression": "true", "description": "orig",
}

func TestCreatedTemplateIsUserTemplateAndDeletable(t *testing.T) {
	r, db := newPolicyTestRouter(t)
	tpl := map[string]any{"isSystem": true}
	for k, v := range testTemplate {
		tpl[k] = v
	}
	if w := doJSON(t, r, http.MethodPost, "/templates", tpl); w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var stored models.PolicyTemplate
	db.First(&stored, "template_id = ?", "t1")
	if stored.IsSystem {
		t.Fatal("API-created template stored as system template")
	}
	if w := doJSON(t, r, http.MethodDelete, "/templates/t1/1.0.0", nil); w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}
}

func TestCreateTemplateRejectsInvalidEnums(t *testing.T) {
	r, _ := newPolicyTestRouter(t)
	for field, value := range map[string]string{"category": "nope", "defaultAction": "remediate", "defaultSeverity": "huge"} {
		tpl := map[string]any{}
		for k, v := range testTemplate {
			tpl[k] = v
		}
		tpl[field] = value
		if w := doJSON(t, r, http.MethodPost, "/templates", tpl); w.Code != http.StatusBadRequest {
			t.Errorf("%s=%q: got %d, want 400", field, value, w.Code)
		}
	}
}

func TestUpdateTemplateKeepsOmittedFields(t *testing.T) {
	r, db := newPolicyTestRouter(t)
	doJSON(t, r, http.MethodPost, "/templates", testTemplate)
	if w := doJSON(t, r, http.MethodPut, "/templates/t1/1.0.0", map[string]any{"rationale": "why"}); w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	var stored models.PolicyTemplate
	db.First(&stored, "template_id = ?", "t1")
	if stored.Description != "orig" || stored.Rationale != "why" {
		t.Fatalf("description=%q rationale=%q", stored.Description, stored.Rationale)
	}
}

func TestPolicyInstanceCreateAndPartialUpdate(t *testing.T) {
	r, db := newPolicyTestRouter(t)
	doJSON(t, r, http.MethodPost, "/templates", testTemplate)

	w := doJSON(t, r, http.MethodPost, "/instances", map[string]any{
		"templateId": "t1", "templateVersion": "1.0.0", "instanceName": "i1",
		"resourceTypes": []string{"Pod"}, "enabled": false,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var stored models.PolicyInstance
	db.First(&stored, "instance_name = ?", "i1")
	if stored.Enabled || !stored.RemediationDryRun {
		t.Fatalf("enabled=%v dryRun=%v, want false/true", stored.Enabled, stored.RemediationDryRun)
	}
	if stored.Action != "alert" || stored.Severity != "high" {
		t.Fatalf("action=%q severity=%q, want template defaults", stored.Action, stored.Severity)
	}

	if w := doJSON(t, r, http.MethodPut, "/instances/i1", map[string]any{"enabled": true}); w.Code != http.StatusOK {
		t.Fatalf("toggle: %d %s", w.Code, w.Body)
	}
	stored = models.PolicyInstance{}
	db.First(&stored, "instance_name = ?", "i1")
	if !stored.Enabled || stored.Action != "alert" || stored.Severity != "high" ||
		len(stored.ResourceTypes) != 1 || stored.ResourceTypes[0] != "Pod" {
		t.Fatalf("partial update lost fields: %+v", stored)
	}

	if w := doJSON(t, r, http.MethodPut, "/instances/i1", map[string]any{"action": "drop"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid action: got %d, want 400", w.Code)
	}
}

func TestDeletedNamesAreHandled(t *testing.T) {
	r, _ := newPolicyTestRouter(t)
	doJSON(t, r, http.MethodPost, "/templates", testTemplate)
	inst := map[string]any{"templateId": "t1", "templateVersion": "1.0.0", "instanceName": "i1"}
	for i := 0; i < 2; i++ {
		if w := doJSON(t, r, http.MethodPost, "/instances", inst); w.Code != http.StatusCreated {
			t.Fatalf("create #%d: %d %s", i, w.Code, w.Body)
		}
		if w := doJSON(t, r, http.MethodDelete, "/instances/i1", nil); w.Code != http.StatusOK {
			t.Fatalf("delete #%d: %d %s", i, w.Code, w.Body)
		}
	}

	if w := doJSON(t, r, http.MethodDelete, "/templates/t1/1.0.0", nil); w.Code != http.StatusOK {
		t.Fatalf("delete template: %d %s", w.Code, w.Body)
	}
	if w := doJSON(t, r, http.MethodPost, "/templates", testTemplate); w.Code != http.StatusConflict {
		t.Fatalf("recreate deleted template version: got %d, want 409", w.Code)
	}
}
