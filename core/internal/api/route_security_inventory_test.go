package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api"
	"github.com/fortuna/core/internal/config"
	"github.com/fortuna/core/pkg/models"
)

func TestVerifyFortunaRouteSecurityContract_DefaultEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.AutoMigrate(&models.User{})
	cfg := &config.Config{JWTSecret: "x", AuthEnabled: false, IngestToken: "ingest"}
	r := gin.New()
	api.SetupRoutesWithCertManager(r, db, cfg, nil, nil, nil)
	opts := api.RouteVerifyOptions{}
	if err := api.VerifyFortunaRouteSecurityContract(r, opts); err != nil {
		t.Fatal(err)
	}
}

func TestAgentIngestRoutesAcceptIngestTokenWithoutJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{JWTSecret: "x", AuthEnabled: true, IngestToken: "ingest"}
	r := gin.New()
	api.SetupRoutesWithCertManager(r, db, cfg, nil, nil, nil)

	body := []byte(`{"podUid":"pod-1","clusterId":"cluster-1","namespace":"default","connections":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/pod-network-connections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Fortuna-Ingest-Token", "ingest")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected ingest token without JWT to be accepted, got status %d: %s", w.Code, w.Body.String())
	}
}

func TestSecurityActivityAppendOnly_SQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.SecurityActivityLog{}); err != nil {
		t.Fatal(err)
	}
	_ = db.Exec(`CREATE TRIGGER IF NOT EXISTS trg_sal_no_update BEFORE UPDATE ON security_activity_logs BEGIN SELECT RAISE(ROLLBACK, 'append-only'); END;`).Error
	_ = db.Exec(`CREATE TRIGGER IF NOT EXISTS trg_sal_no_delete BEFORE DELETE ON security_activity_logs BEGIN SELECT RAISE(ROLLBACK, 'append-only'); END;`).Error
	row := models.SecurityActivityLog{EventID: "e1", Action: "test", Result: "success", ActorUserID: 1}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&row).Update("result", "deny").Error; err == nil {
		t.Fatal("expected update to fail")
	}
	if err := db.Delete(&models.SecurityActivityLog{}, row.ID).Error; err == nil {
		t.Fatal("expected delete to fail")
	}
}
