package migrations

import (
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration105_AddRuntimeIncidents introduces stateful runtime incidents table.
func Migration105_AddRuntimeIncidents(db *gorm.DB) error {
	log.Println("Running migration 105: Add runtime_incidents table")
	if err := db.AutoMigrate(&models.RuntimeIncident{}); err != nil {
		return err
	}
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_incidents_pod_type_last_seen ON runtime_incidents(pod_uid, incident_type, last_seen_at DESC)").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_runtime_incidents_severity_last_seen ON runtime_incidents(severity_hint, last_seen_at DESC)").Error
	return nil
}
