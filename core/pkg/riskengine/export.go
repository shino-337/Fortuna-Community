package riskengine

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

const riskRulesSubdir = "risk"

// GetRiskRulesExportDir returns the directory for persisting risk rules as YAML (FORTUNA_RULES_DIR/risk).
// Empty if FORTUNA_RULES_DIR is not set. Used so rules survive deploy/rebuild when folder is mounted or synced.
func GetRiskRulesExportDir() string {
	base := os.Getenv("FORTUNA_RULES_DIR")
	if base == "" {
		return ""
	}
	return filepath.Join(base, riskRulesSubdir)
}

// ExportRuleToFile writes a single rule to dir/<id>.yaml. Creates dir if needed. No-op if dir is empty.
func ExportRuleToFile(rule *Rule, dir string) error {
	if dir == "" || rule == nil || rule.ID == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(rule)
	if err != nil {
		return err
	}
	fpath := filepath.Join(dir, rule.ID+".yaml")
	if err := os.WriteFile(fpath, data, 0644); err != nil {
		return err
	}
	log.Printf("[RiskRules] Exported rule %s to %s", rule.ID, fpath)
	return nil
}

// RemoveRuleFile removes the YAML file for rule id from dir. No-op if dir is empty.
func RemoveRuleFile(dir, ruleID string) error {
	if dir == "" || ruleID == "" {
		return nil
	}
	fpath := filepath.Join(dir, ruleID+".yaml")
	if err := os.Remove(fpath); err != nil && !os.IsNotExist(err) {
		return err
	}
	log.Printf("[RiskRules] Removed rule file %s", fpath)
	return nil
}

// LoadRiskRulesFromExportDir reads all YAML files from dir (FORTUNA_RULES_DIR/risk). Returns nil if dir empty or missing.
func LoadRiskRulesFromExportDir(dir string) ([]Rule, error) {
	if dir == "" {
		return nil, nil
	}
	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !fi.IsDir() {
		return nil, nil
	}
	var rules []Rule
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[RiskRules] Skip %s: %v", path, err)
			return nil
		}
		var rule Rule
		if err := yaml.Unmarshal(data, &rule); err != nil {
			log.Printf("[RiskRules] Skip %s: invalid YAML: %v", path, err)
			return nil
		}
		if rule.ID == "" {
			log.Printf("[RiskRules] Skip %s: missing id", path)
			return nil
		}
		rules = append(rules, rule)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// SeedRiskRulesFromExportDir inserts rules from FORTUNA_RULES_DIR/risk into risk_rules table when DB has no rules.
// Call once at startup so that after a fresh deploy, rules stored in the folder are restored to the DB.
func SeedRiskRulesFromExportDir(db *gorm.DB) (int, error) {
	if db == nil {
		return 0, nil
	}
	dir := GetRiskRulesExportDir()
	if dir == "" {
		return 0, nil
	}
	if !db.Migrator().HasTable(&models.RiskRule{}) {
		return 0, nil
	}
	var count int64
	if err := db.Model(&models.RiskRule{}).Where("deleted_at IS NULL").Count(&count).Error; err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, nil // DB already has rules; do not overwrite
	}
	rules, err := LoadRiskRulesFromExportDir(dir)
	if err != nil || len(rules) == 0 {
		return 0, err
	}
	inserted := 0
	for i := range rules {
		m, err := RuleToRiskRule(&rules[i])
		if err != nil {
			log.Printf("[RiskRules] Seed skip %s: %v", rules[i].ID, err)
			continue
		}
		if err := db.Create(m).Error; err != nil {
			log.Printf("[RiskRules] Seed create %s: %v", rules[i].ID, err)
			continue
		}
		inserted++
	}
	if inserted > 0 {
		log.Printf("[RiskRules] Seeded %d rules from %s (DB was empty)", inserted, dir)
	}
	return inserted, nil
}
