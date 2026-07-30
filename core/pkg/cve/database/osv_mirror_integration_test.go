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

	// Seed central `cves` row for alias CVE-2023-1234 (higher-fidelity metadata than OSV summary alone)
	now := time.Now()
	if err := db.Create(&models.CVE{
		CVEID:            "CVE-2023-1234",
		Severity:         "CRITICAL",
		CVSSScore:        9.8,
		CVSSVector:       "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		PublishedDate:    &now,
		LastModifiedDate: &now,
		References:       models.ToJSONBString([]string{"https://www.cve.org/CVERecord?id=CVE-2023-1234"}),
	}).Error; err != nil {
		t.Fatalf("seed CVE row: %v", err)
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
	// Severity/CVSS should be enriched from the `cves` row for CVE-2023-1234 (CRITICAL / 9.8), not OSV summary alone
	if gotCVE.Severity != "CRITICAL" {
		t.Fatalf("expected severity CRITICAL from cves table, got %q", gotCVE.Severity)
	}
	if gotCVE.CVSSScore != 9.8 {
		t.Fatalf("expected CVSSScore 9.8 from cves table, got %v", gotCVE.CVSSScore)
	}
	if len(gotCVE.References) == 0 {
		t.Fatalf("expected at least one reference from cves table, got 0")
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
		CVEID:            "CVE-2023-42363",
		Severity:         "HIGH",
		CVSSScore:        7.5,
		PublishedDate:    &now,
		LastModifiedDate: &now,
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

func TestGetVulnerabilitiesForPackages_FallsBackWhenOSVMirrorTablesEmpty(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{},
		&models.CVE{}, &models.PackageVulnerability{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now()
	if err := db.Create(&models.CVE{
		CVEID:            "GO-2023-2402",
		Severity:         "MEDIUM",
		CVSSScore:        5,
		Description:      "x/crypto vulnerable before 0.17.0",
		PublishedDate:    &now,
		LastModifiedDate: &now,
	}).Error; err != nil {
		t.Fatalf("seed cve: %v", err)
	}
	if err := db.Create(&models.PackageVulnerability{
		CVEID:               "GO-2023-2402",
		PackageName:         "golang.org/x/crypto",
		Ecosystem:           "go",
		VersionEndExcluding: "0.17.0",
		FixedVersion:        "0.17.0",
	}).Error; err != nil {
		t.Fatalf("seed package vulnerability: %v", err)
	}

	m := NewPostgresManager(db)
	got, err := m.GetVulnerabilitiesForPackages(context.Background(), "go", []string{"golang.org/x/crypto"})
	if err != nil {
		t.Fatalf("GetVulnerabilitiesForPackages: %v", err)
	}
	cves := got["golang.org/x/crypto"]
	if len(cves) != 1 {
		t.Fatalf("expected 1 CVE from package_vulnerabilities fallback, got %d (%+v)", len(cves), cves)
	}
	if cves[0].ID != "GO-2023-2402" {
		t.Fatalf("unexpected id %q", cves[0].ID)
	}
	if cves[0].Constraint != "<0.17.0" {
		t.Fatalf("unexpected constraint %q", cves[0].Constraint)
	}
}

func TestGetVulnerabilitiesForPackages_DistroMergesCPESupplementWhenOSVEmpty(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{},
		&models.CVE{}, &models.PackageVulnerability{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.CVE{
		CVEID:            "CVE-2021-28831",
		Severity:         "HIGH",
		CVSSScore:        7.5,
		Description:      "busybox",
		PublishedDate:    &now,
		LastModifiedDate: &now,
	}).Error; err != nil {
		t.Fatalf("seed cve: %v", err)
	}
	if err := db.Create(&models.PackageVulnerability{
		CVEID:                 "CVE-2021-28831",
		PackageName:           "busybox",
		Ecosystem:             "nvd",
		VersionStartIncluding: "1.32.0",
	}).Error; err != nil {
		t.Fatalf("seed pv: %v", err)
	}

	m := NewPostgresManager(db)
	ctx := context.Background()
	got, err := m.GetVulnerabilitiesForPackages(ctx, "alpine", []string{"busybox"})
	if err != nil {
		t.Fatalf("GetVulnerabilitiesForPackages: %v", err)
	}
	cves := got["busybox"]
	if len(cves) != 1 {
		t.Fatalf("expected 1 CVE from CPE supplement merge, got %d (%+v)", len(cves), cves)
	}
	if cves[0].ID != "CVE-2021-28831" {
		t.Fatalf("unexpected id %q", cves[0].ID)
	}
	if cves[0].Constraint == "" {
		t.Fatal("expected version constraint from package_vulnerabilities")
	}
}
