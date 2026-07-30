package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration091_AddInsightConfidenceColumns adds confidence model columns to insights.
// (Phase 2 - confidence propagation)
func Migration091_AddInsightConfidenceColumns(db *gorm.DB) error {
	log.Println("[Migration 091] Add insight confidence columns")

	if !db.Migrator().HasTable("insights") {
		log.Println("[Migration 091] insights table does not exist, skipping")
		return nil
	}

	cols := []struct {
		name string
		sql  string
	}{
		{
			name: "match_confidence",
			sql:  `ALTER TABLE insights ADD COLUMN match_confidence VARCHAR(20) NOT NULL DEFAULT ''`,
		},
		{
			name: "component_confidence",
			sql:  `ALTER TABLE insights ADD COLUMN component_confidence VARCHAR(20) NOT NULL DEFAULT ''`,
		},
		{
			name: "sbom_confidence",
			sql:  `ALTER TABLE insights ADD COLUMN sbom_confidence VARCHAR(20) NOT NULL DEFAULT ''`,
		},
		{
			name: "final_risk_confidence",
			sql:  `ALTER TABLE insights ADD COLUMN final_risk_confidence VARCHAR(20) NOT NULL DEFAULT ''`,
		},
		{
			name: "degraded",
			sql:  `ALTER TABLE insights ADD COLUMN degraded BOOLEAN NOT NULL DEFAULT FALSE`,
		},
	}

	for _, col := range cols {
		var has bool
		if err := db.Raw(`
SELECT EXISTS (
  SELECT 1
  FROM information_schema.columns
  WHERE table_schema = current_schema()
    AND table_name = 'insights'
    AND column_name = ?
);`, col.name).Scan(&has).Error; err != nil {
			return fmt.Errorf("[Migration 091] check insights.%s: %w", col.name, err)
		}
		if !has {
			if err := db.Exec(col.sql).Error; err != nil {
				return fmt.Errorf("[Migration 091] add insights.%s: %w", col.name, err)
			}
		}
	}

	log.Println("[Migration 091] Completed insight confidence migration")
	return nil
}

