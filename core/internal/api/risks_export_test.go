package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func setupExportTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Minimal schema for ExportRisksCSV tests. Include risk_scores because handler supports scoreBin filter
	// using a subquery on risk_scores (even if tests don't set scoreBin).
	if err := db.AutoMigrate(&models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// One cluster and one pod so cluster filter works
	_ = db.Create(&models.Cluster{ID: "c1", Name: "cluster1"}).Error
	_ = db.Create(&models.Pod{UID: "pod-1", Name: "p1", Namespace: "default", ClusterID: "c1"}).Error
	return db
}

func TestExportRisksCSV_Empty(t *testing.T) {
	db := setupExportTestDB(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risks/export", nil)

	ExportRisksCSV(db)(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least header line")
	}
	if !strings.Contains(lines[0], "id") || !strings.Contains(lines[0], "title") {
		t.Errorf("expected CSV header with id, title; got %q", lines[0])
	}
}

func TestExportRisksCSV_WithInsights(t *testing.T) {
	db := setupExportTestDB(t)
	now := time.Now()
	_ = db.Create(&models.Insight{
		ResourceType: "Pod", ResourceUID: "pod-1", ResourceName: "p1", ResourceNamespace: "default",
		InsightType: "vulnerability", Severity: "high", Title: "CVE-2024-1", Description: "Test",
		Status: "active", DetectedAt: now, CreatedAt: now, UpdatedAt: now,
	}).Error

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Use export without cluster filter to avoid coupling test to NormalizeClusterID logic.
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/risks/export", nil)

	ExportRisksCSV(db)(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) < 2 {
		t.Errorf("expected header + 1 data row, got %d lines", len(lines))
	}
	if !strings.Contains(body, "CVE-2024-1") {
		t.Errorf("expected title in CSV; got %s", body)
	}
}
