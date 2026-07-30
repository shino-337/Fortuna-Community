package migrations

import (
	"fmt"
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration096_PolicyEngineBaselineBootstrap ensures policy engine schema and seeds baseline data.
func Migration096_PolicyEngineBaselineBootstrap(db *gorm.DB) error {
	log.Println("Running migration 096: Policy engine baseline bootstrap")

	if err := db.AutoMigrate(&models.PolicyTemplate{}, &models.PolicyInstance{}); err != nil {
		return fmt.Errorf("failed to ensure policy template/instance tables: %w", err)
	}

	// Old SQL migration had template_id UNIQUE (single-version); keep composite uniqueness only.
	_ = db.Exec("ALTER TABLE policy_templates DROP CONSTRAINT IF EXISTS policy_templates_template_id_key").Error
	_ = db.Exec("DROP INDEX IF EXISTS idx_policy_templates_template_id").Error

	if err := seedBaselinePolicyTemplates(db); err != nil {
		return err
	}
	if err := seedBaselinePolicyInstances(db); err != nil {
		return err
	}

	log.Println("Migration 096 completed: policy engine baseline ensured")
	return nil
}

func seedBaselinePolicyTemplates(db *gorm.DB) error {
	templates := []models.PolicyTemplate{
		{
			TemplateID:          "k8s-no-privileged-container",
			Version:             "1.0.0",
			Name:                "Disallow privileged containers",
			Description:         "Flags Pod workloads that run any container with privileged=true.",
			Category:            "security",
			DefaultSeverity:     "high",
			CELExpression:       "has(object.spec) && has(object.spec.containers) && object.spec.containers.exists(c, has(c.securityContext) && has(c.securityContext.privileged) && c.securityContext.privileged == true)",
			DefaultScope:        `{"resourceTypes":["Pod"]}`,
			DefaultAction:       "alert",
			SupportsRemediation: false,
			Rationale:           "Privileged containers weaken isolation boundaries and increase host takeover risk.",
			CreatedBy:           "system",
			IsSystem:            true,
		},
		{
			TemplateID:          "k8s-no-host-namespace-sharing",
			Version:             "1.0.0",
			Name:                "Restrict host namespace sharing",
			Description:         "Flags Pod workloads enabling hostNetwork, hostPID, or hostIPC.",
			Category:            "security",
			DefaultSeverity:     "high",
			CELExpression:       "has(object.spec) && ((has(object.spec.hostNetwork) && object.spec.hostNetwork == true) || (has(object.spec.hostPID) && object.spec.hostPID == true) || (has(object.spec.hostIPC) && object.spec.hostIPC == true))",
			DefaultScope:        `{"resourceTypes":["Pod"]}`,
			DefaultAction:       "alert",
			SupportsRemediation: false,
			Rationale:           "Host namespace sharing broadens lateral movement and process/network visibility on the node.",
			CreatedBy:           "system",
			IsSystem:            true,
		},
	}

	for i := range templates {
		tpl := templates[i]
		if err := db.Where("template_id = ? AND version = ?", tpl.TemplateID, tpl.Version).FirstOrCreate(&tpl).Error; err != nil {
			return fmt.Errorf("failed to seed policy template %s:%s: %w", templates[i].TemplateID, templates[i].Version, err)
		}
	}
	return nil
}

func seedBaselinePolicyInstances(db *gorm.DB) error {
	instances := []models.PolicyInstance{
		{
			TemplateID:      "k8s-no-privileged-container",
			TemplateVersion: "1.0.0",
			InstanceName:    "baseline-no-privileged-container",
			Description:     "Default baseline guard for privileged containers.",
			Enabled:         true,
			Action:          "alert",
			Severity:        "high",
			CreatedBy:       "system",
			UpdatedBy:       "system",
		},
		{
			TemplateID:      "k8s-no-host-namespace-sharing",
			TemplateVersion: "1.0.0",
			InstanceName:    "baseline-no-host-namespace-sharing",
			Description:     "Default baseline guard for host namespace sharing.",
			Enabled:         true,
			Action:          "alert",
			Severity:        "high",
			CreatedBy:       "system",
			UpdatedBy:       "system",
		},
	}

	for i := range instances {
		inst := instances[i]
		if err := db.Where("instance_name = ?", inst.InstanceName).FirstOrCreate(&inst).Error; err != nil {
			return fmt.Errorf("failed to seed policy instance %s: %w", instances[i].InstanceName, err)
		}
	}
	return nil
}
