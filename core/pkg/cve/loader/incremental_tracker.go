package loader

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

// FileMetadata tracks CVE file metadata for incremental updates
type FileMetadata struct {
	ID               uint      `gorm:"primaryKey"`
	FilePath         string    `gorm:"type:varchar(500);uniqueIndex;not null"`
	CVEID            string    `gorm:"type:varchar(255);not null;index"`
	FileSize         int64     `gorm:"not null"`
	FileMTime        time.Time `gorm:"column:file_mtime;not null;index"`
	FileHash         string    `gorm:"type:varchar(64)"` // SHA256
	LastProcessedAt  time.Time `gorm:"not null;index"`
	ProcessingStatus string    `gorm:"type:varchar(20);default:'success';index"` // success, failed, pending
	ErrorMessage     string    `gorm:"type:text"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// TableName specifies the table name
func (FileMetadata) TableName() string {
	return "cve_file_metadata"
}

// IncrementalTracker manages incremental CVE updates
type IncrementalTracker struct {
	db          *gorm.DB
	sourceDir   string
	computeHash bool // Compute file hash (slower but more accurate)
}

// NewIncrementalTracker creates a new incremental update tracker
func NewIncrementalTracker(db *gorm.DB, sourceDir string, computeHash bool) *IncrementalTracker {
	return &IncrementalTracker{
		db:          db,
		sourceDir:   sourceDir,
		computeHash: computeHash,
	}
}

// GetFilesToProcess returns list of files that need processing
// A file needs processing if:
// 1. It's new (not in database)
// 2. It's been modified (mtime changed)
// 3. Previous processing failed
// 4. Hash changed (if hash checking enabled)
func (t *IncrementalTracker) GetFilesToProcess(ctx context.Context) ([]string, error) {
	// Step 1: Scan directory for all JSON files
	allFiles, err := t.scanDirectory()
	if err != nil {
		return nil, fmt.Errorf("directory scan failed: %w", err)
	}

	// Step 2: Get existing metadata from database
	existingMeta, err := t.getExistingMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing metadata: %w", err)
	}

	// Step 3: Determine which files need processing
	var toProcess []string
	var toUpdate []FileMetadata
	var newFiles []FileMetadata

	for filePath, fileInfo := range allFiles {
		meta, exists := existingMeta[filePath]

		shouldProcess := false
		var reason string

		if !exists {
			// New file
			shouldProcess = true
			reason = "new_file"

			// Prepare metadata for new file
			newMeta := FileMetadata{
				FilePath:         filePath,
				CVEID:            extractCVEIDFromPath(filePath),
				FileSize:         fileInfo.Size,
				FileMTime:        fileInfo.MTime,
				ProcessingStatus: "pending",
			}

			if t.computeHash {
				hash, _ := t.computeFileHash(filePath)
				newMeta.FileHash = hash
			}

			newFiles = append(newFiles, newMeta)

		} else {
			// Existing file - check if changed

			// Check 1: Modified time changed
			if fileInfo.MTime.After(meta.FileMTime) {
				shouldProcess = true
				reason = "mtime_changed"
			}

			// Check 2: Size changed (quick check)
			if !shouldProcess && fileInfo.Size != meta.FileSize {
				shouldProcess = true
				reason = "size_changed"
			}

			// Check 3: Previous processing failed
			if !shouldProcess && meta.ProcessingStatus == "failed" {
				shouldProcess = true
				reason = "retry_failed"
			}

			// Check 4: Hash changed (if enabled)
			if !shouldProcess && t.computeHash && meta.FileHash != "" {
				currentHash, err := t.computeFileHash(filePath)
				if err == nil && currentHash != meta.FileHash {
					shouldProcess = true
					reason = "hash_changed"
				}
			}

			if shouldProcess {
				// Update metadata
				meta.FileSize = fileInfo.Size
				meta.FileMTime = fileInfo.MTime
				meta.ProcessingStatus = "pending"
				meta.ErrorMessage = fmt.Sprintf("Changed: %s", reason)

				if t.computeHash {
					hash, _ := t.computeFileHash(filePath)
					meta.FileHash = hash
				}

				toUpdate = append(toUpdate, meta)
			}
		}

		if shouldProcess {
			toProcess = append(toProcess, filePath)
		}
	}

	// Step 4: Persist metadata updates
	if err := t.saveMetadata(ctx, newFiles, toUpdate); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return toProcess, nil
}

// MarkProcessingComplete marks files as successfully processed
func (t *IncrementalTracker) MarkProcessingComplete(ctx context.Context, filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}

	const chunkSize = 10000
	now := time.Now()
	for i := 0; i < len(filePaths); i += chunkSize {
		end := i + chunkSize
		if end > len(filePaths) {
			end = len(filePaths)
		}
		if err := t.db.WithContext(ctx).
			Model(&FileMetadata{}).
			Where("file_path IN ?", filePaths[i:end]).
			Updates(map[string]interface{}{
				"processing_status": "success",
				"last_processed_at": now,
				"error_message":     "",
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

// MarkProcessingFailed marks files as failed
func (t *IncrementalTracker) MarkProcessingFailed(ctx context.Context, filePath string, err error) error {
	return t.db.WithContext(ctx).
		Model(&FileMetadata{}).
		Where("file_path = ?", filePath).
		Updates(map[string]interface{}{
			"processing_status": "failed",
			"error_message":     err.Error(),
			"last_processed_at": time.Now(),
		}).Error
}

// GetStats returns processing statistics
func (t *IncrementalTracker) GetStats(ctx context.Context) (*TrackerStats, error) {
	var stats TrackerStats

	// Total files
	t.db.WithContext(ctx).Model(&FileMetadata{}).Count(&stats.TotalFiles)

	// By status
	t.db.WithContext(ctx).Model(&FileMetadata{}).
		Where("processing_status = ?", "success").
		Count(&stats.SuccessCount)

	t.db.WithContext(ctx).Model(&FileMetadata{}).
		Where("processing_status = ?", "failed").
		Count(&stats.FailedCount)

	t.db.WithContext(ctx).Model(&FileMetadata{}).
		Where("processing_status = ?", "pending").
		Count(&stats.PendingCount)

	// Last update time
	var lastProcessed FileMetadata
	if err := t.db.WithContext(ctx).
		Order("last_processed_at DESC").
		First(&lastProcessed).Error; err == nil {
		stats.LastProcessedAt = lastProcessed.LastProcessedAt
	}

	return &stats, nil
}

// CleanupOrphaned removes metadata for files that no longer exist
func (t *IncrementalTracker) CleanupOrphaned(ctx context.Context) (int64, error) {
	// Get all file paths from database
	var dbPaths []string
	if err := t.db.WithContext(ctx).
		Model(&FileMetadata{}).
		Pluck("file_path", &dbPaths).Error; err != nil {
		return 0, err
	}

	// Check which files still exist
	var orphaned []string
	for _, path := range dbPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			orphaned = append(orphaned, path)
		}
	}

	if len(orphaned) == 0 {
		return 0, nil
	}

	// Delete orphaned entries
	result := t.db.WithContext(ctx).
		Where("file_path IN ?", orphaned).
		Delete(&FileMetadata{})

	return result.RowsAffected, result.Error
}

// Private helper methods

// scanDirectory scans source directory for all JSON files
func (t *IncrementalTracker) scanDirectory() (map[string]FileInfo, error) {
	pattern := filepath.Join(t.sourceDir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	result := make(map[string]FileInfo)
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		result[file] = FileInfo{
			Path:  file,
			Size:  info.Size(),
			MTime: info.ModTime(),
		}
	}

	return result, nil
}

// getExistingMetadata retrieves existing file metadata from database
func (t *IncrementalTracker) getExistingMetadata(ctx context.Context) (map[string]FileMetadata, error) {
	var metadata []FileMetadata
	if err := t.db.WithContext(ctx).Find(&metadata).Error; err != nil {
		return nil, err
	}

	result := make(map[string]FileMetadata, len(metadata))
	for _, meta := range metadata {
		result[meta.FilePath] = meta
	}

	return result, nil
}

// saveMetadata saves new and updated metadata
func (t *IncrementalTracker) saveMetadata(ctx context.Context, newFiles, updates []FileMetadata) error {
	// Insert new files
	if len(newFiles) > 0 {
		if err := t.db.WithContext(ctx).CreateInBatches(newFiles, 100).Error; err != nil {
			return fmt.Errorf("failed to insert new metadata: %w", err)
		}
	}

	// Update existing files
	for _, meta := range updates {
		if err := t.db.WithContext(ctx).Save(&meta).Error; err != nil {
			return fmt.Errorf("failed to update metadata: %w", err)
		}
	}

	return nil
}

// computeFileHash computes SHA256 hash of file
func (t *IncrementalTracker) computeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// extractCVEIDFromPath extracts CVE ID from file path
// Example: /path/to/CVE-2023-12345.json -> CVE-2023-12345
func extractCVEIDFromPath(path string) string {
	base := filepath.Base(path)
	// Remove .json extension
	if len(base) > 5 && base[len(base)-5:] == ".json" {
		return base[:len(base)-5]
	}
	return base
}

// Supporting types

// FileInfo holds file system information
type FileInfo struct {
	Path  string
	Size  int64
	MTime time.Time
}

// TrackerStats holds tracker statistics
type TrackerStats struct {
	TotalFiles      int64
	SuccessCount    int64
	FailedCount     int64
	PendingCount    int64
	LastProcessedAt time.Time
}
