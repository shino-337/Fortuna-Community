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
	// 2 packages inserted; second has ECOSYSTEM ranges which should be skipped, but package row is still inserted
	if stats.PackagesInserted != 2 {
		t.Fatalf("packages inserted=%d, want 2", stats.PackagesInserted)
	}
	// 2 SEMVER intervals created for first affected; second affected ranges are skipped
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

