package loader

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// BulkLoader optimized CVE loader using PostgreSQL COPY for maximum performance
type BulkLoader struct {
	db                *gorm.DB
	batchSize         int
	workers           int
	checkpointInterval int
	logger            *log.Logger

	// Statistics
	stats *BulkLoaderStats
}

// BulkLoaderStats tracks loading statistics
type BulkLoaderStats struct {
	TotalFiles         int64
	ProcessedFiles     int64
	FailedFiles        int64
	CVEsCreated        int64
	CVEsUpdated        int64
	PkgVulnsCreated    int64
	SkippedNoPackages  int64
	StartTime          time.Time
	LastCheckpointTime time.Time
	mu                 sync.Mutex
	Errors             []string
}

// NewBulkLoader creates a new optimized bulk loader
func NewBulkLoader(db *gorm.DB, workers, batchSize, checkpointInterval int) *BulkLoader {
	return &BulkLoader{
		db:                 db,
		batchSize:          batchSize,
		workers:            workers,
		checkpointInterval: checkpointInterval,
		logger:             log.New(log.Writer(), "[BulkLoader] ", log.LstdFlags),
		stats: &BulkLoaderStats{
			StartTime:          time.Now(),
			LastCheckpointTime: time.Now(),
			Errors:             make([]string, 0),
		},
	}
}

// LoadFiles processes files using optimized bulk loading
func (l *BulkLoader) LoadFiles(ctx context.Context, files []string) error {
	l.stats.TotalFiles = int64(len(files))
	l.logger.Printf("Starting bulk load of %d files", len(files))

	// Process files in batches
	for i := 0; i < len(files); i += l.batchSize {
		end := min(i+l.batchSize, len(files))
		batch := files[i:end]

		if err := l.processBatch(ctx, batch); err != nil {
			return fmt.Errorf("batch processing failed: %w", err)
		}

		// Checkpoint progress
		if int64(i)%int64(l.checkpointInterval) == 0 {
			l.printProgress()
			l.stats.LastCheckpointTime = time.Now()
		}
	}

	l.logger.Printf("✅ Bulk load completed successfully")
	l.printFinalStats()
	return nil
}

// processBatch processes a batch of files using parallel parsing + bulk insert
func (l *BulkLoader) processBatch(ctx context.Context, files []string) error {
	// Step 1: Parse files in parallel
	cves, pkgVulns, err := l.parseFilesParallel(files)
	if err != nil {
		return err
	}

	// Step 2: Bulk insert CVEs using PostgreSQL COPY
	if len(cves) > 0 {
		if err := l.bulkInsertCVEs(ctx, cves); err != nil {
			return fmt.Errorf("bulk insert CVEs failed: %w", err)
		}
	}

	// Step 3: Bulk insert package vulnerabilities
	if len(pkgVulns) > 0 {
		if err := l.bulkInsertPackageVulns(ctx, pkgVulns); err != nil {
			return fmt.Errorf("bulk insert package vulns failed: %w", err)
		}
	}

	atomic.AddInt64(&l.stats.ProcessedFiles, int64(len(files)))
	atomic.AddInt64(&l.stats.CVEsCreated, int64(len(cves)))
	atomic.AddInt64(&l.stats.PkgVulnsCreated, int64(len(pkgVulns)))

	return nil
}

// parseFilesParallel parses multiple files in parallel
func (l *BulkLoader) parseFilesParallel(files []string) ([]*ParsedCVE, []*ParsedPackageVulnerability, error) {
	type parseResult struct {
		cve      *ParsedCVE
		pkgVulns []*ParsedPackageVulnerability
		err      error
	}

	results := make(chan parseResult, len(files))
	var wg sync.WaitGroup

	// Parse files in parallel with worker pool
	semaphore := make(chan struct{}, l.workers)

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()

			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release

			// Parse file
			osvVuln, err := ParseFile(f)
			if err != nil {
				results <- parseResult{err: err}
				atomic.AddInt64(&l.stats.FailedFiles, 1)
				return
			}

			// Convert to CVE
			cve, err := ConvertToCVE(osvVuln)
			if err != nil {
				results <- parseResult{err: err}
				return
			}

			// Convert to package vulnerabilities
			pkgVulns, err := ConvertToPackageVulnerabilities(osvVuln)
			if err != nil {
				results <- parseResult{err: err}
				return
			}

			results <- parseResult{cve: cve, pkgVulns: pkgVulns}
		}(file)
	}

	// Wait for all parsers
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var cves []*ParsedCVE
	var pkgVulns []*ParsedPackageVulnerability

	for result := range results {
		if result.err != nil {
			l.logger.Printf("Parse error: %v", result.err)
			continue
		}

		if result.cve != nil {
			cves = append(cves, result.cve)
		}
		pkgVulns = append(pkgVulns, result.pkgVulns...)
	}

	return cves, pkgVulns, nil
}

// bulkInsertCVEs uses PostgreSQL COPY for fast bulk insert
func (l *BulkLoader) bulkInsertCVEs(ctx context.Context, cves []*ParsedCVE) error {
	if len(cves) == 0 {
		return nil
	}

	// Step 1: Create temporary table
	if err := l.createTempCVETable(ctx); err != nil {
		return err
	}
	defer l.dropTempCVETable(ctx)

	// Step 2: Bulk insert to temp table using COPY
	if err := l.copyCVEsToTemp(ctx, cves); err != nil {
		return err
	}

	// Step 3: Merge temp table into main table (handles upserts)
	if err := l.mergeCVEsFromTemp(ctx); err != nil {
		return err
	}

	return nil
}

// createTempCVETable creates a temporary staging table
func (l *BulkLoader) createTempCVETable(ctx context.Context) error {
	sql := `
	CREATE TEMPORARY TABLE IF NOT EXISTS cves_temp (
		cve_id VARCHAR(20) NOT NULL,
		cvss_score DECIMAL(3,1),
		cvss_vector TEXT,
		cvss_version VARCHAR(10),
		severity VARCHAR(20) NOT NULL,
		title TEXT,
		description TEXT,
		published_date TIMESTAMPTZ,
		last_modified_date TIMESTAMPTZ,
		exploit_sources TEXT[],
		cve_references JSONB,
		cwe_ids TEXT[],
		source VARCHAR(50) NOT NULL DEFAULT 'osv'
	);
	`

	return l.db.WithContext(ctx).Exec(sql).Error
}

// dropTempCVETable drops the temporary table
func (l *BulkLoader) dropTempCVETable(ctx context.Context) {
	l.db.WithContext(ctx).Exec("DROP TABLE IF EXISTS cves_temp")
}

// copyCVEsToTemp uses batch INSERT for loading (fallback from COPY)
// This is slower than PostgreSQL COPY but works with any driver and provides good performance
func (l *BulkLoader) copyCVEsToTemp(ctx context.Context, cves []*ParsedCVE) error {
	// Use batch INSERT instead of COPY
	// Process in smaller chunks to avoid parameter limits
	chunkSize := 500
	for i := 0; i < len(cves); i += chunkSize {
		end := min(i+chunkSize, len(cves))
		chunk := cves[i:end]

		if err := l.batchInsertCVEsToTemp(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

// batchInsertCVEsToTemp performs batch INSERT into temp table
func (l *BulkLoader) batchInsertCVEsToTemp(ctx context.Context, cves []*ParsedCVE) error {
	// Build multi-row INSERT
	values := make([]string, len(cves))
	args := make([]interface{}, 0, len(cves)*13)
	argIndex := 1

	for i, cve := range cves {
		// Cast cve_references to JSONB
		values[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d::jsonb, $%d, $%d)",
			argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4,
			argIndex+5, argIndex+6, argIndex+7, argIndex+8, argIndex+9,
			argIndex+10, argIndex+11, argIndex+12)

		args = append(args,
			cve.CVEID,
			cve.CVSSScore,
			cve.CVSSVector,
			cve.CVSSVersion,
			cve.Severity,
			cve.Title,
			cve.Description,
			cve.PublishedDate,
			cve.LastModifiedDate,
			"{}",
			cve.References,
			fmt.Sprintf("{%s}", joinStrings(cve.CWEIDs, ",")),
			cve.Source,
		)
		argIndex += 13
	}

	sql := fmt.Sprintf(`
		INSERT INTO cves_temp (
			cve_id, cvss_score, cvss_vector, cvss_version, severity,
			title, description, published_date, last_modified_date,
			exploit_sources, cve_references, cwe_ids, source
		) VALUES %s
	`, strings.Join(values, ","))

	return l.db.WithContext(ctx).Exec(sql, args...).Error
}

// mergeCVEsFromTemp merges temp table into main CVEs table
func (l *BulkLoader) mergeCVEsFromTemp(ctx context.Context) error {
	sql := `
	INSERT INTO cves (
		cve_id, cvss_score, cvss_vector, cvss_version, severity,
		title, description, published_date, last_modified_date,
		exploit_sources, cve_references, cwe_ids, source,
		created_at, updated_at
	)
	SELECT
		cve_id, cvss_score, cvss_vector, cvss_version, severity,
		title, description, published_date, last_modified_date,
		exploit_sources, cve_references, cwe_ids, source,
		NOW(), NOW()
	FROM cves_temp
	ON CONFLICT (cve_id) DO UPDATE SET
		cvss_score = EXCLUDED.cvss_score,
		cvss_vector = EXCLUDED.cvss_vector,
		cvss_version = EXCLUDED.cvss_version,
		severity = EXCLUDED.severity,
		title = EXCLUDED.title,
		description = EXCLUDED.description,
		last_modified_date = EXCLUDED.last_modified_date,
		cve_references = EXCLUDED.cve_references,
		cwe_ids = EXCLUDED.cwe_ids,
		updated_at = NOW()
	`

	return l.db.WithContext(ctx).Exec(sql).Error
}

// bulkInsertPackageVulns bulk inserts package vulnerabilities
func (l *BulkLoader) bulkInsertPackageVulns(ctx context.Context, pkgVulns []*ParsedPackageVulnerability) error {
	if len(pkgVulns) == 0 {
		return nil
	}

	// Use chunked batch inserts (COPY is more complex for this table)
	chunkSize := 500
	for i := 0; i < len(pkgVulns); i += chunkSize {
		end := min(i+chunkSize, len(pkgVulns))
		chunk := pkgVulns[i:end]

		if err := l.batchInsertPackageVulns(ctx, chunk); err != nil {
			return err
		}
	}

	return nil
}

// batchInsertPackageVulns inserts a batch using standard batch insert
func (l *BulkLoader) batchInsertPackageVulns(ctx context.Context, pkgVulns []*ParsedPackageVulnerability) error {
	// Build multi-row INSERT
	values := make([]string, len(pkgVulns))
	args := make([]interface{}, 0, len(pkgVulns)*10)
	argIndex := 1

	for i, pv := range pkgVulns {
		values[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, NOW(), NOW())",
			argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4, argIndex+5, argIndex+6, argIndex+7)

		args = append(args,
			pv.CVEID,
			pv.PackageName,
			pv.Ecosystem,
			pv.VersionStartIncluding,
			pv.VersionStartExcluding,
			pv.VersionEndIncluding,
			pv.VersionEndExcluding,
			pv.FixedVersion,
		)

		argIndex += 8
	}

	sql := fmt.Sprintf(`
		INSERT INTO package_vulnerabilities (
			cve_id, package_name, ecosystem,
			version_start_including, version_start_excluding,
			version_end_including, version_end_excluding,
			fixed_version, created_at, updated_at
		) VALUES %s
	`, strings.Join(values, ","))

	return l.db.WithContext(ctx).Exec(sql, args...).Error
}

// printProgress prints current progress
func (l *BulkLoader) printProgress() {
	processed := atomic.LoadInt64(&l.stats.ProcessedFiles)
	total := l.stats.TotalFiles

	elapsed := time.Since(l.stats.StartTime)
	rate := float64(processed) / elapsed.Seconds()
	remaining := total - processed
	eta := time.Duration(0)
	if rate > 0 {
		eta = time.Duration(float64(remaining)/rate) * time.Second
	}

	percentage := float64(processed) / float64(total) * 100

	l.logger.Printf("Progress: %d/%d (%.1f%%), Rate: %.1f files/sec, ETA: %v",
		processed, total, percentage, rate, eta.Round(time.Second))
}

// printFinalStats prints final statistics
func (l *BulkLoader) printFinalStats() {
	elapsed := time.Since(l.stats.StartTime)

	l.logger.Printf("")
	l.logger.Printf("=========================================")
	l.logger.Printf("✅ Bulk Load Complete!")
	l.logger.Printf("=========================================")
	l.logger.Printf("Total files: %d", l.stats.TotalFiles)
	l.logger.Printf("Successfully processed: %d", atomic.LoadInt64(&l.stats.ProcessedFiles))
	l.logger.Printf("Failed: %d", atomic.LoadInt64(&l.stats.FailedFiles))
	l.logger.Printf("")
	l.logger.Printf("Database records:")
	l.logger.Printf("  CVEs created/updated: %d", atomic.LoadInt64(&l.stats.CVEsCreated))
	l.logger.Printf("  Package vulnerabilities created: %d", atomic.LoadInt64(&l.stats.PkgVulnsCreated))
	l.logger.Printf("")
	l.logger.Printf("Performance:")
	l.logger.Printf("  Time taken: %v", elapsed.Round(time.Second))
	l.logger.Printf("  Average rate: %.1f files/sec", float64(l.stats.TotalFiles)/elapsed.Seconds())
	l.logger.Printf("=========================================")
}

// Helper functions

func formatTimestamp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	return strings.Join(strs, sep)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
