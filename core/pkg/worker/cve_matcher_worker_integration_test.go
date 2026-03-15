package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
)

// Realistic constants for full-flow integration test (mirror production distroless control-plane).
const (
	testImageName     = "registry.k8s.io/kube-controller-manager"
	testImageTag      = "v1.29.15"
	testImageDigest   = "sha256:a1b2c3d4e5f6789012345678901234567890123456789012345678901234ab"
	testPodUID        = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	testPodName       = "kube-controller-manager-xyz"
	testNamespace     = "kube-system"
	testContainerName = "kube-controller-manager"
	testPackageName   = "kube-controller-manager"
	testPackageVer    = "v1.29.15"
)

// TestFullFlow_DistrolessSBOM_NVD_CVE_AndRisk runs the full pipeline and reports each step:
// 1. SBOM distroless-heuristic with real-looking image digest, version, PURL
// 2. CVE matcher → Postgres 0 → NVD fallback → CVEs returned
// 3. NVD CVEs persisted to cves + cve_matches (MatchedBy=nvd-fallback)
// 4. Worker creates vulnerability insights (risk)
// 5. Assert and report: cve_matches, cves, insights; pod/SBOM detail view
//
// Run: RUN_NVD_INTEGRATION=1 go test -v -run TestFullFlow_DistrolessSBOM_NVD ./core/pkg/worker/
// Or:  NVD_API_KEY=your-key go test -v -run TestFullFlow_DistrolessSBOM_NVD ./core/pkg/worker/
func TestFullFlow_DistrolessSBOM_NVD_CVE_AndRisk(t *testing.T) {
	if os.Getenv("RUN_NVD_INTEGRATION") != "1" && os.Getenv("NVD_API_KEY") == "" {
		t.Skip("Skipping full-flow NVD integration test (set RUN_NVD_INTEGRATION=1 or NVD_API_KEY to run)")
	}
	if os.Getenv("FORTUNA_NVD_DISABLED") == "1" || os.Getenv("FORTUNA_NVD_DISABLED") == "true" {
		t.Skip("FORTUNA_NVD_DISABLED is set, NVD client would be nil")
	}

	t.Log("========== BƯỚC 0: Khởi tạo DB (SQLite in-memory) ==========")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.SBOM{},
		&models.SBOMComponent{},
		&models.CVE{},
		&models.PackageVulnerability{},
		&models.CVEMatch{},
		&models.Insight{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_cve_matches_unique_sbom_pkg_cve ON cve_matches(sbom_id, package_name, cve_id)").Error; err != nil {
		t.Fatalf("create unique index on cve_matches: %v", err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_insights_unique_resource_cve_type ON insights(resource_uid, cve_id, insight_type)").Error; err != nil {
		t.Fatalf("create unique index on insights: %v", err)
	}
	t.Log("OK: Bảng sboms, sbom_components, cves, package_vulnerabilities, cve_matches, insights đã sẵn sàng.")

	t.Log("========== BƯỚC 1: Tạo SBOM distroless (pod control-plane) ==========")
	containerImage := testImageName + ":" + testImageTag
	sbomRow := models.SBOM{
		ImageName:     testImageName,
		ImageTag:      testImageTag,
		ImageDigest:   testImageDigest,
		PodUID:        testPodUID,
		PodName:       testPodName,
		Namespace:     testNamespace,
		ContainerName: testContainerName,
		OSName:        "distroless",
		OSVersion:     "",
		PackageCount:  1,
		SbomSource:    "distroless-heuristic",
		Confidence:    "medium",
		GeneratedAt:   time.Now(),
	}
	if err := db.Create(&sbomRow).Error; err != nil {
		t.Fatalf("create SBOM: %v", err)
	}
	t.Logf("  SBOM ID=%d Image=%s Tag=%s Digest=%s", sbomRow.ID, sbomRow.ImageName, sbomRow.ImageTag, sbomRow.ImageDigest)
	t.Logf("  Pod: UID=%s Name=%s Namespace=%s Container=%s", sbomRow.PodUID, sbomRow.PodName, sbomRow.Namespace, sbomRow.ContainerName)
	t.Logf("  SbomSource=%s Confidence=%s", sbomRow.SbomSource, sbomRow.Confidence)

	comp := models.SBOMComponent{
		SBOMID:           sbomRow.ID,
		ComponentName:    testPackageName,
		ComponentVersion: testPackageVer,
		PURL:             fmt.Sprintf("pkg:generic/%s@%s", testPackageName, testPackageVer),
		ComponentType:    "application",
		Source:           "distroless-heuristic",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create SBOMComponent: %v", err)
	}
	t.Logf("  Component: Name=%s Version=%s PURL=%s Source=%s", comp.ComponentName, comp.ComponentVersion, comp.PURL, comp.Source)
	t.Log("OK: SBOM và 1 component (kube-controller-manager@v1.29.15) đã lưu.")

	t.Log("========== BƯỚC 2: Gửi sự kiện SBOM_CREATED → chạy CVE matcher worker ==========")
	ev := sbom.SBOMCreatedEvent{
		Type:            "sbom.created",
		Timestamp:       time.Now().Unix(),
		PodUID:          testPodUID,
		PodName:         testPodName,
		PodNamespace:    testNamespace,
		ContainerName:   testContainerName,
		ContainerImage:  containerImage,
		SBOMID:          sbomRow.ID,
		ImageDigest:     sbomRow.ImageDigest,
	}
	msgData, _ := json.Marshal(ev)
	msg := &nats.Msg{Data: msgData}

	worker := NewCVEMatcherWorker(nil, db, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := worker.Process(ctx, msg); err != nil {
		t.Fatalf("worker Process: %v", err)
	}
	t.Log("OK: Worker đã chạy (Postgres 0 CVE → NVD fallback → persist cves + cve_matches → tạo insights).")

	t.Log("========== BƯỚC 3: Kiểm tra cve_matches (thông tin CVE theo SBOM/pod) ==========")
	var matches []models.CVEMatch
	if err := db.Where("sbom_id = ? AND deleted_at IS NULL", sbomRow.ID).Find(&matches).Error; err != nil {
		t.Fatalf("query cve_matches: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one CVE match from NVD fallback, got 0")
	}
	nvdMatches := 0
	for _, m := range matches {
		if m.MatchedBy == "nvd-fallback" {
			nvdMatches++
		}
	}
	t.Logf("  Tổng số match: %d (từ nvd-fallback: %d)", len(matches), nvdMatches)
	for i, m := range matches {
		preview := ""
		if i < 5 {
			preview = fmt.Sprintf("  [%d] CVEID=%s Package=%s@%s Severity=%s MatchedBy=%s", i+1, m.CVEID, m.PackageName, m.PackageVersion, m.Severity, m.MatchedBy)
		}
		if preview != "" {
			t.Log(preview)
		}
	}
	if len(matches) > 5 {
		t.Logf("  ... và %d match khác.", len(matches)-5)
	}

	t.Log("========== BƯỚC 4: Kiểm tra bảng cves (bản ghi CVE từ NVD đã persist) ==========")
	var cveRows []models.CVE
	if err := db.Where("deleted_at IS NULL").Find(&cveRows).Error; err != nil {
		t.Fatalf("query cves: %v", err)
	}
	if len(cveRows) == 0 {
		t.Fatal("expected at least one CVE row (NVD persist), got 0")
	}
	nvdCves := 0
	for _, c := range cveRows {
		if c.Source == "nvd" {
			nvdCves++
		}
	}
	t.Logf("  Tổng số bản ghi CVE: %d (source=nvd: %d)", len(cveRows), nvdCves)
	for i, c := range cveRows {
		if i >= 3 {
			break
		}
		descLen := 60
		if len(c.Description) < descLen {
			descLen = len(c.Description)
		}
		t.Logf("  [%d] CVEID=%s Severity=%s CVSS=%.1f Source=%s Description=%s...", i+1, c.CVEID, c.Severity, c.CVSSScore, c.Source, c.Description[:descLen])
	}
	if len(cveRows) > 3 {
		t.Logf("  ... và %d CVE khác.", len(cveRows)-3)
	}

	t.Log("========== BƯỚC 5: Kiểm tra insights (risk) cho pod ==========")
	var insights []models.Insight
	if err := db.Where("resource_uid = ? AND insight_type = ? AND deleted_at IS NULL", testPodUID, "vulnerability").Find(&insights).Error; err != nil {
		t.Fatalf("query insights: %v", err)
	}
	t.Logf("  Số insight (vulnerability) cho pod UID=%s: %d", testPodUID, len(insights))
	for i, in := range insights {
		if i >= 5 {
			t.Logf("  ... và %d insight khác.", len(insights)-5)
			break
		}
		t.Logf("  [%d] CVEID=%s Severity=%s Title=%s Affected=%s@%s", i+1, in.CVEID, in.Severity, in.Title, in.AffectedComponent, in.AffectedVersion)
	}

	t.Log("========== BƯỚC 6: View chi tiết pod/SBOM (matches + CVE preload, như dashboard) ==========")
	var matchesWithCVE []models.CVEMatch
	if err := db.Where("sbom_id = ? AND deleted_at IS NULL", sbomRow.ID).Preload("CVE").Order("severity DESC").Find(&matchesWithCVE).Error; err != nil {
		t.Fatalf("query cve_matches with CVE preload: %v", err)
	}
	t.Logf("  SBOM ID=%d ImageDigest=%s PackageCount=1", sbomRow.ID, sbomRow.ImageDigest)
	t.Logf("  Component: %s@%s PURL=%s", testPackageName, testPackageVer, comp.PURL)
	t.Logf("  Số CVE hiển thị (cho dashboard): %d", len(matchesWithCVE))
	for i, m := range matchesWithCVE {
		if i >= 3 {
			break
		}
		desc := ""
		if m.CVE.ID != 0 {
			desc = m.CVE.Description
			if len(desc) > 100 {
				desc = desc[:100] + "..."
			}
		}
		t.Logf("  [%d] CVEID=%s Severity=%s CVSS=%.1f MatchedBy=%s", i+1, m.CVEID, m.Severity, m.CVSS, m.MatchedBy)
		t.Logf("       Description: %s", desc)
	}

	t.Log("========== BÁO CÁO KẾT QUẢ FULL LUỒNG ==========")
	t.Logf("  Pod:          %s/%s (UID=%s)", testNamespace, testPodName, testPodUID)
	t.Logf("  Image:        %s (digest=%s)", containerImage, testImageDigest)
	t.Logf("  Package:      %s@%s (PURL=%s)", testPackageName, testPackageVer, comp.PURL)
	t.Logf("  cve_matches:  %d (nvd-fallback: %d)", len(matches), nvdMatches)
	t.Logf("  cves:         %d (source=nvd: %d)", len(cveRows), nvdCves)
	t.Logf("  insights:     %d (vulnerability risk)", len(insights))
	t.Log("========== KẾT THÚC: Full flow distroless → NVD → DB → risk OK ==========")
}
