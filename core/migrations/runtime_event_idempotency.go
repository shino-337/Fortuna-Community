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
	for _, column := range []string{"ClusterID", "AgentID", "SourceRecordID"} {
		if !db.Migrator().HasColumn(&models.RuntimeEvent{}, column) {
			if column == "ClusterID" {
				return fmt.Errorf("runtime event idempotency: runtime_events.%s is missing", column)
			}
			if err := db.Migrator().AddColumn(&models.RuntimeEvent{}, column); err != nil {
				return fmt.Errorf("runtime event idempotency: add %s: %w", column, err)
			}
		}
	}

	var unownedSourceRecords int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM runtime_events
		WHERE source_record_id IS NOT NULL AND source_record_id <> ''
		  AND (agent_id IS NULL OR agent_id = '')
	`).Scan(&unownedSourceRecords).Error; err != nil {
		return fmt.Errorf("runtime event idempotency: validate source-record agent ownership: %w", err)
	}
	if unownedSourceRecords != 0 {
		return fmt.Errorf("runtime event idempotency: found %d source-record identities without agent ownership", unownedSourceRecords)
	}

	var duplicateGroups int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT cluster_id, agent_id, source_record_id
			FROM runtime_events
			WHERE source_record_id IS NOT NULL AND source_record_id <> ''
			GROUP BY cluster_id, agent_id, source_record_id
			HAVING COUNT(*) > 1
		) duplicates
	`).Scan(&duplicateGroups).Error; err != nil {
		return fmt.Errorf("runtime event idempotency: validate existing source identities: %w", err)
	}
	if duplicateGroups != 0 {
		return fmt.Errorf("runtime event idempotency: found %d duplicate physical source-record identities", duplicateGroups)
	}

	if err := db.Exec("DROP INDEX IF EXISTS idx_runtime_event_source_record_identity").Error; err != nil {
		return fmt.Errorf("runtime event idempotency: drop obsolete source-record index: %w", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_runtime_event_agent_source_record_identity
		ON runtime_events(cluster_id, agent_id, source_record_id)
		WHERE source_record_id IS NOT NULL AND source_record_id <> ''
	`).Error; err != nil {
		return fmt.Errorf("runtime event idempotency: create source-record unique index: %w", err)
	}
	return nil
}
