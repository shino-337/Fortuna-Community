package migrations

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
)

// Migration100_RefreshYAMLRiskRulesMITRE refreshes risk_rules table from core/rules YAML
// so any newly added/updated YAML rules (including ATT&CK tags) appear immediately in DB/UI.
func Migration100_RefreshYAMLRiskRulesMITRE(db *gorm.DB) error {
	log.Println("Running migration 100: Refresh YAMLRiskRules (MITRE tags)")

	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&models.RiskRule{}) {
		return nil
	}

	_ = db.Unscoped().Where("rule_id = ''").Delete(&models.RiskRule{})

	rulesDir := resolveRulesDir()
	if rulesDir == "" {
		log.Println("[migration 100] No rules directory found. Skipping.")
		return nil
	}

	ye, err := riskengine.NewYAMLEngine(nil, rulesDir)
	if err != nil {
		return fmt.Errorf("failed to create YAML engine from %s: %w", rulesDir, err)
	}

	rules := ye.GetRules()
	if len(rules) == 0 {
		log.Printf("[migration 100] No YAML rules loaded from %s. Skipping.\n", rulesDir)
		return nil
	}

	now := time.Now()
	upserted := 0
	for i := range rules {
		r := rules[i]
		m, err := riskengine.RuleToRiskRule(&r)
		if err != nil {
			log.Printf("[migration 100] Skip %s: RuleToRiskRule error: %v\n", r.ID, err)
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

	log.Printf("[migration 100] Upserted/Refreshed %d risk rules from %s\n", upserted, rulesDir)
	return nil
}

