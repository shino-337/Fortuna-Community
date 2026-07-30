package api

import (
	"log"
	"time"

	"gorm.io/gorm"
)

const (
	ingestRetryAttempts = 3
	ingestRetryBackoff  = time.Second
)

// dbIngestWithRetry runs fn (e.g. db.CreateInBatches) with retry on transient DB errors (Phase 3.3).
func dbIngestWithRetry(db *gorm.DB, fn func(*gorm.DB) error) error {
	var lastErr error
	backoff := ingestRetryBackoff
	for attempt := 0; attempt < ingestRetryAttempts; attempt++ {
		lastErr = fn(db)
		if lastErr == nil {
			return nil
		}
		if attempt < ingestRetryAttempts-1 {
			log.Printf("[PodDetail] ingest DB error (attempt %d/%d): %v; retry in %v", attempt+1, ingestRetryAttempts, lastErr, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 8*time.Second {
				backoff = 8 * time.Second
			}
		}
	}
	return lastErr
}
