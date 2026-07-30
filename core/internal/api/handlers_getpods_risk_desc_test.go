package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

// TestGetPods_RiskDesc_No500 reproduces GET /inventory/pods?sortBy=risk_desc without 500 (SQL must reference main table correctly).
func TestGetPods_RiskDesc_No500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Cluster{},
		&models.Pod{},
		&models.RiskScore{},
		&models.Insight{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cid := "cluster-a"
	if err := db.Create(&models.Cluster{ID: cid, Name: "c"}).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"high", "low"} {
		p := models.Pod{
			ClusterID: cid, UID: "uid-" + name, Name: name, Namespace: "default",
			ServiceAccount: "default",
		}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
	}
	rsHigh := models.RiskScore{
		ResourceType: "pod", ResourceUID: "uid-high", ClusterID: cid,
		ResourceName: "high", Namespace: "default",
		TotalScore: 90, ScorerVersion: "v3",
	}
	rsLow := models.RiskScore{
		ResourceType: "pod", ResourceUID: "uid-low", ClusterID: cid,
		ResourceName: "low", Namespace: "default",
		TotalScore: 10, ScorerVersion: "v3",
	}
	if err := db.Create(&rsHigh).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&rsLow).Error; err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{Role: models.RoleAdmin})
		c.Set(middleware.CtxNormalizedRole, models.RoleAdmin)
		c.Next()
	})
	r.GET("/inventory/pods", GetPods(db))

	req := httptest.NewRequest(http.MethodGet, "/inventory/pods?cluster="+cid+"&sortBy=risk_desc&page=1&pageSize=20", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		var er map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &er)
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Pods []map[string]interface{} `json:"pods"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Pods) < 2 {
		t.Fatalf("expected at least 2 pods, got %d", len(resp.Pods))
	}
	if resp.Pods[0]["name"] != "high" {
		t.Fatalf("expected highest score first, got %v", resp.Pods[0]["name"])
	}
}
