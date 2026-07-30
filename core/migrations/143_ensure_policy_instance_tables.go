package migrations

import (
	"fmt"
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration143_EnsurePolicyInstanceTables repairs policy engine tables when
// earlier repair migrations were already marked applied but the schema was
// later partially reset. The Policy Rules page depends on policy_instances for
// the Instances tab and policy_violations for future evaluation output.
func Migration143_EnsurePolicyInstanceTables(db *gorm.DB) error {
	log.Println("Running migration 143: ensure policy instance and violation tables")

	if err := db.AutoMigrate(
		&models.PolicyTemplate{},
		&models.PolicyInstance{},
	); err != nil {
		return fmt.Errorf("migration 143 automigrate policy templates/instances: %w", err)
	}

	if db.Dialector.Name() != "sqlite" {
		if err := db.AutoMigrate(&models.PolicyViolation{}); err != nil {
			return fmt.Errorf("migration 143 automigrate policy violations: %w", err)
		}
	}

	_ = db.Exec("ALTER TABLE policy_templates DROP CONSTRAINT IF EXISTS policy_templates_template_id_key").Error
	_ = db.Exec("DROP INDEX IF EXISTS idx_policy_templates_template_id").Error

	var tplCount int64
	if err := db.Model(&models.PolicyTemplate{}).Count(&tplCount).Error; err != nil {
		return fmt.Errorf("migration 143 count policy_templates: %w", err)
	}
	if tplCount == 0 {
		if err := seedBaselinePolicyTemplates(db); err != nil {
			return err
		}
	}

	var instCount int64
	if err := db.Model(&models.PolicyInstance{}).Count(&instCount).Error; err != nil {
		return fmt.Errorf("migration 143 count policy_instances: %w", err)
	}
	if instCount == 0 {
		if err := seedBaselinePolicyInstances(db); err != nil {
			return err
		}
	}

	log.Printf("Migration 143 completed: policy_templates=%d policy_instances=%d", tplCount, instCount)
	return nil
}
