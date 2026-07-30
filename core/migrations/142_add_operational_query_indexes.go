package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration142_AddOperationalQueryIndexes adds composite/partial indexes for
// recurring operational reads surfaced by GORM slow-query logging.
func Migration142_AddOperationalQueryIndexes(db *gorm.DB) error {
	log.Println("Running migration 142: Operational query indexes")

	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_cluster_roles_cluster_active
			ON cluster_roles(cluster_id)
			WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_risk_scores_uid_version_active_latest
			ON risk_scores(resource_uid, scorer_version, calculated_at DESC, id DESC)
			WHERE deleted_at IS NULL`,
	}

	for _, stmt := range statements {
		if err := execDDL(db, stmt); err != nil {
			return err
		}
	}

	log.Println("Migration 142 completed: operational query indexes")
	return nil
}
