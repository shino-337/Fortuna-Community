package loader

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openGenerationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1) // one in-memory database
	if err := db.AutoMigrate(&models.CatalogGeneration{}, &models.PackageVulnerability{}, &FileMetadata{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func activeGen(t *testing.T, db *gorm.DB, at time.Time) uint {
	t.Helper()
	g := models.CatalogGeneration{CatalogType: "cve", SourceName: "osv", Status: "active", StartedAt: at, ActivatedAt: &at, RecordCounts: "{}"}
	if err := db.Create(&g).Error; err != nil {
		t.Fatal(err)
	}
	return g.ID
}

func idsOfGeneration(t *testing.T, db *gorm.DB, gen uint) []string {
	t.Helper()
	var ids []string
	db.Model(&models.PackageVulnerability{}).Where("catalog_generation_id = ?", gen).Order("cve_id").Pluck("cve_id", &ids)
	return ids
}

func TestRemovedCVEIDsOnlyCountsTrackedDirectory(t *testing.T) {
	db := openGenerationDB(t)
	dir := t.TempDir()
	keep := filepath.Join(dir, "KEEP-1.json")
	if err := os.WriteFile(keep, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, m := range []FileMetadata{
		{FilePath: keep, CVEID: "KEEP-1", LastProcessedAt: time.Now(), FileMTime: time.Now()},
		{FilePath: filepath.Join(dir, "GONE-1.json"), CVEID: "GONE-1", LastProcessedAt: time.Now(), FileMTime: time.Now()},
		{FilePath: "/elsewhere/OTHER-1.json", CVEID: "OTHER-1", LastProcessedAt: time.Now(), FileMTime: time.Now()},
	} {
		m := m
		if err := db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}
	got, err := NewIncrementalTracker(db, dir, false).RemovedCVEIDs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if len(got) != 1 || got[0] != "GONE-1" {
		t.Fatalf("removed = %v, want [GONE-1]", got)
	}
}

// PostgreSQL keeps microseconds; a file whose stored mtime lost its nanoseconds is unchanged.
func TestTrackerIgnoresSubMicrosecondMTime(t *testing.T) {
	db := openGenerationDB(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "ADV-1.json")
	if err := os.WriteFile(path, []byte(`{"id":"ADV-1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(2026, 10, 8, 9, 0, 0, 123456789, time.UTC)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&FileMetadata{FilePath: path, CVEID: "ADV-1", FileSize: 14, FileMTime: mtime.Truncate(time.Microsecond),
		LastProcessedAt: time.Now(), ProcessingStatus: "success"}).Error; err != nil {
		t.Fatal(err)
	}
	files, err := NewIncrementalTracker(db, dir, false).GetFilesToProcess(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("unchanged file reprocessed: %v", files)
	}
}
