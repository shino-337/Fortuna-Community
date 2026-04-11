package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration116_AddExceptionPolicies creates the exception_policies table used by RP-5.
//
// Purpose: Allow users to permanently suppress (dismiss) false-positive insights so
// that the risk engine does not re-activate them on subsequent scan cycles.
//
// Rollback:
//
//	DROP TABLE IF EXISTS exception_policies;
func Migration116_AddExceptionPolicies(db *gorm.DB) error {
	log.Println("Running migration 116: add exception_policies table")

	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS exception_policies (
    id          SERIAL PRIMARY KEY,
    resource_uid VARCHAR(255),
    cve_id       VARCHAR(100),
    insight_type VARCHAR(50),
    reason       TEXT,
    expires_at   TIMESTAMP,
    created_by   VARCHAR(255),
    created_at   TIMESTAMP DEFAULT NOW(),
    updated_at   TIMESTAMP DEFAULT NOW(),
    deleted_at   TIMESTAMP
);
`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
CREATE INDEX IF NOT EXISTS idx_exception_policies_lookup
    ON exception_policies(resource_uid, cve_id, insight_type)
    WHERE deleted_at IS NULL;
`).Error; err != nil {
		return err
	}

	log.Println("[Migration 116] Completed successfully")
	return nil
}
