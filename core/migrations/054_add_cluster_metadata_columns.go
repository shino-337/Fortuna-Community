package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration054_AddClusterMetadataColumns adds cluster metadata for SSOT (source, k8s_version, distribution).
// cluster_id remains immutable; name/source/k8s_version/distribution are mutable (updated on sync).
func Migration054_AddClusterMetadataColumns(db *gorm.DB) error {
	log.Println("Running migration 054: Add cluster metadata columns (source, k8s_version, distribution)")

	if !db.Migrator().HasTable("clusters") {
		log.Println("[Migration 054] clusters table does not exist, skipping")
		return nil
	}

	for _, col := range []struct{ name, def string }{
		{"source", "VARCHAR(32)"},
		{"k8s_version", "VARCHAR(64)"},
		{"distribution", "VARCHAR(32)"},
	} {
		if db.Migrator().HasColumn("clusters", col.name) {
			continue
		}
		if err := db.Exec("ALTER TABLE clusters ADD COLUMN IF NOT EXISTS " + col.name + " " + col.def).Error; err != nil {
			log.Printf("[Migration 054] ⚠️  Add column %s: %v", col.name, err)
			continue
		}
		log.Printf("[Migration 054] ✅ Added clusters.%s", col.name)
	}
	return nil
}
