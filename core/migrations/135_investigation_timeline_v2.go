package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration135_InvestigationTimelineV2 adds append-only timeline + case collaboration/archive columns.
func Migration135_InvestigationTimelineV2(db *gorm.DB) error {
	log.Println("Running migration 135: investigation_activity_logs + case v2 columns")
	if err := db.AutoMigrate(&models.InvestigationCase{}, &models.InvestigationActivityLog{}); err != nil {
		return err
	}
	if db.Migrator().HasTable("investigation_activity_logs") {
		switch db.Dialector.Name() {
		case "sqlite":
			_ = db.Exec(`CREATE TRIGGER IF NOT EXISTS trg_ial_no_update BEFORE UPDATE ON investigation_activity_logs BEGIN SELECT RAISE(ROLLBACK, 'investigation_activity_logs is append-only'); END;`).Error
			_ = db.Exec(`CREATE TRIGGER IF NOT EXISTS trg_ial_no_delete BEFORE DELETE ON investigation_activity_logs BEGIN SELECT RAISE(ROLLBACK, 'investigation_activity_logs is append-only'); END;`).Error
		default:
			_ = db.Exec(`
CREATE OR REPLACE FUNCTION forbid_investigation_activity_logs_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'investigation_activity_logs is append-only';
END;
$$ LANGUAGE plpgsql;`).Error
			_ = db.Exec(`DROP TRIGGER IF EXISTS trg_ial_no_mutate ON investigation_activity_logs;`).Error
			_ = db.Exec(`
CREATE TRIGGER trg_ial_no_mutate
BEFORE UPDATE OR DELETE ON investigation_activity_logs
FOR EACH ROW EXECUTE FUNCTION forbid_investigation_activity_logs_mutation();`).Error
		}
	}
	// Normalize legacy status values to uppercase lifecycle.
	_ = db.Exec(`UPDATE investigation_cases SET status = 'OPEN' WHERE LOWER(status) = 'open'`).Error
	_ = db.Exec(`UPDATE investigation_cases SET status = 'TRIAGED' WHERE LOWER(status) IN ('triaging', 'triaged')`).Error
	_ = db.Exec(`UPDATE investigation_cases SET status = 'RESOLVED' WHERE LOWER(status) = 'closed'`).Error
	_ = db.Exec(`UPDATE investigation_cases SET status = 'CONTAINED' WHERE LOWER(status) = 'contained'`).Error
	log.Println("Migration 135 completed successfully")
	return nil
}
