package migrations

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
)

// Migration098_SeedYAMLRiskRulesMITREFix cleans up the accidental empty rule_id row (if any)
// and re-upserts all YAML risk rules with MITRE tags into risk_rules.
func Migration098_SeedYAMLRiskRulesMITREFix(db *gorm.DB) error {
	log.Println("Running migration 098: Seed YAMLRiskRules MITRE fix")

	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.RiskRule{}) {
		return nil
	}

	// Cleanup: remove the accidental empty rule_id row created by an earlier bad upsert.
	_ = db.Unscoped().Where("rule_id = ''").Delete(&models.RiskRule{})

	rulesDir := resolveRulesDir()
	if rulesDir == "" {
		log.Println("[migration 098] No rules directory found. Skipping.")
		return nil
	}

	ye, err := riskengine.NewYAMLEngine(nil, rulesDir)
	if err != nil {
		return fmt.Errorf("failed to create YAML engine from %s: %w", rulesDir, err)
	}

	rules := ye.GetRules()
	if len(rules) == 0 {
		log.Printf("[migration 098] No YAML rules loaded from %s. Skipping.\n", rulesDir)
		return nil
	}

	now := time.Now()
	upserted := 0
	for i := range rules {
		r := rules[i]
		m, err := riskengine.RuleToRiskRule(&r)
		if err != nil {
			log.Printf("[migration 098] Skip %s: RuleToRiskRule error: %v\n", r.ID, err)
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

	log.Printf("[migration 098] Upserted %d risk rules from %s\n", upserted, rulesDir)
	return nil
}
