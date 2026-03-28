package migrations

import (
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration108_AddRuntimeEventsCanonicalColumns evolves runtime_events toward canonical contract fields.
func Migration108_AddRuntimeEventsCanonicalColumns(db *gorm.DB) error {
	log.Println("Running migration 108: Add runtime_events canonical columns")
	if err := db.AutoMigrate(&models.RuntimeEvent{}); err != nil {
		return err
	}
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_event_id ON runtime_events(event_id)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_events_pod_observed ON runtime_events(pod_uid, observed_at DESC)").Error
	return nil
}
