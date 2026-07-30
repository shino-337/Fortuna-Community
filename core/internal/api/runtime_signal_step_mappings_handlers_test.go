package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func TestRuntimeSignalStepMappingsCRUD(t *testing.T) {
	origMode := os.Getenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE")
	_ = os.Setenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE", "ENABLED")
	defer func() { _ = os.Setenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE", origMode) }()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeSignalStepMapping{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	gin.SetMode(gin.TestMode)

	// create
	body := []byte(`{"signalType":"PROC_EXEC","stepId":"EXEC","enabled":true}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/runtime/signal-step-mappings", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	UpsertRuntimeSignalStepMapping(db)(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("create expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var created models.RuntimeSignalStepMapping
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("parse created: %v", err)
	}
	if created.SignalType != "PROC_EXEC" || created.StepID != "EXEC" {
		t.Fatalf("expected uppercase normalized mapping, got %+v", created)
	}

	// list
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/runtime/signal-step-mappings", nil)
	GetRuntimeSignalStepMappings(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d", w.Code)
	}

	// disable
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/runtime/signal-step-mappings/1/enabled", bytes.NewReader([]byte(`{"enabled":false}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	SetRuntimeSignalStepMappingEnabled(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("patch expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var patched models.RuntimeSignalStepMapping
	if err := json.Unmarshal(w.Body.Bytes(), &patched); err != nil {
		t.Fatalf("parse patched: %v", err)
	}
	if patched.Enabled {
		t.Fatalf("expected mapping disabled")
	}
}

func TestRuntimeSignalStepMappings_VerifyAuditChainTamperFails(t *testing.T) {
	origMode := os.Getenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE")
	_ = os.Setenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE", "ENABLED")
	defer func() { _ = os.Setenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE", origMode) }()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeSignalStepMapping{}, &models.AuditLog{}, &models.AuditAnchor{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("username", "admin")
	c.Set("userID", uint(1))
	c.Set("auth_source", "jwt")
	c.Set("scope_source", "server_role_map")
	body := []byte(`{"signalType":"PROC_EXEC","stepId":"EXEC","enabled":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/runtime/signal-step-mappings", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	UpsertRuntimeSignalStepMapping(db)(c)
	if err := VerifyRuntimeMappingAuditChain(db, 0); err != nil {
		t.Fatalf("expected valid chain, got %v", err)
	}
	var row models.AuditLog
	if err := db.Where("resource = ?", "runtime_signal_step_mapping").First(&row).Error; err != nil {
		t.Fatalf("load audit: %v", err)
	}
	row.Details = `{"tampered":true}`
	if err := db.Save(&row).Error; err != nil {
		t.Fatalf("tamper save: %v", err)
	}
	if err := VerifyRuntimeMappingAuditChain(db, 0); err == nil {
		t.Fatalf("expected chain verification failure after tamper")
	}
}

func TestRuntimeSignalStepMappings_DryRunAndEffectiveFrom(t *testing.T) {
	origMode := os.Getenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE")
	_ = os.Setenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE", "DRY_RUN")
	defer func() { _ = os.Setenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE", origMode) }()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeSignalStepMapping{}, &models.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	eff := time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)
	body := []byte(`{"signalType":"DNS_QUERY","stepId":"RECON","enabled":true,"effectiveFrom":"` + eff + `"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/runtime/signal-step-mappings", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	UpsertRuntimeSignalStepMapping(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("dry-run expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var count int64
	_ = db.Model(&models.RuntimeSignalStepMapping{}).Count(&count).Error
	if count != 0 {
		t.Fatalf("expected no row persisted in DRY_RUN, got %d", count)
	}
}

func TestRuntimeSignalStepMappings_PatchRejectsIdentityFields(t *testing.T) {
	origMode := os.Getenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE")
	_ = os.Setenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE", "ENABLED")
	defer func() { _ = os.Setenv("FORTUNA_RUNTIME_MAPPING_WRITE_MODE", origMode) }()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeSignalStepMapping{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&models.RuntimeSignalStepMapping{SignalType: "PROC_EXEC", StepID: "EXEC", Enabled: true}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/runtime/signal-step-mappings/1/enabled", bytes.NewReader([]byte(`{"enabled":false,"signalType":"DNS_QUERY"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	SetRuntimeSignalStepMappingEnabled(db)(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on immutable identity patch, got %d body=%s", w.Code, w.Body.String())
	}
}

