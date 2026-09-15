package riskengine

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// LoadRulesFromDB loads risk rules from risk_rules table. Returns nil, nil if table missing or empty.
func LoadRulesFromDB(db *gorm.DB) ([]Rule, error) {
	if db == nil {
		return nil, nil
	}
	// Check table exists (migration may not have run)
	if !db.Migrator().HasTable(&models.RiskRule{}) {
		return nil, nil
	}
	var rows []models.RiskRule
	if err := db.Where("deleted_at IS NULL").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	rules := make([]Rule, 0, len(rows))
	for i := range rows {
		r, err := riskRuleToRule(&rows[i])
		if err != nil {
			return nil, fmt.Errorf("invalid database rule %s: %w", rows[i].RuleID, err)
		}
		rules = append(rules, *r)
	}
	return rules, nil
}

func riskRuleToRule(m *models.RiskRule) (*Rule, error) {
	var conditions []Condition
	if m.Conditions != "" {
		if err := json.Unmarshal([]byte(m.Conditions), &conditions); err != nil {
			return nil, err
		}
	}
	var tags []string
	if m.Tags != "" {
		_ = json.Unmarshal([]byte(m.Tags), &tags)
	}
	return &Rule{
		ID:          m.RuleID,
		Name:        m.Name,
		Category:    RuleCategory(m.Category),
		Severity:    Severity(m.Severity),
		Description: m.Description,
		Enabled:     m.Enabled,
		Conditions:  conditions,
		Aggregation: AggregationType(m.Aggregation),
		BaseScore:   m.BaseScore,
		Tags:        tags,
	}, nil
}

// RuleToRiskRule converts engine Rule to DB model (for create/update).
func RuleToRiskRule(r *Rule) (*models.RiskRule, error) {
	condJSON, err := json.Marshal(r.Conditions)
	if err != nil {
		return nil, err
	}
	tagsJSON, _ := json.Marshal(r.Tags)
	if r.Tags == nil {
		tagsJSON = []byte("[]")
	}
	return &models.RiskRule{
		RuleID:      r.ID,
		Name:        r.Name,
		Category:    string(r.Category),
		Severity:    string(r.Severity),
		Description: r.Description,
		Enabled:     r.Enabled,
		Conditions:  string(condJSON),
		Aggregation: string(r.Aggregation),
		BaseScore:   r.BaseScore,
		Tags:        string(tagsJSON),
	}, nil
}
