package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration129_SecurityActivityAuditV2 expands security_activity_logs for investigation-grade auditing
// and installs DB-level immutability triggers (append-only).
func Migration129_SecurityActivityAuditV2(db *gorm.DB) error {
	log.Println("Running migration 129: security_activity_logs v2 + immutability triggers")
	if err := db.AutoMigrate(&models.SecurityActivityLog{}); err != nil {
		return err
	}
	if !db.Migrator().HasTable("security_activity_logs") {
		log.Println("Migration 129: table missing, skipping triggers")
		return nil
	}
	switch db.Dialector.Name() {
	case "sqlite":
		_ = db.Exec(`CREATE TRIGGER IF NOT EXISTS trg_sal_no_update BEFORE UPDATE ON security_activity_logs BEGIN SELECT RAISE(ROLLBACK, 'security_activity_logs is append-only'); END;`).Error
		_ = db.Exec(`CREATE TRIGGER IF NOT EXISTS trg_sal_no_delete BEFORE DELETE ON security_activity_logs BEGIN SELECT RAISE(ROLLBACK, 'security_activity_logs is append-only'); END;`).Error
	default:
		_ = db.Exec(`
CREATE OR REPLACE FUNCTION forbid_security_activity_logs_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'security_activity_logs is append-only';
END;
$$ LANGUAGE plpgsql;`).Error
		_ = db.Exec(`DROP TRIGGER IF EXISTS trg_sal_no_mutate ON security_activity_logs;`).Error
		_ = db.Exec(`
CREATE TRIGGER trg_sal_no_mutate
BEFORE UPDATE OR DELETE ON security_activity_logs
FOR EACH ROW EXECUTE FUNCTION forbid_security_activity_logs_mutation();`).Error
	}
	log.Println("Migration 129 completed successfully")
	return nil
}
