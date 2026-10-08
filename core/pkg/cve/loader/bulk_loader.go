package loader

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// BulkLoader parses OSV files in parallel and writes them to the versioned catalog as part of
// one catalog generation.
type BulkLoader struct {
	db                  *gorm.DB
	batchSize           int
	workers             int
	checkpointInterval  int
	catalogGenerationID uint
	catalog             *VersionedCatalog
	seenAdvisoryIDs     []string
	logger              *log.Logger

	// Statistics
	stats *BulkLoaderStats
}

// BulkLoaderStats tracks loading statistics
type BulkLoaderStats struct {
	TotalFiles     int64
	ProcessedFiles int64
	FailedFiles    int64
	// Versioned catalog (vuln_advisories): new versions written, unchanged advisories skipped,
	// current versions closed.
	AdvisoriesWritten   int64
	AdvisoriesUnchanged int64
	AdvisoriesClosed    int64
	StartTime           time.Time
	LastCheckpointTime  time.Time
	// Pointer avoids copying a sync.Mutex when snapshots are returned by value.
	mu     *sync.Mutex
	Errors []string
}

// Stats returns a point-in-time snapshot of loader counters.
func (l *BulkLoader) Stats() BulkLoaderStats {
	if l == nil || l.stats == nil {
		return BulkLoaderStats{}
	}
	if l.stats.mu != nil {
		l.stats.mu.Lock()
		defer l.stats.mu.Unlock()
	}
	return BulkLoaderStats{
		TotalFiles:          atomic.LoadInt64(&l.stats.TotalFiles),
		ProcessedFiles:      atomic.LoadInt64(&l.stats.ProcessedFiles),
		FailedFiles:         atomic.LoadInt64(&l.stats.FailedFiles),
		AdvisoriesWritten:   atomic.LoadInt64(&l.stats.AdvisoriesWritten),
		AdvisoriesUnchanged: atomic.LoadInt64(&l.stats.AdvisoriesUnchanged),
		AdvisoriesClosed:    atomic.LoadInt64(&l.stats.AdvisoriesClosed),
		StartTime:           l.stats.StartTime,
		LastCheckpointTime:  l.stats.LastCheckpointTime,
		Errors:              append([]string(nil), l.stats.Errors...),
	}
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
			mu:                 &sync.Mutex{},
			Errors:             make([]string, 0),
		},
	}
}

// SetCatalogGenerationID sets the generation the loaded advisories belong to.
func (l *BulkLoader) SetCatalogGenerationID(id uint) {
	l.catalogGenerationID = id
	l.catalog = NewVersionedCatalog(l.db, id)
}

// Catalog returns the versioned catalog writer of this load, or nil when the tables do not exist.
func (l *BulkLoader) Catalog() *VersionedCatalog {
	return l.catalog
}

// SeenAdvisoryIDs returns the IDs of every advisory parsed so far, withdrawn ones included.
func (l *BulkLoader) SeenAdvisoryIDs() []string {
	return l.seenAdvisoryIDs
}

// AddClosed counts advisories closed outside LoadFiles (removed from the source).
func (l *BulkLoader) AddClosed(n int) {
	atomic.AddInt64(&l.stats.AdvisoriesClosed, int64(n))
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

// processBatch parses a batch of files in parallel and writes their advisories to the
// versioned catalog.
func (l *BulkLoader) processBatch(ctx context.Context, files []string) error {
	if l.catalog == nil {
		return fmt.Errorf("versioned catalog tables are missing; run the core migrations first")
	}
	advisories := l.parseFilesParallel(files)
	for _, a := range advisories {
		l.seenAdvisoryIDs = append(l.seenAdvisoryIDs, a.ID)
	}
	st, err := l.catalog.Write(ctx, advisories)
	if err != nil {
		return fmt.Errorf("write versioned catalog failed: %w", err)
	}
	atomic.AddInt64(&l.stats.AdvisoriesWritten, int64(st.Written))
	atomic.AddInt64(&l.stats.AdvisoriesUnchanged, int64(st.Unchanged))
	atomic.AddInt64(&l.stats.AdvisoriesClosed, int64(st.Closed))
	atomic.AddInt64(&l.stats.ProcessedFiles, int64(len(files)))
	return nil
}

// parseFilesParallel parses multiple files in parallel; a file that cannot be read or parsed
// is counted as failed.
func (l *BulkLoader) parseFilesParallel(files []string) []*ParsedAdvisory {
	results := make(chan *ParsedAdvisory, len(files))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, l.workers)

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			raw, err := os.ReadFile(f)
			if err != nil {
				l.logger.Printf("Parse error: failed to read file %s: %v", f, err)
				atomic.AddInt64(&l.stats.FailedFiles, 1)
				return
			}
			osvVuln, err := ParseBytes(raw, f)
			if err != nil {
				l.logger.Printf("Parse error: %v", err)
				atomic.AddInt64(&l.stats.FailedFiles, 1)
				return
			}
			results <- BuildAdvisory(osvVuln, raw)
		}(file)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var advisories []*ParsedAdvisory
	for a := range results {
		advisories = append(advisories, a)
	}
	return advisories
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
	l.logger.Printf("Versioned catalog: %d written, %d unchanged, %d closed",
		atomic.LoadInt64(&l.stats.AdvisoriesWritten), atomic.LoadInt64(&l.stats.AdvisoriesUnchanged),
		atomic.LoadInt64(&l.stats.AdvisoriesClosed))
	l.logger.Printf("")
	l.logger.Printf("Performance:")
	l.logger.Printf("  Time taken: %v", elapsed.Round(time.Second))
	l.logger.Printf("  Average rate: %.1f files/sec", float64(l.stats.TotalFiles)/elapsed.Seconds())
	l.logger.Printf("=========================================")
}

// Helper functions

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
