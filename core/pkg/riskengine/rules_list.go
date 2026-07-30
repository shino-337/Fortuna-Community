package riskengine

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RiskRuleSummary is a minimal view of a rule for the read-only API (Phase 4).
type RiskRuleSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Category    string `json:"category,omitempty"`
	File        string `json:"file,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// ListRuleSummariesFromDir reads YAML rule files from rulesDir and returns summaries (read-only API).
// Returns nil, nil if rulesDir is empty or directory does not exist.
func ListRuleSummariesFromDir(rulesDir string) ([]RiskRuleSummary, error) {
	rulesDir = strings.TrimSpace(rulesDir)
	if rulesDir == "" {
		return nil, nil
	}
	if _, err := os.Stat(rulesDir); os.IsNotExist(err) {
		return nil, nil
	}
	var out []RiskRuleSummary
	err := filepath.WalkDir(rulesDir, func(path string, d fs.DirEntry, err error) error {
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
			return nil // skip unreadable files
		}
		var rule Rule
		if err := yaml.Unmarshal(data, &rule); err != nil {
			return nil
		}
		out = append(out, RiskRuleSummary{
			ID:          rule.ID,
			Name:        rule.Name,
			Severity:    string(rule.Severity),
			Description: rule.Description,
			Category:    string(rule.Category),
			File:        path,
			Enabled:     rule.Enabled,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
