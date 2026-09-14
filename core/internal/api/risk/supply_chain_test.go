package risk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSupplyChainCorrelation_DefaultScopesToActivePods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newSupplyChainTestDB(t)
	seedSupplyChainRows(t, db)

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user", &models.User{Role: models.RoleAdmin}) })
	router.GET("/risk/analytics/supply-chain", GetSupplyChainCorrelation(db))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/risk/analytics/supply-chain?entries=true", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp SupplyChainSummary
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Scope != "current" {
		t.Fatalf("scope=%q, want current", resp.Scope)
	}
	if resp.TotalPods != 1 || len(resp.Entries) != 1 {
		t.Fatalf("default response should include only active pod, pods=%d entries=%d", resp.TotalPods, len(resp.Entries))
	}
	if resp.Entries[0].PodUID != "pod-active" {
		t.Fatalf("entry pod=%q, want pod-active", resp.Entries[0].PodUID)
	}
}

func TestSupplyChainCorrelation_CanIncludeHistoricalSBOMs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newSupplyChainTestDB(t)
	seedSupplyChainRows(t, db)

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user", &models.User{Role: models.RoleAdmin}) })
	router.GET("/risk/analytics/supply-chain", GetSupplyChainCorrelation(db))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/risk/analytics/supply-chain?entries=true&includeHistorical=true", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp SupplyChainSummary
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Scope != "current_and_historical" {
		t.Fatalf("scope=%q, want current_and_historical", resp.Scope)
	}
	if resp.TotalPods != 2 || len(resp.Entries) != 2 {
		t.Fatalf("historical response should include both pods, pods=%d entries=%d", resp.TotalPods, len(resp.Entries))
	}
}

func newSupplyChainTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.SBOM{}, &models.CVEMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedSupplyChainRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&models.Pod{UID: "pod-active", ClusterID: "c1", Name: "api", Namespace: "default", NodeName: "node-a"}).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}
	sboms := []models.SBOM{
		{ID: 101, PodUID: "pod-active", PodName: "api", Namespace: "default", ImageName: "repo/api", ImageTag: "1", Status: "complete"},
		{ID: 102, PodUID: "pod-stale", PodName: "old", Namespace: "default", ImageName: "repo/old", ImageTag: "1", Status: "complete"},
	}
	if err := db.Create(&sboms).Error; err != nil {
		t.Fatalf("seed sboms: %v", err)
	}
	matches := []models.CVEMatch{
		{SBOMID: 101, PodUID: "pod-active", CVEID: "CVE-2026-0001", Severity: "HIGH", CVSS: 8.1, PackageName: "openssl", PackageVersion: "1.0.0"},
		{SBOMID: 102, PodUID: "pod-stale", CVEID: "CVE-2026-0002", Severity: "HIGH", CVSS: 8.0, PackageName: "curl", PackageVersion: "1.0.0"},
	}
	if err := db.Create(&matches).Error; err != nil {
		t.Fatalf("seed cve matches: %v", err)
	}
}
