package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration044_AddPodInstancesTable creates pod_instances table for lifecycle normalization
func Migration044_AddPodInstancesTable(db *gorm.DB) error {
	log.Println("Running migration 044: Add pod_instances table")

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS pod_instances (
			pod_uid VARCHAR(255) PRIMARY KEY,
			workload_id VARCHAR(255),
			namespace VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			generation INTEGER DEFAULT 1,
			started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			terminated_at TIMESTAMP WITH TIME ZONE,
			status VARCHAR(20) CHECK (status IN ('active', 'terminated')) NOT NULL DEFAULT 'active'
		);
		CREATE INDEX IF NOT EXISTS idx_pod_instances_workload_id ON pod_instances(workload_id);
		CREATE INDEX IF NOT EXISTS idx_pod_instances_status ON pod_instances(status);
		CREATE INDEX IF NOT EXISTS idx_pod_instances_namespace_name ON pod_instances(namespace, name);
	`).Error; err != nil {
		return err
	}

	// Migrate existing pods to pod_instances
	if err := db.Exec(`
		INSERT INTO pod_instances (pod_uid, namespace, name, started_at, status)
		SELECT 
			uid,
			namespace,
			name,
			created_at,
			CASE WHEN deleted_at IS NULL THEN 'active' ELSE 'terminated' END
		FROM pods
		WHERE uid NOT IN (SELECT pod_uid FROM pod_instances)
		ON CONFLICT (pod_uid) DO NOTHING;
	`).Error; err != nil {
		log.Printf("[Migration 044] Warning: Failed to migrate existing pods: %v", err)
		// Non-fatal, continue
	}

	log.Println("[Migration 044] ✅ Completed successfully")
	return nil
}
