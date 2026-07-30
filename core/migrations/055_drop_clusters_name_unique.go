package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration055_DropClustersNameUnique drops the UNIQUE constraint on clusters(name)
// so that multiple clusters can share the same display name (e.g. "inferred-k8s-cluster")
// after SSOT: cluster identity is id (immutable); name is mutable and not globally unique.
func Migration055_DropClustersNameUnique(db *gorm.DB) error {
	log.Println("Running migration 055: Drop UNIQUE on clusters(name) for SSOT multi-cluster")

	if !db.Migrator().HasTable("clusters") {
		log.Println("[Migration 055] clusters table does not exist, skipping")
		return nil
	}

	// PostgreSQL default name for UNIQUE on column is clusters_name_key (table_column_key)
	// If constraint does not exist (e.g. DB created by GORM only), no-op
	for _, constraintName := range []string{"clusters_name_key", "clusters_name_uniq"} {
		var count int64
		db.Raw(
			"SELECT COUNT(1) FROM information_schema.table_constraints WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'clusters' AND constraint_name = ? AND constraint_type = 'UNIQUE'",
			constraintName,
		).Scan(&count)
		if count > 0 {
			if err := db.Exec("ALTER TABLE clusters DROP CONSTRAINT " + constraintName).Error; err != nil {
				log.Printf("[Migration 055] ⚠️  Drop constraint %s: %v", constraintName, err)
				continue
			}
			log.Printf("[Migration 055] ✅ Dropped constraint %s on clusters(name)", constraintName)
			return nil
		}
	}

	// Fallback: find any unique constraint on clusters that only involves name
	var conname string
	err := db.Raw(`
		SELECT c.conname FROM pg_constraint c
		JOIN pg_class t ON c.conrelid = t.oid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(c.conkey) AND NOT a.attisdropped
		WHERE t.relname = 'clusters' AND c.contype = 'u' AND a.attname = 'name' AND array_length(c.conkey, 1) = 1
		LIMIT 1
	`).Scan(&conname).Error
	if err == nil && conname != "" {
		if err := db.Exec("ALTER TABLE clusters DROP CONSTRAINT " + conname).Error; err != nil {
			log.Printf("[Migration 055] ⚠️  Drop constraint %s: %v", conname, err)
			return nil
		}
		log.Printf("[Migration 055] ✅ Dropped constraint %s on clusters(name)", conname)
		return nil
	}

	log.Println("[Migration 055] No UNIQUE on clusters(name) found (already dropped or created without it), skipping")
	return nil
}
