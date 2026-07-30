package migrations

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// Migration125_BackfillInsightCVEIDRuleIDs backfills insights.cve_id for rows
// created by the risk engine from YAML rules.
//
// Date: 2026-04-22
// Description:
//   The risk engine stores rule.ID in insights.cve_id (see riskengine.createInsight).
//   Earlier versions matched rules to insights via LIKE on insight_type / description,
//   which could produce false positives and prevented efficient indexed queries.
//   This migration finds insights whose insight_type equals a known rule category and
//   whose description starts with the rule name, and sets cve_id = rule.ID when it is
//   currently empty. Only non-vulnerability rows are touched (vulnerability insights
//   already have a real CVE-ID in that column).
//
// Tables Affected:
//   - insights: updates cve_id column for matching rows
//
// Rollback Plan:
//   UPDATE insights SET cve_id = '' WHERE cve_id IN (...rule ids...) AND insight_type != 'vulnerability';
func Migration125_BackfillInsightCVEIDRuleIDs(db *gorm.DB) error {
	log.Println("[Migration 125] Starting: Backfill insights.cve_id with YAML rule IDs")

	if !db.Migrator().HasTable("insights") {
		log.Println("[Migration 125] insights table does not exist, skipping")
		return nil
	}

	type yamlRule struct {
		ID       string `yaml:"id"`
		Name     string `yaml:"name"`
		Category string `yaml:"category"`
	}

	rulesDir := os.Getenv("FORTUNA_RULES_DIR")
	if rulesDir == "" {
		if _, err := os.Stat("./rules"); err == nil {
			rulesDir = "./rules"
		} else if _, err := os.Stat("core/rules"); err == nil {
			rulesDir = "core/rules"
		}
	}
	if rulesDir == "" {
		log.Println("[Migration 125] No rules directory found; skipping backfill")
		return nil
	}

	var rules []yamlRule
	_ = filepath.WalkDir(rulesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var r yamlRule
		if err := yaml.Unmarshal(data, &r); err != nil || r.ID == "" {
			return nil
		}
		rules = append(rules, r)
		return nil
	})

	if len(rules) == 0 {
		log.Println("[Migration 125] No YAML rules loaded; skipping backfill")
		return nil
	}

	totalUpdated := int64(0)
	for _, rule := range rules {
		if rule.ID == "" || rule.Name == "" {
			continue
		}
		descPrefix := rule.Name + ":"
		result := db.Exec(
			`UPDATE insights
			 SET cve_id = ?, updated_at = NOW()
			 WHERE cve_id = ''
			   AND insight_type != 'vulnerability'
			   AND insight_type = ?
			   AND description LIKE ?
			   AND deleted_at IS NULL`,
			rule.ID,
			rule.Category,
			descPrefix+"%",
		)
		if result.Error != nil {
			log.Printf("[Migration 125] WARNING: backfill for rule %s failed: %v", rule.ID, result.Error)
			continue
		}
		if result.RowsAffected > 0 {
			log.Printf("[Migration 125] Backfilled %d rows for rule %s", result.RowsAffected, rule.ID)
			totalUpdated += result.RowsAffected
		}
	}

	log.Printf("[Migration 125] Completed: backfilled %d total insight rows", totalUpdated)

	if !indexExists(db, "insights", "idx_insights_cve_id") {
		log.Println("[Migration 125] Creating index idx_insights_cve_id on insights(cve_id)")
		if err := db.Exec(`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_insights_cve_id ON insights(cve_id)`).Error; err != nil {
			return fmt.Errorf("failed to create index idx_insights_cve_id: %w", err)
		}
	}

	return nil
}

func indexExists(db *gorm.DB, table, indexName string) bool {
	var exists bool
	db.Raw(
		"SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = ? AND indexname = ?)",
		table, indexName,
	).Scan(&exists)
	return exists
}
