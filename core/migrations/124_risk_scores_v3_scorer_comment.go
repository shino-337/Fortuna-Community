package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration124_RiskScoresV3ScorerComment updates DB documentation: Fortuna uses
// unified scorer V3 as the only authoritative persisted risk_scores row per resource
// (legacy v1/v2 rows are soft-deleted by migration 120).
func Migration124_RiskScoresV3ScorerComment(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		log.Println("[Migration 124] skip: COMMENT ON COLUMN is PostgreSQL-specific")
		return nil
	}
	log.Println("[Migration 124] COMMENT risk_scores.scorer_version (V3 authoritative)")

	stmts := []string{
		`COMMENT ON COLUMN risk_scores.scorer_version IS 'Unified risk scorer version: v3 is authoritative. Legacy v1/v2 rows were soft-deleted (migration 120). New writes use SaveScoreV3 (ON CONFLICT upsert).'`,
		`COMMENT ON COLUMN risk_scores.total_score IS 'Unified risk 0–100 (V3). Severity bands for UI use final_level derived from this score (ADR).'`,
	}
	for _, sql := range stmts {
		if err := execDDL(db, sql); err != nil {
			return err
		}
	}
	log.Println("[Migration 124] ✅ Applied risk_scores column comments")
	return nil
}
