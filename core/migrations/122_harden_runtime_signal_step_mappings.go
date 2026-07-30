package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration122_HardenRuntimeSignalStepMappings adds rollout/audit fields for control-plane guardrails.
func Migration122_HardenRuntimeSignalStepMappings(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.RuntimeSignalStepMapping{}); err != nil {
		return err
	}
	log.Println("Migration122: runtime_signal_step_mappings hardened (effective_from, created_by, updated_by)")
	return nil
}

