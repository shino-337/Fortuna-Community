package migrations

import (
	"fmt"
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration133_RepairPolicyEngineAfterPartialReset restores policy engine tables and
// baseline seed when policy_* tables were dropped (e.g. ad-hoc SQL) while schema_migrations
// stayed applied — so early migrations 014–016 / 096 never re-run.
// Symptom: GET /api/v1/policy/instances returns 500 (missing policy_instances).
func Migration133_RepairPolicyEngineAfterPartialReset(db *gorm.DB) error {
	log.Println("Running migration 133: repair policy engine tables + baseline seed if needed")

	tplOK := db.Migrator().HasTable("policy_templates")
	instOK := db.Migrator().HasTable("policy_instances")
	violOK := db.Migrator().HasTable("policy_violations")

	if !tplOK || !instOK || !violOK {
		if err := db.AutoMigrate(&models.PolicyTemplate{}, &models.PolicyInstance{}); err != nil {
			return fmt.Errorf("migration 133 AutoMigrate policy templates/instances: %w", err)
		}
		// PolicyViolation uses CHECK + default:now() tags that SQLite's GORM dialect rejects; Postgres is the production target.
		if !violOK && db.Dialector.Name() != "sqlite" {
			if err := db.AutoMigrate(&models.PolicyViolation{}); err != nil {
				return fmt.Errorf("migration 133 AutoMigrate policy_violations: %w", err)
			}
		}
		_ = db.Exec("ALTER TABLE policy_templates DROP CONSTRAINT IF EXISTS policy_templates_template_id_key").Error
		_ = db.Exec("DROP INDEX IF EXISTS idx_policy_templates_template_id").Error
		log.Println("Migration 133: ensured policy_templates and policy_instances (policy_violations on non-sqlite)")
	}

	var tplCount int64
	if err := db.Raw("SELECT COUNT(*) FROM policy_templates").Scan(&tplCount).Error; err != nil {
		return fmt.Errorf("migration 133 count policy_templates: %w", err)
	}
	if tplCount == 0 {
		if err := seedBaselinePolicyTemplates(db); err != nil {
			return err
		}
		if err := seedBaselinePolicyInstances(db); err != nil {
			return err
		}
		log.Println("Migration 133: seeded baseline policy templates and instances (templates were empty)")
		return nil
	}

	var instCount int64
	if err := db.Model(&models.PolicyInstance{}).Count(&instCount).Error; err != nil {
		return fmt.Errorf("migration 133 count policy_instances: %w", err)
	}
	if instCount == 0 {
		if err := seedBaselinePolicyInstances(db); err != nil {
			return err
		}
		log.Println("Migration 133: seeded baseline policy instances (instances were empty)")
	}

	return nil
}
