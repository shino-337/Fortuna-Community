package osv

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

func TestIngestDocument_FlattensPackagesAndRanges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	doc := &Document{
		ID:      "GO-TEST-1234",
		Summary: "test",
		Details: "details",
		Aliases: []string{"cve-2023-1234", " GHSA-aaaa-bbbb-cccc "},
		Affected: []Affected{
			{
				Package: Package{Ecosystem: "Go", Name: "github.com/sirupsen/logrus"},
				Ranges: []Range{
					{
						Type: "SEMVER",
						Events: []Event{
							{Introduced: "0"},
							{Fixed: "v1.9.1"},
							{Introduced: "v1.10.0"},
							{LastAffected: "v1.10.2"},
						},
					},
				},
			},
			{
				Package: Package{Ecosystem: "Golang", Name: "github.com/sirupsen/logrus/hooks/syslog"},
				Ranges: []Range{
					{
						Type: "ECOSYSTEM",
						Events: []Event{
							{Introduced: "0"},
							{Fixed: "v9.9.9"},
						},
					},
				},
			},
		},
	}

	stats, err := IngestDocument(context.Background(), db, doc)
	if err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if stats.VulnsUpserted != 1 {
		t.Fatalf("vulns upserted=%d, want 1", stats.VulnsUpserted)
	}
	// 2 packages inserted; second is still go — ECOSYSTEM ranges for go are skipped; package row is still inserted
	if stats.PackagesInserted != 2 {
		t.Fatalf("packages inserted=%d, want 2", stats.PackagesInserted)
	}
	// 2 SEMVER intervals from first affected only
	if stats.RangesInserted != 2 {
		t.Fatalf("ranges inserted=%d, want 2", stats.RangesInserted)
	}

	var v models.OSVVulnerability
	if err := db.First(&v, "id = ?", "GO-TEST-1234").Error; err != nil {
		t.Fatalf("load vuln: %v", err)
	}
	if v.Aliases == "" || v.Aliases == "[]" {
		t.Fatalf("expected aliases JSON to be stored, got %q", v.Aliases)
	}

	var pkgs []models.OSVPackage
	if err := db.Find(&pkgs).Error; err != nil {
		t.Fatalf("load packages: %v", err)
	}
	// verify ecosystem normalized to lowercase go
	for _, p := range pkgs {
		if p.Ecosystem != "go" {
			t.Fatalf("expected ecosystem normalized to go, got %q", p.Ecosystem)
		}
	}
}

func TestIngestDocument_AlpineECOSYSTEMRange(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	doc := &Document{
		ID: "ALPINE-TEST-1", Summary: "s", Details: "d",
		Affected: []Affected{
			{
				Package: Package{Ecosystem: "Alpine:v3.19", Name: "busybox"},
				Ranges: []Range{
					{Type: "ECOSYSTEM", Events: []Event{{Introduced: "0"}, {Fixed: "1.99.0"}}},
				},
			},
		},
	}
	stats, err := IngestDocument(context.Background(), db, doc)
	if err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if stats.RangesInserted != 1 {
		t.Fatalf("ranges inserted=%d, want 1", stats.RangesInserted)
	}
	var r models.OSVRange
	if err := db.First(&r).Error; err != nil {
		t.Fatalf("range: %v", err)
	}
	if r.RangeType != "ECOSYSTEM" {
		t.Fatalf("range type=%q, want ECOSYSTEM", r.RangeType)
	}
	var p models.OSVPackage
	if err := db.First(&p).Error; err != nil {
		t.Fatalf("pkg: %v", err)
	}
	if p.Ecosystem != "alpine" {
		t.Fatalf("ecosystem=%q, want alpine", p.Ecosystem)
	}
}

