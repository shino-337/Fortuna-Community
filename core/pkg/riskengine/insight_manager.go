package riskengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/evidence"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
)

// InsightManager manages insight creation and updates
type InsightManager struct {
	db *gorm.DB
}

// NewInsightManager creates a new InsightManager
func NewInsightManager(db *gorm.DB) *InsightManager {
	return &InsightManager{db: db}
}

// isExempted checks whether an active (non-expired), cluster-qualified exception
// policy exists. Empty cluster ownership fails closed: an exception must never be
// applied to an ambiguous legacy finding merely because ResourceUID matches.
func isExempted(tx *gorm.DB, clusterID, resourceUID, cveID, insightType string) bool {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID == "" {
		return false
	}
	now := time.Now()
	var count int64
	if err := tx.Model(&models.ExceptionPolicy{}).
		Where("cluster_id = ? AND resource_uid = ? AND cve_id = ? AND insight_type = ? AND deleted_at IS NULL AND (expires_at IS NULL OR expires_at > ?)",
			clusterID, resourceUID, cveID, insightType, now).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// isUniqueViolation returns true when err is a duplicate-key / unique-index violation
// (PostgreSQL 23505, SQLite UNIQUE constraint failed).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "23505") ||
		strings.Contains(s, "duplicate key") ||
		strings.Contains(s, "UNIQUE constraint failed")
}

// createOrUpdateInsightTx performs the actual work within a transaction
// UPDATED: Uses new Insight schema (no AffectedResources JSONB, direct resource fields)
func (m *InsightManager) createOrUpdateInsightTx(tx *gorm.DB, insight *models.Insight) error {
	// Use direct resource fields (no JSONB parsing needed)
	if insight.ResourceUID == "" {
		return fmt.Errorf("resource_uid is required")
	}
	insight.CVEID = strings.TrimSpace(insight.CVEID)

	// For vulnerability insights, use more precise deduplication: resource_uid + cve_id
	if insight.InsightType == "vulnerability" && insight.CVEID != "" {
		var existingVuln models.Insight

		// Use efficient composite index query
		query := tx.Where("insight_type = ? AND resource_uid = ? AND cve_id = ? AND (status = ? OR status IS NULL) AND deleted_at IS NULL",
			"vulnerability", insight.ResourceUID, insight.CVEID, "active")

		if query.First(&existingVuln).Error == nil {
			// Found existing - update it
			needsUpdate := false
			if existingVuln.Description != insight.Description {
				existingVuln.Description = insight.Description
				needsUpdate = true
			}
			if existingVuln.Recommendation != insight.Recommendation {
				existingVuln.Recommendation = insight.Recommendation
				needsUpdate = true
			}
			if existingVuln.CVSS != insight.CVSS {
				existingVuln.CVSS = insight.CVSS
				needsUpdate = true
			}
			if existingVuln.Severity != insight.Severity {
				existingVuln.Severity = insight.Severity
				needsUpdate = true
			}
			if needsUpdate {
				existingVuln.UpdatedAt = time.Now()
				if err := tx.Save(&existingVuln).Error; err != nil {
					return fmt.Errorf("failed to update insight: %w", err)
				}
				log.Printf("[InsightManager] Updated vulnerability insight ID=%d (resource_uid=%s, cve_id=%s)",
					existingVuln.ID, insight.ResourceUID, insight.CVEID)
			} else {
				log.Printf("[InsightManager] Vulnerability insight already exists (ID=%d, resource_uid=%s, cve_id=%s), no update needed",
					existingVuln.ID, insight.ResourceUID, insight.CVEID)
			}
			return nil
		}

		// Check for resolved/dismissed vulnerability insights
		queryResolved := tx.Where("insight_type = ? AND resource_uid = ? AND cve_id = ? AND status IN (?, ?) AND deleted_at IS NULL",
			"vulnerability", insight.ResourceUID, insight.CVEID, "resolved", "dismissed")

		if queryResolved.First(&existingVuln).Error == nil {
			// RP-5: respect active exception policies — keep dismissed if exempted.
			if existingVuln.Status == "dismissed" && isExempted(tx, existingVuln.ClusterID, insight.ResourceUID, insight.CVEID, "vulnerability") {
				log.Printf("[InsightManager] Keeping vulnerability insight ID=%d dismissed (exception policy active, cluster_id=%s, resource_uid=%s, cve_id=%s)",
					existingVuln.ID, existingVuln.ClusterID, insight.ResourceUID, insight.CVEID)
				return nil
			}
			existingVuln.Status = "active"
			existingVuln.Description = insight.Description
			existingVuln.Recommendation = insight.Recommendation
			existingVuln.CVSS = insight.CVSS
			existingVuln.AffectedComponent = insight.AffectedComponent
			existingVuln.AffectedVersion = insight.AffectedVersion
			existingVuln.FixedVersion = insight.FixedVersion
			existingVuln.Severity = insight.Severity
			existingVuln.DetectedAt = time.Now()
			existingVuln.UpdatedAt = time.Now()
			if err := tx.Save(&existingVuln).Error; err != nil {
				return fmt.Errorf("failed to re-activate insight: %w", err)
			}
			log.Printf("[InsightManager] Re-activated vulnerability insight ID=%d (resource_uid=%s, cve_id=%s)",
				existingVuln.ID, insight.ResourceUID, insight.CVEID)
			return nil
		}
	}

	// Non-vulnerability insights that store a logical key in cve_id (YAML rule.ID, capability id, …):
	// DB enforces UNIQUE (resource_uid, cve_id, insight_type) (idx_insights_unique_resource_cve_type_all).
	// Deduplicate on that triple — not on title alone — so re-runs, duplicate API objects with the same UID,
	// or legacy rows with empty/unexpected status still upsert instead of raising 23505.
	if insight.InsightType != "vulnerability" && insight.CVEID != "" {
		cveKey := insight.CVEID
		var existingKey models.Insight
		if tx.Where("insight_type = ? AND resource_uid = ? AND cve_id = ? AND deleted_at IS NULL",
			insight.InsightType, insight.ResourceUID, cveKey).First(&existingKey).Error == nil {
			wasResolvedOrDismissed := existingKey.Status == "resolved" || existingKey.Status == "dismissed"
			// RP-5: respect active exception policies — keep dismissed if exempted.
			if existingKey.Status == "dismissed" && isExempted(tx, existingKey.ClusterID, insight.ResourceUID, cveKey, insight.InsightType) {
				log.Printf("[InsightManager] Keeping insight ID=%d dismissed (exception policy active, cluster_id=%s, resource_uid=%s, type=%s, cve_id=%s)",
					existingKey.ID, existingKey.ClusterID, insight.ResourceUID, insight.InsightType, cveKey)
				return nil
			}
			existingKey.Status = "active"
			existingKey.Severity = insight.Severity
			existingKey.Description = insight.Description
			existingKey.Recommendation = insight.Recommendation
			existingKey.Title = insight.Title
			existingKey.UpdatedAt = time.Now()
			if wasResolvedOrDismissed {
				existingKey.DetectedAt = time.Now()
			}
			if err := tx.Save(&existingKey).Error; err != nil {
				return fmt.Errorf("failed to update insight (resource+cve+type key): %w", err)
			}
			log.Printf("[InsightManager] Updated insight ID=%d (resource_uid=%s, insight_type=%s, cve_id=%s)",
				existingKey.ID, insight.ResourceUID, insight.InsightType, cveKey)
			return nil
		}
	}

	// Same DB unique key (resource_uid, cve_id, insight_type): when cve_id is empty, Postgres/SQLite
	// still allow only one row per (uid, '', type). Title-based dedup below misses if the title changes.
	if insight.CVEID == "" {
		var existingEmptyCVE models.Insight
		if tx.Where("insight_type = ? AND resource_uid = ? AND deleted_at IS NULL AND (cve_id IS NULL OR cve_id = '')",
			insight.InsightType, insight.ResourceUID).First(&existingEmptyCVE).Error == nil {
			wasResolvedOrDismissed := existingEmptyCVE.Status == "resolved" || existingEmptyCVE.Status == "dismissed"
			if existingEmptyCVE.Status == "dismissed" && isExempted(tx, existingEmptyCVE.ClusterID, insight.ResourceUID, "", insight.InsightType) {
				log.Printf("[InsightManager] Keeping insight ID=%d dismissed (exception policy active, cluster_id=%s, resource_uid=%s, type=%s, empty cve_id)",
					existingEmptyCVE.ID, existingEmptyCVE.ClusterID, insight.ResourceUID, insight.InsightType)
				return nil
			}
			existingEmptyCVE.Status = "active"
			existingEmptyCVE.Severity = insight.Severity
			existingEmptyCVE.Description = insight.Description
			existingEmptyCVE.Recommendation = insight.Recommendation
			existingEmptyCVE.Title = insight.Title
			existingEmptyCVE.CVSS = insight.CVSS
			existingEmptyCVE.AffectedComponent = insight.AffectedComponent
			existingEmptyCVE.AffectedVersion = insight.AffectedVersion
			existingEmptyCVE.FixedVersion = insight.FixedVersion
			existingEmptyCVE.UpdatedAt = time.Now()
			if wasResolvedOrDismissed {
				existingEmptyCVE.DetectedAt = time.Now()
			}
			if err := tx.Save(&existingEmptyCVE).Error; err != nil {
				return fmt.Errorf("failed to update insight (resource+empty cve+type key): %w", err)
			}
			log.Printf("[InsightManager] Updated insight ID=%d (resource_uid=%s, insight_type=%s, empty cve_id)",
				existingEmptyCVE.ID, insight.ResourceUID, insight.InsightType)
			return nil
		}
	}

	// For other non-vulnerability insights, deduplicate by resource_uid + insight_type + title.
	keyQuery := tx.Where(
		"resource_uid = ? AND insight_type = ? AND title = ? AND deleted_at IS NULL",
		insight.ResourceUID, insight.InsightType, insight.Title,
	)

	var existing models.Insight
	if keyQuery.Where("status = ? OR status IS NULL", "active").First(&existing).Error == nil {
		needsUpdate := false
		if existing.Description != insight.Description {
			existing.Description = insight.Description
			needsUpdate = true
		}
		if existing.Recommendation != insight.Recommendation {
			existing.Recommendation = insight.Recommendation
			needsUpdate = true
		}
		if existing.Severity != insight.Severity {
			existing.Severity = insight.Severity
			needsUpdate = true
		}
		if needsUpdate {
			existing.UpdatedAt = time.Now()
			if err := tx.Save(&existing).Error; err != nil {
				return fmt.Errorf("failed to update insight: %w", err)
			}
			log.Printf("[InsightManager] Updated insight ID=%d for %s/%s/%s",
				existing.ID, insight.ResourceType, insight.ResourceNamespace, insight.ResourceName)
		} else {
			log.Printf("[InsightManager] Insight already exists (ID=%d), no update needed", existing.ID)
		}
		return nil
	}

	var resolved models.Insight
	if keyQuery.Where("status IN (?, ?)", "resolved", "dismissed").First(&resolved).Error == nil {
		// RP-5: respect active exception policies — keep dismissed if exempted.
		if resolved.Status == "dismissed" && isExempted(tx, resolved.ClusterID, insight.ResourceUID, insight.CVEID, insight.InsightType) {
			log.Printf("[InsightManager] Keeping insight ID=%d dismissed (exception policy active, cluster_id=%s, resource_uid=%s, type=%s)",
				resolved.ID, resolved.ClusterID, insight.ResourceUID, insight.InsightType)
			return nil
		}
		resolved.Status = "active"
		resolved.Severity = insight.Severity
		resolved.Description = insight.Description
		resolved.Recommendation = insight.Recommendation
		resolved.UpdatedAt = time.Now()
		resolved.DetectedAt = time.Now()
		if err := tx.Save(&resolved).Error; err != nil {
			return fmt.Errorf("failed to re-activate insight: %w", err)
		}
		log.Printf("[InsightManager] Re-activated insight ID=%d", resolved.ID)
		return nil
	}

	return m.createInsightTx(tx, insight)
}

// mergeInsightAfterUniqueConflict loads the row matching the DB unique index
// (resource_uid, cve_id, insight_type) and applies the same merge semantics as createOrUpdateInsightTx.
// Used when tx.Create hits a unique violation (race or legacy row shape).
func (m *InsightManager) mergeInsightAfterUniqueConflict(tx *gorm.DB, insight *models.Insight) error {
	var existing models.Insight
	q := tx.Where("resource_uid = ? AND insight_type = ? AND deleted_at IS NULL", insight.ResourceUID, insight.InsightType)
	if insight.CVEID == "" {
		q = q.Where("(cve_id IS NULL OR cve_id = '')")
	} else {
		q = q.Where("cve_id = ?", insight.CVEID)
	}
	if err := q.First(&existing).Error; err != nil {
		return err
	}

	if insight.InsightType == "vulnerability" && insight.CVEID != "" {
		if existing.Status == "dismissed" && isExempted(tx, existing.ClusterID, insight.ResourceUID, insight.CVEID, "vulnerability") {
			log.Printf("[InsightManager] Keeping vulnerability insight ID=%d dismissed after unique conflict (cluster_id=%s, resource_uid=%s, cve_id=%s)",
				existing.ID, existing.ClusterID, insight.ResourceUID, insight.CVEID)
			return nil
		}
		if existing.Status == "resolved" || existing.Status == "dismissed" {
			existing.Status = "active"
			existing.Description = insight.Description
			existing.Recommendation = insight.Recommendation
			existing.CVSS = insight.CVSS
			existing.AffectedComponent = insight.AffectedComponent
			existing.AffectedVersion = insight.AffectedVersion
			existing.FixedVersion = insight.FixedVersion
			existing.Severity = insight.Severity
			existing.DetectedAt = time.Now()
			existing.UpdatedAt = time.Now()
			if err := tx.Save(&existing).Error; err != nil {
				return fmt.Errorf("failed to re-activate insight after unique conflict: %w", err)
			}
			log.Printf("[InsightManager] Re-activated vulnerability insight ID=%d after unique conflict (resource_uid=%s, cve_id=%s)",
				existing.ID, insight.ResourceUID, insight.CVEID)
			return nil
		}
		needsUpdate := false
		if existing.Description != insight.Description {
			existing.Description = insight.Description
			needsUpdate = true
		}
		if existing.Recommendation != insight.Recommendation {
			existing.Recommendation = insight.Recommendation
			needsUpdate = true
		}
		if existing.CVSS != insight.CVSS {
			existing.CVSS = insight.CVSS
			needsUpdate = true
		}
		if existing.Severity != insight.Severity {
			existing.Severity = insight.Severity
			needsUpdate = true
		}
		if needsUpdate {
			existing.UpdatedAt = time.Now()
			if err := tx.Save(&existing).Error; err != nil {
				return fmt.Errorf("failed to update insight after unique conflict: %w", err)
			}
			log.Printf("[InsightManager] Updated vulnerability insight ID=%d after unique conflict (resource_uid=%s, cve_id=%s)",
				existing.ID, insight.ResourceUID, insight.CVEID)
		}
		return nil
	}

	cveKey := insight.CVEID
	wasResolvedOrDismissed := existing.Status == "resolved" || existing.Status == "dismissed"
	if existing.Status == "dismissed" && isExempted(tx, existing.ClusterID, insight.ResourceUID, cveKey, insight.InsightType) {
		log.Printf("[InsightManager] Keeping insight ID=%d dismissed after unique conflict (cluster_id=%s, resource_uid=%s, type=%s, cve_id=%s)",
			existing.ID, existing.ClusterID, insight.ResourceUID, insight.InsightType, cveKey)
		return nil
	}
	existing.Status = "active"
	existing.Severity = insight.Severity
	existing.Description = insight.Description
	existing.Recommendation = insight.Recommendation
	existing.Title = insight.Title
	existing.CVSS = insight.CVSS
	existing.AffectedComponent = insight.AffectedComponent
	existing.AffectedVersion = insight.AffectedVersion
	existing.FixedVersion = insight.FixedVersion
	existing.UpdatedAt = time.Now()
	if wasResolvedOrDismissed {
		existing.DetectedAt = time.Now()
	}
	if err := tx.Save(&existing).Error; err != nil {
		return fmt.Errorf("failed to update insight after unique conflict: %w", err)
	}
	log.Printf("[InsightManager] Updated insight ID=%d after unique conflict (resource_uid=%s, type=%s, cve_id=%q)",
		existing.ID, insight.ResourceUID, insight.InsightType, cveKey)
	return nil
}

// createInsightTx creates a new insight within a transaction
func (m *InsightManager) createInsightTx(tx *gorm.DB, insight *models.Insight) error {
	// Ensure JSONB fields always contain valid JSON; mask sensitive keys (Phase 3 evidence masking).
	if strings.TrimSpace(insight.Evidence) == "" {
		insight.Evidence = "{}"
	} else {
		insight.Evidence = evidence.MaskSensitiveInJSON(insight.Evidence)
	}
	if strings.TrimSpace(insight.ViolatedRules) == "" {
		insight.ViolatedRules = "[]"
	} else {
		insight.ViolatedRules = evidence.MaskSensitiveInJSON(insight.ViolatedRules)
	}
	// Remediation is JSONB; empty or invalid string causes PostgreSQL "invalid input syntax for type json".
	if strings.TrimSpace(insight.Remediation) == "" {
		insight.Remediation = "{}"
	} else if !json.Valid([]byte(insight.Remediation)) {
		insight.Remediation = "{}"
	}
	// INSERT unique violation aborts the whole Postgres transaction unless we roll back
	// to a savepoint first; otherwise mergeInsightAfterUniqueConflict hits 25P02.
	const createInsightSP = "sp_create_insight"
	if err := tx.SavePoint(createInsightSP).Error; err != nil {
		return fmt.Errorf("savepoint before insert insight: %w", err)
	}
	if err := tx.Create(insight).Error; err != nil {
		if rbErr := tx.RollbackTo(createInsightSP).Error; rbErr != nil {
			return fmt.Errorf("failed to create insight: %w (rollback to savepoint: %v)", err, rbErr)
		}
		if isUniqueViolation(err) {
			if mergeErr := m.mergeInsightAfterUniqueConflict(tx, insight); mergeErr == nil {
				return nil
			} else if !errors.Is(mergeErr, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to create insight: %w (merge after unique conflict: %v)", err, mergeErr)
			}
		}
		return fmt.Errorf("failed to create insight: %w", err)
	}
	log.Printf("[InsightManager] Created new insight ID=%d: type=%s, severity=%s, resource=%s/%s/%s",
		insight.ID, insight.InsightType, insight.Severity,
		insight.ResourceType, insight.ResourceNamespace, insight.ResourceName)
	return nil
}

// runRiskScoreCalculation calculates and saves risk score for one resource (blocking).
// Used from scheduleRiskScoreCalculation in a goroutine so API is not blocked.
// Unified standard: run V3 scorer as the single authoritative scoring path.
func (m *InsightManager) runRiskScoreCalculation(ctx context.Context, resourceUID string) {
	if resourceUID == "" {
		return
	}

	// V3 unified scorer (authoritative)
	scorerV3 := risk.NewUnifiedScorerV3(m.db)
	scoreV3, err := scorerV3.CalculateScoreV3(ctx, resourceUID)
	if err != nil {
		log.Printf("[InsightManager] V3 risk score calculation failed for resource_uid=%s: %v", resourceUID, err)
		return
	}
	if err := scorerV3.SaveScoreV3(ctx, scoreV3); err != nil {
		log.Printf("[InsightManager] V3 risk score save failed for resource_uid=%s: %v", resourceUID, err)
		return
	}
	log.Printf("[InsightManager] V3 risk score updated for resource_uid=%s total=%.1f priority=%s", resourceUID, scoreV3.TotalScore, scoreV3.PriorityLevel)
}

// scheduleRiskScoreCalculation schedules risk score calculation for the resource of the given insight.
// Runs in a goroutine so insight create/update API response is not blocked.
func (m *InsightManager) scheduleRiskScoreCalculation(insight *models.Insight) {
	if insight == nil || insight.ResourceUID == "" {
		return
	}
	go m.runRiskScoreCalculation(context.Background(), insight.ResourceUID)
}

// CreateOrUpdateInsight creates or updates an insight (public API)
func (m *InsightManager) CreateOrUpdateInsight(insight *models.Insight) error {
	err := m.db.Transaction(func(tx *gorm.DB) error {
		return m.createOrUpdateInsightTx(tx, insight)
	})
	if err == nil && insight != nil && insight.ResourceUID != "" {
		m.scheduleRiskScoreCalculation(insight)
	}
	return err
}

// BatchCreateOrUpdateInsights processes multiple insights in batch using PostgreSQL UPSERT
func (m *InsightManager) BatchCreateOrUpdateInsights(insights []*models.Insight) error {
	if len(insights) == 0 {
		return nil
	}

	err := m.db.Transaction(func(tx *gorm.DB) error {
		// Separate vulnerability insights from other types for different upsert strategies
		vulnInsights := make([]*models.Insight, 0)
		otherInsights := make([]*models.Insight, 0)

		for _, insight := range insights {
			if insight.InsightType == "vulnerability" && insight.CVEID != "" {
				vulnInsights = append(vulnInsights, insight)
			} else {
				otherInsights = append(otherInsights, insight)
			}
		}

		// Batch upsert vulnerability insights using PostgreSQL ON CONFLICT
		if len(vulnInsights) > 0 {
			// Use raw SQL for efficient batch UPSERT with proper conflict handling
			const batchSize = 100
			for i := 0; i < len(vulnInsights); i += batchSize {
				end := i + batchSize
				if end > len(vulnInsights) {
					end = len(vulnInsights)
				}
				batch := vulnInsights[i:end]

				// Build bulk INSERT with ON CONFLICT for vulnerability insights
				if err := m.batchUpsertVulnerabilityInsights(tx, batch); err != nil {
					return fmt.Errorf("batch upsert vulnerability insights: %w", err)
				}
			}
			log.Printf("[InsightManager] Batch upserted %d vulnerability insights", len(vulnInsights))
		}

		// Process other insights using existing logic (fallback for non-vulnerability)
		for _, insight := range otherInsights {
			if err := m.createOrUpdateInsightTx(tx, insight); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Schedule risk score calculation for all affected resources (after commit)
	seen := make(map[string]struct{})
	for _, insight := range insights {
		if insight != nil && insight.ResourceUID != "" {
			seen[insight.ResourceUID] = struct{}{}
		}
	}
	for uid := range seen {
		go m.runRiskScoreCalculation(context.Background(), uid)
	}
	if n := len(seen); n > 0 {
		log.Printf("[InsightManager] Scheduled risk score calculation for %d unique resources", n)
	}
	return nil
}

// batchUpsertVulnerabilityInsights performs efficient batch UPSERT for vulnerability insights
func (m *InsightManager) batchUpsertVulnerabilityInsights(tx *gorm.DB, insights []*models.Insight) error {
	if len(insights) == 0 {
		return nil
	}

	// The bulk path below is PostgreSQL-specific ($n placeholders, NOW(), partial UNIQUE ON CONFLICT).
	// SQLite in-memory tests (and any non-Postgres DB) use per-row ORM upserts instead.
	if tx.Dialector == nil || tx.Dialector.Name() != "postgres" {
		for _, insight := range insights {
			if err := m.createOrUpdateInsightTx(tx, insight); err != nil {
				return err
			}
		}
		return nil
	}

	// Deduplicate insights by (resource_uid, cve_id, insight_type) to avoid ON CONFLICT errors.
	// The cluster-qualified uniqueness migration is intentionally deferred to #46.
	seen := make(map[string]*models.Insight)
	for _, insight := range insights {
		key := fmt.Sprintf("%s:%s:%s", insight.ResourceUID, insight.CVEID, insight.InsightType)
		if existing, exists := seen[key]; exists {
			// Keep the one with higher CVSS or more recent detected_at
			if insight.CVSS > existing.CVSS || (insight.CVSS == existing.CVSS && insight.DetectedAt.After(existing.DetectedAt)) {
				seen[key] = insight
			}
		} else {
			seen[key] = insight
		}
	}

	// Convert map back to slice
	deduplicated := make([]*models.Insight, 0, len(seen))
	for _, insight := range seen {
		deduplicated = append(deduplicated, insight)
	}

	// Prepare data for bulk insert
	now := time.Now()
	values := make([]interface{}, 0, len(deduplicated)*23)
	placeholders := make([]string, 0, len(deduplicated))

	paramIndex := 1
	for _, insight := range deduplicated {
		// Set timestamps
		if insight.DetectedAt.IsZero() {
			insight.DetectedAt = now
		}
		if insight.Status == "" {
			insight.Status = "active"
		}

		// Build placeholder for this row (23 columns: cluster_id + original fields).
		placeholder := fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			paramIndex, paramIndex+1, paramIndex+2, paramIndex+3, paramIndex+4, paramIndex+5,
			paramIndex+6, paramIndex+7, paramIndex+8, paramIndex+9, paramIndex+10, paramIndex+11,
			paramIndex+12, paramIndex+13, paramIndex+14, paramIndex+15, paramIndex+16, paramIndex+17,
			paramIndex+18, paramIndex+19, paramIndex+20, paramIndex+21, paramIndex+22)
		placeholders = append(placeholders, placeholder)

		values = append(values,
			insight.ClusterID,
			insight.ResourceType,
			insight.ResourceNamespace,
			insight.ResourceName,
			insight.ResourceUID,
			insight.InsightType,
			insight.Severity,
			insight.Title,
			insight.Description,
			insight.Status,
			insight.Recommendation,
			insight.MatchConfidence,
			insight.ComponentConfidence,
			insight.SBOMConfidence,
			insight.FinalRiskConfidence,
			insight.Degraded,
			insight.CVEID,
			insight.CVSS,
			insight.AffectedComponent,
			insight.AffectedVersion,
			insight.DetectedAt,
			now, // created_at
			now, // updated_at
		)

		paramIndex += 23
	}

	query := fmt.Sprintf(`
INSERT INTO insights (
	cluster_id, resource_type, resource_namespace, resource_name, resource_uid,
	insight_type, severity, title, description, status, recommendation,
	match_confidence, component_confidence, sbom_confidence, final_risk_confidence, degraded,
	cve_id, cvss, affected_component, affected_version,
	detected_at, created_at, updated_at
) VALUES %s
ON CONFLICT (resource_uid, cve_id, insight_type)
WHERE deleted_at IS NULL
DO UPDATE SET
	description = EXCLUDED.description,
	recommendation = EXCLUDED.recommendation,
	match_confidence = CASE
		WHEN EXCLUDED.updated_at >= insights.updated_at THEN EXCLUDED.match_confidence
		ELSE insights.match_confidence
	END,
	component_confidence = CASE
		WHEN EXCLUDED.updated_at >= insights.updated_at THEN EXCLUDED.component_confidence
		ELSE insights.component_confidence
	END,
	sbom_confidence = CASE
		WHEN EXCLUDED.updated_at >= insights.updated_at THEN EXCLUDED.sbom_confidence
		ELSE insights.sbom_confidence
	END,
	final_risk_confidence = CASE
		WHEN EXCLUDED.updated_at >= insights.updated_at THEN EXCLUDED.final_risk_confidence
		ELSE insights.final_risk_confidence
	END,
	degraded = CASE
		WHEN EXCLUDED.updated_at >= insights.updated_at THEN EXCLUDED.degraded
		ELSE insights.degraded
	END,
	cvss = EXCLUDED.cvss,
	severity = EXCLUDED.severity,
	affected_version = EXCLUDED.affected_version,
	status = CASE
		WHEN insights.status = 'dismissed' AND COALESCE(insights.cluster_id, '') <> '' AND EXISTS (
			SELECT 1 FROM exception_policies ep
			WHERE ep.cluster_id = insights.cluster_id
			  AND ep.resource_uid = insights.resource_uid
			  AND ep.cve_id = insights.cve_id
			  AND ep.insight_type = insights.insight_type
			  AND ep.deleted_at IS NULL
			  AND (ep.expires_at IS NULL OR ep.expires_at > NOW())
		) THEN insights.status
		WHEN insights.status IN ('resolved', 'dismissed') THEN 'active'
		ELSE insights.status
	END,
	detected_at = CASE
		WHEN insights.status = 'dismissed' AND COALESCE(insights.cluster_id, '') <> '' AND EXISTS (
			SELECT 1 FROM exception_policies ep
			WHERE ep.cluster_id = insights.cluster_id
			  AND ep.resource_uid = insights.resource_uid
			  AND ep.cve_id = insights.cve_id
			  AND ep.insight_type = insights.insight_type
			  AND ep.deleted_at IS NULL
			  AND (ep.expires_at IS NULL OR ep.expires_at > NOW())
		) THEN insights.detected_at
		WHEN insights.status IN ('resolved', 'dismissed') THEN EXCLUDED.detected_at
		ELSE insights.detected_at
	END,
	updated_at = EXCLUDED.updated_at
`, strings.Join(placeholders, ", "))

	if err := tx.Exec(query, values...).Error; err != nil {
		return fmt.Errorf("execute batch upsert: %w", err)
	}

	return nil
}

// Stop stops the insight manager (placeholder for async workers)
func (m *InsightManager) Stop() {
	// No async workers in simplified version
}

// GetInsightsByType returns insights by type
func (m *InsightManager) GetInsightsByType(insightType string) ([]models.Insight, error) {
	var insights []models.Insight
	if err := m.db.Where("insight_type = ?", insightType).Find(&insights).Error; err != nil {
		return nil, fmt.Errorf("failed to get insights: %w", err)
	}
	return insights, nil
}
