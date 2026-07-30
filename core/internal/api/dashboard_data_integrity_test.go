package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDashboardDataIntegrity_CatalogHealthCurrentMirrorCoverage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Agent{},
		&models.Cluster{},
		&models.Pod{},
		&models.SBOM{},
		&models.SBOMMatchRun{},
		&models.CVEMatch{},
		&models.CVE{},
		&models.PackageVulnerability{},
		&models.OSVPackage{},
		&models.MirrorState{},
		&models.CatalogGeneration{},
		&models.MalwarePackage{},
		&models.Insight{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Create(&models.MirrorState{Name: "osv", Version: 7}).Error; err != nil {
		t.Fatalf("seed mirror: %v", err)
	}
	activeCatalog := models.CatalogGeneration{CatalogType: "cve", SourceName: "osv", SourceDigest: "sha256:test", Status: "active", MirrorVersion: "7", StartedAt: nowForTest(), ActivatedAt: timePtrForTest()}
	if err := db.Create(&activeCatalog).Error; err != nil {
		t.Fatalf("seed catalog generation: %v", err)
	}
	if err := db.Create(&models.CatalogGeneration{CatalogType: "malware", SourceName: "aikido", SourceDigest: "sha256:malware", Status: "active", StartedAt: nowForTest(), ActivatedAt: timePtrForTest()}).Error; err != nil {
		t.Fatalf("seed malware generation: %v", err)
	}
	if err := db.Create(&models.CVE{CVEID: "GO-2023-2402", Severity: "MEDIUM", CVSSScore: 5, Source: "osv"}).Error; err != nil {
		t.Fatalf("seed cve: %v", err)
	}
	if err := db.Create(&models.PackageVulnerability{
		CVEID:               "GO-2023-2402",
		PackageName:         "golang.org/x/crypto",
		Ecosystem:           "go",
		VersionEndExcluding: "0.17.0",
	}).Error; err != nil {
		t.Fatalf("seed package vulnerability: %v", err)
	}
	if err := db.Create(&models.OSVPackage{VulnID: "GO-2023-2402", Ecosystem: "go", PackageName: "golang.org/x/crypto"}).Error; err != nil {
		t.Fatalf("seed osv package: %v", err)
	}
	if err := db.Create(&models.MalwarePackage{PackageName: "bad", Version: "1.0.0", Reason: "MALWARE", Source: "aikido-predictions"}).Error; err != nil {
		t.Fatalf("seed malware: %v", err)
	}
	if err := db.Create(&models.Pod{UID: "pod-active", ClusterID: "c1", Name: "api", Namespace: "default", ServiceAccount: "default"}).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}
	if err := db.Create(&models.SBOM{
		ID:            101,
		PodUID:        "pod-active",
		PodName:       "api",
		Namespace:     "default",
		ContainerName: "api",
		ImageName:     "api",
		ImageTag:      "1",
		Status:        "complete",
		Version:       2,
	}).Error; err != nil {
		t.Fatalf("seed active sbom: %v", err)
	}
	if err := db.Create(&models.SBOM{
		ID:            102,
		PodUID:        "pod-stale",
		PodName:       "old",
		Namespace:     "default",
		ContainerName: "old",
		ImageName:     "old",
		ImageTag:      "1",
		Status:        "complete",
		Version:       1,
	}).Error; err != nil {
		t.Fatalf("seed stale sbom: %v", err)
	}
	if err := db.Create(&models.SBOMMatchRun{SBOMID: 101, Version: 2, MirrorVersion: "7", CatalogGenerationID: activeCatalog.ID, Status: "succeeded"}).Error; err != nil {
		t.Fatalf("seed match run: %v", err)
	}
	if err := db.Create(&models.CVEMatch{
		SBOMID:         101,
		PodUID:         "pod-active",
		ContainerName:  "api",
		CVEID:          "GO-2023-2402",
		PackageName:    "golang.org/x/crypto",
		PackageVersion: "0.16.0",
		Severity:       "MEDIUM",
		MatchedBy:      "test",
	}).Error; err != nil {
		t.Fatalf("seed cve match: %v", err)
	}

	router := gin.New()
	router.GET("/health/dashboard-data-integrity", DashboardDataIntegrity(db))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/dashboard-data-integrity", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp DashboardDataIntegrityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CatalogHealth.Status != "healthy" {
		t.Fatalf("catalog status=%q", resp.CatalogHealth.Status)
	}
	if resp.CatalogHealth.MirrorVersion != "7" {
		t.Fatalf("mirror version=%q", resp.CatalogHealth.MirrorVersion)
	}
	if resp.CatalogHealth.ActiveSBOMs != 1 || resp.CatalogHealth.StaleSBOMs != 1 {
		t.Fatalf("sbom active/stale=%d/%d", resp.CatalogHealth.ActiveSBOMs, resp.CatalogHealth.StaleSBOMs)
	}
	if resp.CatalogHealth.ActiveSBOMsMatchedMirror != 1 || resp.CatalogHealth.ActiveSBOMsMissingMirrorMatch != 0 {
		t.Fatalf("mirror coverage matched/missing=%d/%d", resp.CatalogHealth.ActiveSBOMsMatchedMirror, resp.CatalogHealth.ActiveSBOMsMissingMirrorMatch)
	}
	if resp.CatalogHealth.ActiveSBOMsMatchedGeneration != 1 || resp.CatalogHealth.ActiveSBOMsMissingGenerationMatch != 0 {
		t.Fatalf("generation coverage matched/missing=%d/%d", resp.CatalogHealth.ActiveSBOMsMatchedGeneration, resp.CatalogHealth.ActiveSBOMsMissingGenerationMatch)
	}
	if resp.CatalogHealth.ActivePodCVEMatches != 1 || resp.CatalogHealth.StalePodCVEMatches != 0 {
		t.Fatalf("cve matches active/stale=%d/%d", resp.CatalogHealth.ActivePodCVEMatches, resp.CatalogHealth.StalePodCVEMatches)
	}
	if !containsAlert(resp.Alerts, "stale_sboms_present") {
		t.Fatalf("expected stale_sboms_present alert, got %#v", resp.Alerts)
	}
}

func TestDashboardDataIntegrity_CatalogHealthStaleWhenActiveSBOMNotMatchedCurrentMirror(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Pod{},
		&models.SBOM{},
		&models.SBOMMatchRun{},
		&models.CVE{},
		&models.PackageVulnerability{},
		&models.OSVPackage{},
		&models.MirrorState{},
		&models.CatalogGeneration{},
		&models.MalwarePackage{},
		&models.Insight{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Create(&models.MirrorState{Name: "osv", Version: 8}).Error
	_ = db.Create(&models.CatalogGeneration{CatalogType: "cve", SourceName: "osv", SourceDigest: "sha256:test", Status: "active", MirrorVersion: "8", StartedAt: nowForTest(), ActivatedAt: timePtrForTest()}).Error
	_ = db.Create(&models.CatalogGeneration{CatalogType: "malware", SourceName: "aikido", SourceDigest: "sha256:malware", Status: "active", StartedAt: nowForTest(), ActivatedAt: timePtrForTest()}).Error
	_ = db.Create(&models.CVE{CVEID: "GO-2023-2402", Severity: "MEDIUM", Source: "osv"}).Error
	_ = db.Create(&models.PackageVulnerability{CVEID: "GO-2023-2402", PackageName: "golang.org/x/crypto", Ecosystem: "go"}).Error
	_ = db.Create(&models.OSVPackage{VulnID: "GO-2023-2402", Ecosystem: "go", PackageName: "golang.org/x/crypto"}).Error
	_ = db.Create(&models.MalwarePackage{PackageName: "bad", Version: "1.0.0", Reason: "MALWARE"}).Error
	_ = db.Create(&models.Pod{UID: "pod-active", ClusterID: "c1", Name: "api", Namespace: "default", ServiceAccount: "default"}).Error
	_ = db.Create(&models.SBOM{ID: 201, PodUID: "pod-active", ImageName: "api", ImageTag: "1", Version: 3, Status: "complete"}).Error
	_ = db.Create(&models.SBOMMatchRun{SBOMID: 201, Version: 3, MirrorVersion: "7", Status: "succeeded"}).Error

	router := gin.New()
	router.GET("/health/dashboard-data-integrity", DashboardDataIntegrity(db))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/dashboard-data-integrity", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp DashboardDataIntegrityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CatalogHealth.Status != "stale" {
		t.Fatalf("catalog status=%q", resp.CatalogHealth.Status)
	}
	if resp.CatalogHealth.ActiveSBOMsMissingMirrorMatch != 1 {
		t.Fatalf("missing current mirror matches=%d", resp.CatalogHealth.ActiveSBOMsMissingMirrorMatch)
	}
	if resp.CatalogHealth.ActiveSBOMsMissingGenerationMatch != 1 {
		t.Fatalf("missing active generation matches=%d", resp.CatalogHealth.ActiveSBOMsMissingGenerationMatch)
	}
	if !containsAlert(resp.Alerts, "catalog_stale") {
		t.Fatalf("expected catalog_stale alert, got %#v", resp.Alerts)
	}
}

func TestDashboardDataIntegrity_CatalogHealthStaleWhenMirrorMatchedButGenerationMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Pod{},
		&models.SBOM{},
		&models.SBOMMatchRun{},
		&models.CVE{},
		&models.PackageVulnerability{},
		&models.OSVPackage{},
		&models.MirrorState{},
		&models.CatalogGeneration{},
		&models.MalwarePackage{},
		&models.Insight{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	activeCatalog := models.CatalogGeneration{CatalogType: "cve", SourceName: "osv", SourceDigest: "sha256:test", Status: "active", MirrorVersion: "8", StartedAt: nowForTest(), ActivatedAt: timePtrForTest()}
	_ = db.Create(&models.MirrorState{Name: "osv", Version: 8}).Error
	_ = db.Create(&activeCatalog).Error
	_ = db.Create(&models.CatalogGeneration{CatalogType: "malware", SourceName: "aikido", SourceDigest: "sha256:malware", Status: "active", StartedAt: nowForTest(), ActivatedAt: timePtrForTest()}).Error
	_ = db.Create(&models.CVE{CVEID: "GO-2023-2402", Severity: "MEDIUM", Source: "osv"}).Error
	_ = db.Create(&models.PackageVulnerability{CVEID: "GO-2023-2402", PackageName: "golang.org/x/crypto", Ecosystem: "go"}).Error
	_ = db.Create(&models.OSVPackage{VulnID: "GO-2023-2402", Ecosystem: "go", PackageName: "golang.org/x/crypto"}).Error
	_ = db.Create(&models.MalwarePackage{PackageName: "bad", Version: "1.0.0", Reason: "MALWARE"}).Error
	_ = db.Create(&models.Pod{UID: "pod-active", ClusterID: "c1", Name: "api", Namespace: "default", ServiceAccount: "default"}).Error
	_ = db.Create(&models.SBOM{ID: 301, PodUID: "pod-active", ImageName: "api", ImageTag: "1", Version: 3, Status: "complete"}).Error
	_ = db.Create(&models.SBOMMatchRun{SBOMID: 301, Version: 3, MirrorVersion: "8", Status: "succeeded"}).Error

	router := gin.New()
	router.GET("/health/dashboard-data-integrity", DashboardDataIntegrity(db))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/dashboard-data-integrity", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp DashboardDataIntegrityResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CatalogHealth.Status != "stale" {
		t.Fatalf("catalog status=%q", resp.CatalogHealth.Status)
	}
	if resp.CatalogHealth.ActiveSBOMsMissingMirrorMatch != 0 {
		t.Fatalf("missing mirror matches=%d", resp.CatalogHealth.ActiveSBOMsMissingMirrorMatch)
	}
	if resp.CatalogHealth.ActiveSBOMsMissingGenerationMatch != 1 {
		t.Fatalf("missing active generation matches=%d", resp.CatalogHealth.ActiveSBOMsMissingGenerationMatch)
	}
	if !containsAlert(resp.Alerts, "catalog_generation_match_missing") {
		t.Fatalf("expected catalog_generation_match_missing alert, got %#v", resp.Alerts)
	}
}

func containsAlert(alerts []string, prefix string) bool {
	for _, alert := range alerts {
		if strings.HasPrefix(alert, prefix) {
			return true
		}
	}
	return false
}

func nowForTest() time.Time {
	return time.Now()
}

func timePtrForTest() *time.Time {
	t := time.Now()
	return &t
}
