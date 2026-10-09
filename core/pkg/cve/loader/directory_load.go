package loader

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// DirectoryLoad describes one load of a directory of OSV JSON files into the versioned
// catalog. Each load that writes anything becomes a new cve catalog generation.
type DirectoryLoad struct {
	SourceDir          string
	Workers            int
	BatchSize          int
	CheckpointInterval int
	ComputeHash        bool // compare file hashes, not only size and mtime, in LoadChanged
	DryRun             bool
	LoaderVersion      string // recorded on the generation

	RequireIDs           string
	ExpectedSourceDigest string
	SourceManifestPath   string
	SourceManifestSig    string
	SourceManifestPubKey string
}

func (o DirectoryLoad) withDefaults() DirectoryLoad {
	if o.Workers <= 0 {
		o.Workers = 8
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 1000
	}
	if o.CheckpointInterval <= 0 {
		o.CheckpointInterval = 5000
	}
	if o.LoaderVersion == "" {
		o.LoaderVersion = "cve-loader-optimized"
	}
	return o
}

// validate checks the files against the required advisories and the expected source digest,
// and returns the digest of the files.
func (o DirectoryLoad) validate(files []string) (string, error) {
	if err := VerifySourceManifestSignature(o.SourceManifestPath, o.SourceManifestSig, o.SourceManifestPubKey); err != nil {
		return "", err
	}
	manifest, err := LoadSourceManifest(o.SourceManifestPath)
	if err != nil {
		return "", err
	}
	if err := ValidateRequiredAdvisories(files, MergeRequiredAdvisoryIDs(o.RequireIDs, manifest)); err != nil {
		return "", err
	}
	digest, err := ComputeSourceDigest(files)
	if err != nil {
		return "", err
	}
	if err := ValidateExpectedSourceDigest(digest, MergeExpectedDigest(o.ExpectedSourceDigest, manifest)); err != nil {
		return "", err
	}
	return digest, nil
}

// LoadAll reads every file of the directory as a new generation and closes the advisories the
// directory no longer holds.
func LoadAll(ctx context.Context, db *gorm.DB, o DirectoryLoad) error {
	o = o.withDefaults()
	files, err := filepath.Glob(filepath.Join(o.SourceDir, "*.json"))
	if err != nil {
		return fmt.Errorf("list source files: %w", err)
	}
	log.Printf("📁 Found %d advisory files to process", len(files))
	digest, err := o.validate(files)
	if err != nil {
		return err
	}
	if o.DryRun {
		log.Printf("🔵 Dry run - would process %d files", len(files))
		return nil
	}
	gen, err := startCatalogGeneration(db, o, digest, len(files))
	if err != nil {
		return err
	}
	bulk := NewBulkLoader(db, o.Workers, o.BatchSize, o.CheckpointInterval)
	bulk.SetCatalogGenerationID(gen.ID)
	if err := loadIntoGeneration(ctx, db, gen, bulk, files); err != nil {
		return err
	}
	// A full load holds every advisory of the source: close the ones that are gone.
	closed, err := bulk.Catalog().CloseMissing(ctx, bulk.SeenAdvisoryIDs())
	if err != nil {
		failCatalogGeneration(ctx, db, gen, bulk.Stats(), err.Error())
		return fmt.Errorf("close removed advisories: %w", err)
	}
	bulk.AddClosed(closed)

	tracker := NewIncrementalTracker(db, o.SourceDir, o.ComputeHash)
	if _, err := tracker.GetFilesToProcess(ctx); err != nil {
		log.Printf("⚠️  Recording source files failed: %v (the next incremental load reads every file)", err)
	} else if err := tracker.MarkProcessingComplete(ctx, files); err != nil {
		log.Printf("⚠️  Recording source files failed: %v (the next incremental load reads every file)", err)
	}
	return completeCatalogGeneration(db, gen, bulk.Stats(), "active", "")
}

// LoadChanged reads only the files added or changed since the last load and closes the
// advisories whose file was removed. An empty catalog is loaded in full.
func LoadChanged(ctx context.Context, db *gorm.DB, o DirectoryLoad) error {
	o = o.withDefaults()
	tracker := NewIncrementalTracker(db, o.SourceDir, o.ComputeHash)
	files, err := tracker.GetFilesToProcess(ctx)
	if err != nil {
		return fmt.Errorf("list changed files: %w", err)
	}
	log.Printf("📊 Found %d files to process", len(files))
	removedIDs, err := tracker.RemovedCVEIDs(ctx)
	if err != nil {
		return fmt.Errorf("list removed advisories: %w", err)
	}
	if len(removedIDs) > 0 {
		log.Printf("📊 %d advisories were removed from the source", len(removedIDs))
	}
	if !o.DryRun {
		empty, err := VersionedCatalogEmpty(ctx, db)
		if err != nil {
			return fmt.Errorf("read versioned catalog: %w", err)
		}
		if empty {
			log.Printf("📚 Versioned catalog is empty; loading every advisory once")
			return LoadAll(ctx, db, o)
		}
	}
	if len(files) == 0 && len(removedIDs) == 0 {
		log.Printf("✅ No files need processing - all up to date!")
		return nil
	}
	digest, err := o.validate(files)
	if err != nil {
		return err
	}
	if o.DryRun {
		log.Printf("🔵 Dry run - would process %d files", len(files))
		return nil
	}
	gen, err := startCatalogGeneration(db, o, digest, len(files))
	if err != nil {
		return err
	}
	bulk := NewBulkLoader(db, o.Workers, o.BatchSize, o.CheckpointInterval)
	bulk.SetCatalogGenerationID(gen.ID)
	if err := loadIntoGeneration(ctx, db, gen, bulk, files); err != nil {
		return err
	}
	closed, err := bulk.Catalog().Close(ctx, removedIDs)
	if err != nil {
		failCatalogGeneration(ctx, db, gen, bulk.Stats(), err.Error())
		return fmt.Errorf("close removed advisories: %w", err)
	}
	bulk.AddClosed(closed)
	if err := tracker.MarkProcessingComplete(ctx, files); err != nil {
		log.Printf("⚠️  Marking files processed failed: %v", err)
	}
	if err := completeCatalogGeneration(db, gen, bulk.Stats(), "active", ""); err != nil {
		return err
	}
	if removed, err := tracker.CleanupOrphaned(ctx); err == nil && removed > 0 {
		log.Printf("🗑️  Removed %d orphaned file entries", removed)
	}
	return nil
}

// loadIntoGeneration writes files into gen and fails the generation when any file fails.
func loadIntoGeneration(ctx context.Context, db *gorm.DB, gen *models.CatalogGeneration, bulk *BulkLoader, files []string) error {
	if err := bulk.LoadFiles(ctx, files); err != nil {
		failCatalogGeneration(ctx, db, gen, bulk.Stats(), err.Error())
		return fmt.Errorf("load advisories: %w", err)
	}
	if stats := bulk.Stats(); stats.FailedFiles > 0 {
		summary := fmt.Sprintf("%d source files failed; catalog generation not activated", stats.FailedFiles)
		failCatalogGeneration(ctx, db, gen, stats, summary)
		return fmt.Errorf("load advisories: %s", summary)
	}
	return nil
}

func startCatalogGeneration(db *gorm.DB, o DirectoryLoad, sourceDigest string, totalFiles int) (*models.CatalogGeneration, error) {
	counts, _ := json.Marshal(map[string]int{"totalFiles": totalFiles})
	gen := &models.CatalogGeneration{
		CatalogType:       "cve",
		SourceName:        "osv",
		SourceURL:         o.SourceDir,
		SourceDigest:      sourceDigest,
		Status:            "running",
		RecordCounts:      string(counts),
		StartedAt:         time.Now(),
		LoaderVersion:     o.LoaderVersion,
		ValidationStatus:  "pending",
		ValidationSummary: "source digest and required advisory checks passed; load running",
	}
	if err := db.Create(gen).Error; err != nil {
		return nil, err
	}
	return gen, nil
}

// failCatalogGeneration marks a generation failed and undoes its versioned catalog writes.
func failCatalogGeneration(ctx context.Context, db *gorm.DB, gen *models.CatalogGeneration, stats BulkLoaderStats, errorSummary string) {
	if err := completeCatalogGeneration(db, gen, stats, "failed", errorSummary); err != nil {
		log.Printf("⚠️  Warning: failed to mark generation %d failed: %v", gen.ID, err)
	}
	if err := RollbackCatalogGeneration(ctx, db, gen.ID); err != nil {
		log.Printf("⚠️  Warning: failed to roll back versioned catalog of generation %d: %v (the next load retries)", gen.ID, err)
	}
}

func completeCatalogGeneration(db *gorm.DB, gen *models.CatalogGeneration, stats BulkLoaderStats, status, errorSummary string) error {
	now := time.Now()
	rawCounts, err := json.Marshal(map[string]int64{
		"totalFiles":          stats.TotalFiles,
		"processedFiles":      stats.ProcessedFiles,
		"failedFiles":         stats.FailedFiles,
		"advisoriesWritten":   stats.AdvisoriesWritten,
		"advisoriesUnchanged": stats.AdvisoriesUnchanged,
		"advisoriesClosed":    stats.AdvisoriesClosed,
	})
	if err != nil {
		return err
	}
	validationStatus := "passed"
	validationSummary := "source digest validated; required advisories present; load completed"
	if stats.FailedFiles > 0 {
		validationStatus = "degraded"
		validationSummary = "load completed with failed files"
	}
	if status == "failed" {
		validationStatus = "failed"
		validationSummary = "load failed"
	}
	updates := map[string]interface{}{
		"status":             status,
		"record_counts":      string(rawCounts),
		"error_summary":      errorSummary,
		"completed_at":       &now,
		"duration_millis":    now.Sub(gen.StartedAt).Milliseconds(),
		"validation_status":  validationStatus,
		"validation_summary": validationSummary,
		"updated_at":         now,
	}
	if status == "active" {
		updates["activated_at"] = &now
	}
	return db.Model(gen).Updates(updates).Error
}
