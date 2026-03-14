package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration074_AddRuntimeSourcePodDetail adds runtime_source to pod_processes and pod_network_connections
// for UI indicator (Host Inspection vs Container Exec). Values: "host" | "exec" (default).
func Migration074_AddRuntimeSourcePodDetail(db *gorm.DB) error {
	log.Println("Running migration 074: Add runtime_source to pod_processes and pod_network_connections")

	// PostgreSQL: add column if not exists (avoid error on re-run)
	for _, table := range []string{"pod_processes", "pod_network_connections"} {
		// GORM raw: ALTER TABLE ... ADD COLUMN IF NOT EXISTS (PG 9.5+)
		if err := db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN IF NOT EXISTS runtime_source VARCHAR(32) DEFAULT 'exec'`).Error; err != nil {
			log.Printf("[Migration 074] %s: %v", table, err)
			return err
		}
	}
	log.Println("[Migration 074] Completed successfully")
	return nil
}
