package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

func TestGetSBOMDetail_GroupsFindingsByNameAndVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.SBOM{}, &models.SBOMComponent{}, &models.CVEMatch{}, &models.MalwareMatch{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&models.Pod{UID: "pod-1", ClusterID: "c1", Name: "api", Namespace: "default", ServiceAccount: "default"}).Error; err != nil {
		t.Fatalf("seed pod: %v", err)
	}
	sbom := models.SBOM{ClusterID: "c1", PodUID: "pod-1", PodName: "api", Namespace: "default", ContainerName: "api",
		ImageName: "api", ImageTag: "1.0.0", Status: "complete", Version: 1}
	if err := db.Create(&sbom).Error; err != nil {
		t.Fatalf("seed sbom: %v", err)
	}
	comps := []models.SBOMComponent{
		{SBOMID: sbom.ID, ComponentName: "lodash", ComponentVersion: "4.17.20", ComponentType: "library"},
		{SBOMID: sbom.ID, ComponentName: "lodash", ComponentVersion: "4.17.21", ComponentType: "library"},
		{SBOMID: sbom.ID, ComponentName: "zlib", ComponentVersion: "1:1.2.13", ComponentType: "os-package"},
		{SBOMID: sbom.ID, ComponentName: "evil", ComponentVersion: "1.0.0", ComponentType: "library"},
	}
	if err := db.Create(&comps).Error; err != nil {
		t.Fatalf("seed components: %v", err)
	}
	matches := []models.CVEMatch{
		// Inserted out of display order: low, then critical (CVSS unknown), then two highs.
		{SBOMID: sbom.ID, PodUID: "pod-1", CVEID: "CVE-2020-0001", PackageName: "lodash", PackageVersion: "4.17.20", Severity: "low", CVSS: 3.1},
		{SBOMID: sbom.ID, PodUID: "pod-1", CVEID: "CVE-2020-0002", PackageName: "lodash", PackageVersion: "4.17.20", Severity: "critical", SeveritySource: "vendor",
			AdvisoryIDs: pq.StringArray{"GHSA-aaaa-bbbb-cccc"}},
		{SBOMID: sbom.ID, PodUID: "pod-1", CVEID: "CVE-2020-0003", PackageName: "lodash", PackageVersion: "4.17.20", Severity: "high", CVSS: 7.0},
		{SBOMID: sbom.ID, PodUID: "pod-1", CVEID: "CVE-2020-0004", PackageName: "lodash", PackageVersion: "4.17.20", Severity: "high", CVSS: 8.8},
		// Version stored differently from the component (no epoch): falls back to the name.
		{SBOMID: sbom.ID, PodUID: "pod-1", CVEID: "CVE-2022-0005", PackageName: "zlib", PackageVersion: "1.2.13", Severity: "medium", SeveritySource: "default"},
	}
	if err := db.Create(&matches).Error; err != nil {
		t.Fatalf("seed matches: %v", err)
	}
	if err := db.Create(&models.MalwareMatch{SBOMID: sbom.ID, PodUID: "pod-1", ComponentID: comps[3].ID, PackageName: "evil", PackageVersion: "1.0.0",
		Reason: "MALWARE", Confidence: 0.9, Sources: pq.StringArray{"aikido", "osv"}, AdvisoryIDs: pq.StringArray{"MAL-2025-1"}}).Error; err != nil {
		t.Fatalf("seed malware: %v", err)
	}

	router := gin.New()
	useAdminTestPrincipal(router)
	router.GET("/sbom/:uid", middleware.RequirePodUIDClusterScope(db, "uid"), GetSBOMDetail(db))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sbom/pod-1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var detail SBOMDetailDTO
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	byKey := map[string]SBOMComponentDTO{}
	for _, c := range detail.Components {
		byKey[c.Name+"@"+c.Version] = c
	}

	old := byKey["lodash@4.17.20"]
	if old.CveCount != 4 || len(old.Vulnerabilities) != 4 {
		t.Fatalf("lodash@4.17.20 cveCount=%d vulns=%d", old.CveCount, len(old.Vulnerabilities))
	}
	gotOrder := []string{}
	for _, v := range old.Vulnerabilities {
		gotOrder = append(gotOrder, v.ID)
	}
	wantOrder := []string{"CVE-2020-0002", "CVE-2020-0004", "CVE-2020-0003", "CVE-2020-0001"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("vulnerability order=%v want %v", gotOrder, wantOrder)
		}
	}
	if old.MaxSeverity != "critical" {
		t.Fatalf("maxSeverity=%q", old.MaxSeverity)
	}
	if v := old.Vulnerabilities[0]; v.SeveritySource != "vendor" || len(v.Advisories) != 1 || v.Advisories[0] != "GHSA-aaaa-bbbb-cccc" {
		t.Fatalf("first vulnerability=%#v", v)
	}
	if fixed := byKey["lodash@4.17.21"]; fixed.CveCount != 0 || len(fixed.Vulnerabilities) != 0 || fixed.MaxSeverity != "" {
		t.Fatalf("lodash@4.17.21 must not inherit findings of 4.17.20: %#v", fixed)
	}
	if z := byKey["zlib@1:1.2.13"]; z.CveCount != 1 || z.Vulnerabilities[0].SeveritySource != "default" {
		t.Fatalf("zlib name fallback: %#v", z)
	}

	if detail.VulnerablePackageCount != 2 {
		t.Fatalf("vulnerablePackageCount=%d", detail.VulnerablePackageCount)
	}
	want := map[string]int{"critical": 1, "high": 2, "medium": 1, "low": 1}
	for sev, n := range want {
		if detail.VulnerabilitySummary[sev] != n {
			t.Fatalf("summary=%v want %v", detail.VulnerabilitySummary, want)
		}
	}

	mal := byKey["evil@1.0.0"].MalwareMatch
	if mal == nil || len(mal.Sources) != 2 || mal.Sources[0] != "aikido" || len(mal.AdvisoryIDs) != 1 || mal.AdvisoryIDs[0] != "MAL-2025-1" {
		t.Fatalf("malware match=%#v", mal)
	}
}

func TestSortVulnerabilities_SeverityThenCVSS(t *testing.T) {
	vulns := []VulnerabilityDTO{
		{ID: "a", Severity: ""},
		{ID: "b", Severity: "low", CVSSScore: 3},
		{ID: "c", Severity: "medium", CVSSScore: 4},
		{ID: "d", Severity: "medium", CVSSScore: 6.5},
		{ID: "e", Severity: "critical"},
	}
	sortVulnerabilities(vulns)
	got := ""
	for _, v := range vulns {
		got += v.ID
	}
	if got != "edcba" {
		t.Fatalf("order=%s", got)
	}
}
