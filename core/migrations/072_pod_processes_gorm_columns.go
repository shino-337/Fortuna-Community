package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration072_PodProcessesGormColumns renames pod_processes columns to match GORM default (PID->p_id, PPID->pp_id).
func Migration072_PodProcessesGormColumns(db *gorm.DB) error {
	log.Println("Running migration 072: Rename pod_processes pid/ppid to p_id/pp_id for GORM")

	if err := db.Exec(`ALTER TABLE pod_processes RENAME COLUMN pid TO p_id`).Error; err != nil {
		log.Printf("[Migration 072] pid->p_id: %v (column may already be p_id)", err)
	}
	if err := db.Exec(`ALTER TABLE pod_processes RENAME COLUMN ppid TO pp_id`).Error; err != nil {
		log.Printf("[Migration 072] ppid->pp_id: %v (column may already be pp_id)", err)
	}

	log.Println("[Migration 072] Completed successfully")
	return nil
}
