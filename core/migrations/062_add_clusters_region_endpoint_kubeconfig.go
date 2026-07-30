package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration062_AddClustersRegionEndpointKubeconfig adds region, endpoint, kubeconfig to clusters
// so that GORM model Cluster (used in agent sync) matches the DB schema.
func Migration062_AddClustersRegionEndpointKubeconfig(db *gorm.DB) error {
	log.Println("Running migration 062: Add clusters region, endpoint, kubeconfig columns")

	if !db.Migrator().HasTable("clusters") {
		log.Println("[Migration 062] clusters table does not exist, skipping")
		return nil
	}

	for _, col := range []struct{ name, def string }{
		{"region", "VARCHAR(128)"},
		{"endpoint", "VARCHAR(512)"},
		{"kubeconfig", "TEXT"},
	} {
		if db.Migrator().HasColumn("clusters", col.name) {
			continue
		}
		if err := db.Exec("ALTER TABLE clusters ADD COLUMN IF NOT EXISTS " + col.name + " " + col.def).Error; err != nil {
			log.Printf("[Migration 062] ⚠️  Add column %s: %v", col.name, err)
			continue
		}
		log.Printf("[Migration 062] ✅ Added clusters.%s", col.name)
	}
	return nil
}
