package migrations

import (
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// EnsureRuntimeEventIngestClaims installs a new replay boundary without imposing
// a unique constraint on historical runtime_events, which may contain legacy
// duplicates from pre-coverage retry behavior.
func EnsureRuntimeEventIngestClaims(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.RuntimeEventIngestClaim{}) {
		if err := db.Migrator().CreateTable(&models.RuntimeEventIngestClaim{}); err != nil {
			return err
		}
	}
	if err := db.Exec("SELECT cluster_id,event_id,payload_sha256,accepted_at FROM runtime_event_ingest_claims LIMIT 0").Error; err != nil {
		return err
	}
	return ensureIndex(db, "idx_runtime_event_ingest_claim_accepted", "runtime_event_ingest_claims", "accepted_at", false)
}
