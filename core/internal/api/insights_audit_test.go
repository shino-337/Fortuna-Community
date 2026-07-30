package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func setupInsightAuditTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate audit_logs: %v", err)
	}
	return db
}

func TestCreateInsightAuditLog_NoUserInContext(t *testing.T) {
	db := setupInsightAuditTestDB(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	// No userID or username set

	createInsightAuditLog(db, c, "acknowledge", "123", "{}")

	var count int64
	db.Model(&models.AuditLog{}).Where("resource = ? AND resource_id = ? AND action = ?", "insight", "123", "acknowledge").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 audit log, got %d", count)
	}
	var entry models.AuditLog
	db.Where("resource = ? AND resource_id = ?", "insight", "123").First(&entry)
	if entry.User != "system" {
		t.Errorf("expected username system when not in context, got %q", entry.User)
	}
	if entry.UserID != 0 {
		t.Errorf("expected userID 0 when not in context, got %d", entry.UserID)
	}
}

func TestCreateInsightAuditLog_WithUserInContext(t *testing.T) {
	db := setupInsightAuditTestDB(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set("userID", uint(42))
	c.Set("username", "alice")

	createInsightAuditLog(db, c, "resolve", "456", `{"resolution":"fixed"}`)

	var entry models.AuditLog
	db.Where("resource = ? AND resource_id = ? AND action = ?", "insight", "456", "resolve").First(&entry)
	if entry.UserID != 42 {
		t.Errorf("expected userID 42, got %d", entry.UserID)
	}
	if entry.User != "alice" {
		t.Errorf("expected username alice, got %q", entry.User)
	}
	if entry.Details != `{"resolution":"fixed"}` {
		t.Errorf("expected details with resolution, got %q", entry.Details)
	}
}
