package database

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestOSVMirrorBulkQuery_GoModule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{}, &models.CVE{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed NVD CVE backing for alias CVE-2023-1234 (higher-fidelity metadata)
	now := time.Now()
	if err := db.Create(&models.CVE{
		CVEID:         "CVE-2023-1234",
		Severity:      "CRITICAL",
		CVSSScore:     9.8,
		CVSSVector:    "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		PublishedDate: &now,
		LastModifiedDate: &now,
		References:    models.ToJSONBString([]string{"https://nvd.nist.gov/vuln/detail/CVE-2023-1234"}),
	}).Error; err != nil {
		t.Fatalf("seed NVD CVE: %v", err)
	}

	// Seed OSV mirror for logrus: introduced 0, fixed v1.9.1, with alias to CVE-2023-1234
	v := models.OSVVulnerability{ID: "GO-TEST-1", Summary: "logrus vuln", Details: "details", Severity: "HIGH", CVSSScore: 7.5, Aliases: `["CVE-2023-1234"]`}
	if err := db.Create(&v).Error; err != nil {
		t.Fatalf("seed vuln: %v", err)
	}
	p := models.OSVPackage{VulnID: "GO-TEST-1", Ecosystem: "go", PackageName: "github.com/sirupsen/logrus"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed package: %v", err)
	}
	r := models.OSVRange{PackageID: p.ID, RangeType: "SEMVER", Introduced: "0", Fixed: "v1.9.1"}
	if err := db.Create(&r).Error; err != nil {
		t.Fatalf("seed range: %v", err)
	}

	m := NewPostgresManager(db)
	got, err := m.GetVulnerabilitiesForPackages(context.Background(), "go", []string{"github.com/sirupsen/logrus"})
	if err != nil {
		t.Fatalf("GetVulnerabilitiesForPackages: %v", err)
	}
	cves := got["github.com/sirupsen/logrus"]
	if len(cves) != 1 {
		t.Fatalf("expected 1 CVE from OSV mirror, got %d (%+v)", len(cves), cves)
	}
	gotCVE := cves[0]
	// Prefer canonical CVE-* id from OSV aliases when present (dashboard / CVEMatch keys).
	if gotCVE.ID != "CVE-2023-1234" {
		t.Fatalf("unexpected cve id: %q", gotCVE.ID)
	}
	// Constraint should be "<v1.9.1" (introduced 0 means no lower bound)
	if gotCVE.Constraint != "<v1.9.1" {
		t.Fatalf("unexpected constraint: %q", gotCVE.Constraint)
	}
	// Severity/CVSS should be enriched from NVD alias CVE-2023-1234 (CRITICAL / 9.8), not OSV summary
	if gotCVE.Severity != "CRITICAL" {
		t.Fatalf("expected severity CRITICAL from NVD, got %q", gotCVE.Severity)
	}
	if gotCVE.CVSSScore != 9.8 {
		t.Fatalf("expected CVSSScore 9.8 from NVD, got %v", gotCVE.CVSSScore)
	}
	if len(gotCVE.References) == 0 {
		t.Fatalf("expected at least one reference from NVD, got 0")
	}
}

func TestOSVMirrorBulkQuery_AlpineBusybox_ECOSYSTEM(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{}, &models.CVE{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.CVE{
		CVEID:              "CVE-2023-42363",
		Severity:           "HIGH",
		CVSSScore:          7.5,
		PublishedDate:      &now,
		LastModifiedDate:   &now,
	}).Error; err != nil {
		t.Fatalf("seed cve: %v", err)
	}
	v := models.OSVVulnerability{
		ID: "ALPINE-CVE-2023-42363", Summary: "busybox", Details: "uaf", Severity: "HIGH", CVSSScore: 7.5,
		Aliases: `["CVE-2023-42363"]`,
	}
	if err := db.Create(&v).Error; err != nil {
		t.Fatalf("seed vuln: %v", err)
	}
	p := models.OSVPackage{VulnID: v.ID, Ecosystem: "alpine", PackageName: "busybox"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed package: %v", err)
	}
	r := models.OSVRange{PackageID: p.ID, RangeType: "ECOSYSTEM", Introduced: "0", Fixed: "1.99.0"}
	if err := db.Create(&r).Error; err != nil {
		t.Fatalf("seed range: %v", err)
	}
	m := NewPostgresManager(db)
	got, err := m.GetVulnerabilitiesForPackages(context.Background(), "alpine", []string{"busybox"})
	if err != nil {
		t.Fatalf("GetVulnerabilitiesForPackages: %v", err)
	}
	cves := got["busybox"]
	if len(cves) != 1 {
		t.Fatalf("expected 1 CVE, got %d (%+v)", len(cves), cves)
	}
	if cves[0].ID != "CVE-2023-42363" {
		t.Fatalf("cve id: %q", cves[0].ID)
	}
	if cves[0].Constraint != "<1.99.0" {
		t.Fatalf("constraint: %q", cves[0].Constraint)
	}
}

func TestOSVMirrorBulkQuery_SkipsWhenTablesMissing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Migrate primary OSV tables used by existing bulk query path so Manager can fallback without error.
	if err := db.AutoMigrate(&models.CVE{}, &models.PackageVulnerability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	m := NewPostgresManager(db)
	got, err := m.GetVulnerabilitiesForPackages(context.Background(), "go", []string{"k8s.io/kubernetes"})
	if err != nil {
		t.Fatalf("expected no error when mirror tables missing; got %v", err)
	}
	if len(got["k8s.io/kubernetes"]) != 0 {
		t.Fatalf("expected 0 CVEs when mirror tables missing; got %d", len(got["k8s.io/kubernetes"]))
	}
}

func TestShouldUseNVDFallbackForPackage_OSVMirrorPackageRow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	m := NewPostgresManager(db)
	ctx := context.Background()

	if !m.ShouldUseNVDFallbackForPackage(ctx, "alpine", "not-in-osv-pkg") {
		t.Fatal("expected NVD fallback when OSV has no row for package")
	}
	if m.ShouldUseNVDFallbackForPackage(ctx, "alpine", "") {
		t.Fatal("expected no NVD fallback for distro when package name empty")
	}

	v := models.OSVVulnerability{ID: "ALP-1", Summary: "x", Details: "d", Severity: "LOW", CVSSScore: 1}
	if err := db.Create(&v).Error; err != nil {
		t.Fatalf("seed vuln: %v", err)
	}
	p := models.OSVPackage{VulnID: v.ID, Ecosystem: "alpine", PackageName: "busybox"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed pkg: %v", err)
	}
	if m.ShouldUseNVDFallbackForPackage(ctx, "alpine", "busybox") {
		t.Fatal("expected NVD blocked when OSV mirror lists package")
	}
	if m.ShouldUseNVDFallbackForPackage(ctx, "alpine", "BusyBox") {
		t.Fatal("expected NVD blocked for BusyBox when OSV has busybox (case-insensitive)")
	}
	// Non-distro ecosystems: allow regardless of OSV rows
	if !m.ShouldUseNVDFallbackForPackage(ctx, "go", "busybox") {
		t.Fatal("expected NVD policy allowed for go ecosystem")
	}
}

