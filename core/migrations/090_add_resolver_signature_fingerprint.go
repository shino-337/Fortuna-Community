package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration090_AddResolverSignatureFingerprint adds determinism metadata to:
// - sboms: resolver_version, signature_db_version, normalized_fingerprint
// - sbom_match_runs: resolver_version, matcher_version
func Migration090_AddResolverSignatureFingerprint(db *gorm.DB) error {
	log.Println("[Migration 090] Add determinism metadata (resolver_version, signature_db_version, fingerprint)")

	if !db.Migrator().HasTable("sboms") {
		log.Println("[Migration 090] sboms table does not exist, skipping")
		return nil
	}

	// sboms columns
	columnsToAddSboms := []struct {
		name   string
		sql    string
	}{
		{
			name: "resolver_version",
			sql:  `ALTER TABLE sboms ADD COLUMN resolver_version VARCHAR(32) NOT NULL DEFAULT ''`,
		},
		{
			name: "signature_db_version",
			sql:  `ALTER TABLE sboms ADD COLUMN signature_db_version VARCHAR(64) NOT NULL DEFAULT ''`,
		},
		{
			name: "normalized_fingerprint",
			sql:  `ALTER TABLE sboms ADD COLUMN normalized_fingerprint VARCHAR(64) NOT NULL DEFAULT ''`,
		},
	}

	for _, col := range columnsToAddSboms {
		var has bool
		if err := db.Raw(`
SELECT EXISTS (
  SELECT 1
  FROM information_schema.columns
  WHERE table_schema = current_schema()
    AND table_name = 'sboms'
    AND column_name = ?
);`, col.name).Scan(&has).Error; err != nil {
			return fmt.Errorf("[Migration 090] check sboms.%s: %w", col.name, err)
		}
		if !has {
			if err := db.Exec(col.sql).Error; err != nil {
				return fmt.Errorf("[Migration 090] add sboms.%s: %w", col.name, err)
			}
		}
	}

	if db.Migrator().HasTable("sbom_match_runs") {
		columnsToAddRuns := []struct {
			name string
			sql  string
		}{
			{
				name: "resolver_version",
				sql:  `ALTER TABLE sbom_match_runs ADD COLUMN resolver_version VARCHAR(32) NOT NULL DEFAULT ''`,
			},
			{
				name: "matcher_version",
				sql:  `ALTER TABLE sbom_match_runs ADD COLUMN matcher_version VARCHAR(32) NOT NULL DEFAULT ''`,
			},
		}

		for _, col := range columnsToAddRuns {
			var has bool
			if err := db.Raw(`
SELECT EXISTS (
  SELECT 1
  FROM information_schema.columns
  WHERE table_schema = current_schema()
    AND table_name = 'sbom_match_runs'
    AND column_name = ?
);`, col.name).Scan(&has).Error; err != nil {
				return fmt.Errorf("[Migration 090] check sbom_match_runs.%s: %w", col.name, err)
			}
			if !has {
				if err := db.Exec(col.sql).Error; err != nil {
					return fmt.Errorf("[Migration 090] add sbom_match_runs.%s: %w", col.name, err)
				}
			}
		}
	}

	log.Println("[Migration 090] Completed determinism metadata migration")
	return nil
}

