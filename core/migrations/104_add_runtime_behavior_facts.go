package migrations

import (
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration104_AddRuntimeBehaviorFacts introduces Layer-2 normalized runtime facts.
func Migration104_AddRuntimeBehaviorFacts(db *gorm.DB) error {
	log.Println("Running migration 104: Add runtime_behavior_facts table")
	if err := db.AutoMigrate(&models.RuntimeBehaviorFact{}); err != nil {
		return err
	}
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_behavior_facts_pod_observed ON runtime_behavior_facts(pod_uid, observed_at DESC)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_behavior_facts_type_observed ON runtime_behavior_facts(fact_type, observed_at DESC)").Error
	return nil
}
