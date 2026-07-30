package migrations

import (
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration120_SoftDeleteLegacyRiskScoreRows marks risk_scores rows whose
// scorer_version is explicitly v1 or v2 as deleted. V3 upserts (SaveScoreV3)
// revive the same logical row via ON CONFLICT, including clearing deleted_at.
//
// Run order: deploy with V3 backfill job / POST sync before or shortly after this
// migration so UI does not stay empty for insight-backed resources.
func Migration120_SoftDeleteLegacyRiskScoreRows(db *gorm.DB) error {
	log.Println("[Migration 120] Soft-deleting risk_scores rows with legacy scorer_version v1/v2")

	now := time.Now()
	res := db.Model(&models.RiskScore{}).
		Where("deleted_at IS NULL").
		Where("LOWER(TRIM(scorer_version)) IN ?", []string{"v1", "v2"}).
		Update("deleted_at", now)
	if res.Error != nil {
		return res.Error
	}
	log.Printf("[Migration 120] ✅ Soft-deleted %d legacy risk_score rows", res.RowsAffected)
	return nil
}
