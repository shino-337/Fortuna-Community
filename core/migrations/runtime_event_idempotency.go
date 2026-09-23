package migrations

import (
	"fmt"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// EnsureRuntimeEventIdempotency installs the physical source-record replay key
// used by scoped runtime ingest. Legacy rows are left with NULL/empty identities
// and remain outside the partial uniqueness contract.
func EnsureRuntimeEventIdempotency(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("runtime event idempotency: database is nil")
	}
	if !db.Migrator().HasTable(&models.RuntimeEvent{}) {
		return fmt.Errorf("runtime event idempotency: runtime_events table is missing")
	}
	for _, column := range []string{"ClusterID", "SourceRecordID"} {
		if !db.Migrator().HasColumn(&models.RuntimeEvent{}, column) {
			if column != "SourceRecordID" {
				return fmt.Errorf("runtime event idempotency: runtime_events.%s is missing", column)
			}
			if err := db.Migrator().AddColumn(&models.RuntimeEvent{}, column); err != nil {
				return fmt.Errorf("runtime event idempotency: add source_record_id: %w", err)
			}
		}
	}

	var duplicateGroups int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT cluster_id, source_record_id
			FROM runtime_events
			WHERE source_record_id IS NOT NULL AND source_record_id <> ''
			GROUP BY cluster_id, source_record_id
			HAVING COUNT(*) > 1
		) duplicates
	`).Scan(&duplicateGroups).Error; err != nil {
		return fmt.Errorf("runtime event idempotency: validate existing source identities: %w", err)
	}
	if duplicateGroups != 0 {
		return fmt.Errorf("runtime event idempotency: found %d duplicate physical source-record identities", duplicateGroups)
	}

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_runtime_event_source_record_identity
		ON runtime_events(cluster_id, source_record_id)
		WHERE source_record_id IS NOT NULL AND source_record_id <> ''
	`).Error; err != nil {
		return fmt.Errorf("runtime event idempotency: create source-record unique index: %w", err)
	}
	return nil
}
