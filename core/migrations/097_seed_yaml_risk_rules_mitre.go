package migrations

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
)

// Migration097_SeedYAMLRiskRulesMITRE seeds/updates risk_rules from core/rules YAML directory.
// This ensures runtime/pod-security rules (and their MITRE ATT&CK tag mappings) are present in DB,
// since the risk evaluation engine prioritizes DB rules over YAML rules.
func Migration097_SeedYAMLRiskRulesMITRE(db *gorm.DB) error {
	log.Println("Running migration 097: Seed YAMLRiskRules (MITRE tags)")

	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.RiskRule{}) {
		// Risk rules table might not be present in some dev/test envs.
		return nil
	}

	rulesDir := resolveRulesDir()
	if rulesDir == "" {
		log.Printf("[migration 097] No rules directory found (FORTUNA_RULES_DIR unset and neither ./rules nor core/rules exist). Skipping.\n")
		return nil
	}

	ye, err := riskengine.NewYAMLEngine(nil, rulesDir)
	if err != nil {
		return fmt.Errorf("failed to create YAML engine from %s: %w", rulesDir, err)
	}

	rules := ye.GetRules()
	if len(rules) == 0 {
		log.Printf("[migration 097] No YAML rules loaded from %s. Skipping.\n", rulesDir)
		return nil
	}

	now := time.Now()
	upserted := 0
	for i := range rules {
		r := rules[i]
		m, err := riskengine.RuleToRiskRule(&r)
		if err != nil {
			log.Printf("[migration 097] Skip %s: RuleToRiskRule error: %v\n", r.ID, err)
			continue
		}

		assign := map[string]interface{}{
			"rule_id":      m.RuleID,
			"name":         m.Name,
			"category":     m.Category,
			"severity":     m.Severity,
			"description":  m.Description,
			"enabled":      m.Enabled,
			"conditions":   m.Conditions,
			"aggregation":  m.Aggregation,
			"base_score":   m.BaseScore,
			"tags":         m.Tags,
			"updated_at":   now,
			"deleted_at":   nil,
		}

		if err := db.Model(&models.RiskRule{}).
			Where("rule_id = ?", m.RuleID).
			Assign(assign).
			FirstOrCreate(&models.RiskRule{RuleID: m.RuleID}).Error; err != nil {
			return fmt.Errorf("failed to upsert risk rule %s: %w", m.RuleID, err)
		}
		upserted++
	}

	log.Printf("[migration 097] Upserted %d risk rules from %s\n", upserted, rulesDir)
	return nil
}

func resolveRulesDir() string {
	// Mirror riskengine.getRulesDirectory() logic in a migration-safe way.
	if dir := os.Getenv("FORTUNA_RULES_DIR"); dir != "" {
		return dir
	}
	if _, err := os.Stat("./rules"); err == nil {
		return "./rules"
	}
	if _, err := os.Stat("core/rules"); err == nil {
		return "core/rules"
	}
	// Docker/workdir fallback (most common in container images).
	if _, err := os.Stat("/app/rules"); err == nil {
		return "/app/rules"
	}
	return ""
}

