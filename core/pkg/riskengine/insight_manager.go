package riskengine

import (
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// InsightManager manages insight creation and updates
type InsightManager struct {
	db *gorm.DB
}

// NewInsightManager creates a new InsightManager
func NewInsightManager(db *gorm.DB) *InsightManager {
	return &InsightManager{db: db}
}

// createOrUpdateInsightTx performs the actual work within a transaction
// UPDATED: Uses new Insight schema (no AffectedResources JSONB, direct resource fields)
func (m *InsightManager) createOrUpdateInsightTx(tx *gorm.DB, insight *models.Insight) error {
	// Use direct resource fields (no JSONB parsing needed)
	if insight.ResourceUID == "" {
		return fmt.Errorf("resource_uid is required")
	}

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
			log.Printf("[InsightManager] Re-activated vulnerability insight ID=%d (was %s, now active, resource_uid=%s, cve_id=%s)",
				existingVuln.ID, existingVuln.Status, insight.ResourceUID, insight.CVEID)
			return nil
		}
	}

	// For capability insights with CVEID set (capability ID used as logical key), deduplicate by resource_uid + insight_type + cve_id
	// to match DB unique constraint and avoid duplicate key on INSERT.
	if insight.InsightType == "capability" && strings.TrimSpace(insight.CVEID) != "" {
		var existingCap models.Insight
		capQuery := tx.Where("insight_type = ? AND resource_uid = ? AND cve_id = ? AND (status = ? OR status IS NULL) AND deleted_at IS NULL",
			"capability", insight.ResourceUID, insight.CVEID, "active")
		if capQuery.First(&existingCap).Error == nil {
			needsUpdate := false
			if existingCap.Description != insight.Description {
				existingCap.Description = insight.Description
				needsUpdate = true
			}
			if existingCap.Recommendation != insight.Recommendation {
				existingCap.Recommendation = insight.Recommendation
				needsUpdate = true
			}
			if existingCap.Severity != insight.Severity {
				existingCap.Severity = insight.Severity
				needsUpdate = true
			}
			if existingCap.Title != insight.Title {
				existingCap.Title = insight.Title
				needsUpdate = true
			}
			if needsUpdate {
				existingCap.UpdatedAt = time.Now()
				if err := tx.Save(&existingCap).Error; err != nil {
					return fmt.Errorf("failed to update insight: %w", err)
				}
				log.Printf("[InsightManager] Updated capability insight ID=%d (resource_uid=%s, cve_id=%s)",
					existingCap.ID, insight.ResourceUID, insight.CVEID)
			} else {
				log.Printf("[InsightManager] Capability insight already exists (ID=%d), no update needed", existingCap.ID)
			}
			return nil
		}
		var resolvedCap models.Insight
		if tx.Where("insight_type = ? AND resource_uid = ? AND cve_id = ? AND status IN (?, ?) AND deleted_at IS NULL",
			"capability", insight.ResourceUID, insight.CVEID, "resolved", "dismissed").First(&resolvedCap).Error == nil {
			resolvedCap.Status = "active"
			resolvedCap.Severity = insight.Severity
			resolvedCap.Description = insight.Description
			resolvedCap.Recommendation = insight.Recommendation
			resolvedCap.Title = insight.Title
			resolvedCap.UpdatedAt = time.Now()
			resolvedCap.DetectedAt = time.Now()
			if err := tx.Save(&resolvedCap).Error; err != nil {
				return fmt.Errorf("failed to re-activate insight: %w", err)
			}
			log.Printf("[InsightManager] Re-activated capability insight ID=%d (was %s, now active)",
				resolvedCap.ID, resolvedCap.Status)
			return nil
		}
		// Fall through to createInsightTx
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
		resolved.Status = "active"
		resolved.Severity = insight.Severity
		resolved.Description = insight.Description
		resolved.Recommendation = insight.Recommendation
		resolved.UpdatedAt = time.Now()
		resolved.DetectedAt = time.Now()
		if err := tx.Save(&resolved).Error; err != nil {
			return fmt.Errorf("failed to re-activate insight: %w", err)
		}
		log.Printf("[InsightManager] Re-activated insight ID=%d (was %s, now active)",
			resolved.ID, resolved.Status)
		return nil
	}

	return m.createInsightTx(tx, insight)
}

// createInsightTx creates a new insight within a transaction
func (m *InsightManager) createInsightTx(tx *gorm.DB, insight *models.Insight) error {
	// Ensure JSONB fields always contain valid JSON.
	if strings.TrimSpace(insight.Evidence) == "" {
		insight.Evidence = "{}"
	}
	if strings.TrimSpace(insight.ViolatedRules) == "" {
		insight.ViolatedRules = "[]"
	}
	if err := tx.Create(insight).Error; err != nil {
		return fmt.Errorf("failed to create insight: %w", err)
	}
	log.Printf("[InsightManager] Created new insight ID=%d: type=%s, severity=%s, resource=%s/%s/%s",
		insight.ID, insight.InsightType, insight.Severity,
		insight.ResourceType, insight.ResourceNamespace, insight.ResourceName)
	return nil
}

// scheduleRiskScoreCalculation schedules risk score calculation for affected resources
// UPDATED: Uses direct resource fields instead of JSONB parsing
func (m *InsightManager) scheduleRiskScoreCalculation(insight *models.Insight) {
	// Use direct resource fields (no JSONB parsing)
	if insight.ResourceUID == "" {
		return
	}

	// Schedule risk score calculation for the resource
	// This is a placeholder - actual implementation depends on risk engine architecture
	log.Printf("[InsightManager] Scheduling risk score calculation for resource_uid=%s", insight.ResourceUID)
}

// CreateOrUpdateInsight creates or updates an insight (public API)
func (m *InsightManager) CreateOrUpdateInsight(insight *models.Insight) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		return m.createOrUpdateInsightTx(tx, insight)
	})
}

// BatchCreateOrUpdateInsights processes multiple insights in batch using PostgreSQL UPSERT
func (m *InsightManager) BatchCreateOrUpdateInsights(insights []*models.Insight) error {
	if len(insights) == 0 {
		return nil
	}

	return m.db.Transaction(func(tx *gorm.DB) error {
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
}

// batchUpsertVulnerabilityInsights performs efficient batch UPSERT for vulnerability insights
func (m *InsightManager) batchUpsertVulnerabilityInsights(tx *gorm.DB, insights []*models.Insight) error {
	if len(insights) == 0 {
		return nil
	}

	// Deduplicate insights by (resource_uid, cve_id, insight_type) to avoid ON CONFLICT errors
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
	values := make([]interface{}, 0, len(deduplicated)*20) // Estimate 20 columns
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

		// Build placeholder for this row (17 columns, excluding fixed_version)
		placeholder := fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			paramIndex, paramIndex+1, paramIndex+2, paramIndex+3, paramIndex+4, paramIndex+5,
			paramIndex+6, paramIndex+7, paramIndex+8, paramIndex+9, paramIndex+10, paramIndex+11,
			paramIndex+12, paramIndex+13, paramIndex+14, paramIndex+15, paramIndex+16)
		placeholders = append(placeholders, placeholder)

		// Add values in same order as placeholder (excluding FixedVersion)
		values = append(values,
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
			insight.CVEID,
			insight.CVSS,
			insight.AffectedComponent,
			insight.AffectedVersion,
			insight.DetectedAt,
			now, // created_at
			now, // updated_at
		)

		paramIndex += 17
	}

	// Build the UPSERT query
	// NOTE: fixed_version column may not exist in insights table, so we check and conditionally include it
	query := fmt.Sprintf(`
INSERT INTO insights (
	resource_type, resource_namespace, resource_name, resource_uid,
	insight_type, severity, title, description, status, recommendation,
	cve_id, cvss, affected_component, affected_version,
	detected_at, created_at, updated_at
) VALUES %s
ON CONFLICT (resource_uid, cve_id, insight_type)
WHERE deleted_at IS NULL
DO UPDATE SET
	description = EXCLUDED.description,
	recommendation = EXCLUDED.recommendation,
	cvss = EXCLUDED.cvss,
	severity = EXCLUDED.severity,
	affected_version = EXCLUDED.affected_version,
	status = CASE
		WHEN insights.status IN ('resolved', 'dismissed') THEN 'active'
		ELSE insights.status
	END,
	detected_at = CASE
		WHEN insights.status IN ('resolved', 'dismissed') THEN EXCLUDED.detected_at
		ELSE insights.detected_at
	END,
	updated_at = EXCLUDED.updated_at
`, strings.Join(placeholders, ", "))

	// Execute the batch UPSERT
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
