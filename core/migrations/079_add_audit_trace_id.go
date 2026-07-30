package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration079_AddAuditTraceID adds trace_id to audit_logs (Finding #5.2).
// Enables tracing from Agent request (X-Correlation-ID) through sync to audit log and insight.
func Migration079_AddAuditTraceID(db *gorm.DB) error {
	log.Println("[Migration 079] Add trace_id to audit_logs (Finding #5.2)")

	if !db.Migrator().HasTable("audit_logs") {
		log.Println("[Migration 079] audit_logs table does not exist, skipping")
		return nil
	}

	var count int64
	_ = db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'audit_logs' AND column_name = 'trace_id'").Scan(&count).Error
	if count != 0 {
		log.Println("[Migration 079] trace_id already exists, skipping")
		return nil
	}
	if err := db.Exec("ALTER TABLE audit_logs ADD COLUMN trace_id VARCHAR(255) DEFAULT ''").Error; err != nil {
		return err
	}
	log.Println("[Migration 079] ✅ Added trace_id column")
	return nil
}
