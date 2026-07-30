package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration049_AddPodAttackStepsTable creates pod_attack_steps table for minimal AttackStep model
func Migration049_AddPodAttackStepsTable(db *gorm.DB) error {
	log.Println("Running migration 049: Add pod_attack_steps table")

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS pod_attack_steps (
			pod_uid VARCHAR(255) NOT NULL,
			step_id VARCHAR(100) NOT NULL,
			description TEXT,
			category VARCHAR(50) NOT NULL,
			confidence FLOAT DEFAULT 0.5 CHECK (confidence >= 0 AND confidence <= 1),
			evidence JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (pod_uid, step_id)
		);
		CREATE INDEX IF NOT EXISTS idx_pod_attack_steps_pod_uid ON pod_attack_steps(pod_uid);
		CREATE INDEX IF NOT EXISTS idx_pod_attack_steps_step_id ON pod_attack_steps(step_id);
		CREATE INDEX IF NOT EXISTS idx_pod_attack_steps_category ON pod_attack_steps(category);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 049] ✅ Completed successfully")
	return nil
}
