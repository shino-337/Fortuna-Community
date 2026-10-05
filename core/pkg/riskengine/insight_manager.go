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
	"gorm.io/gorm/clause"

	"github.com/fortuna/core/pkg/evidence"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
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
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("cluster_id = ? AND insight_type = ? AND resource_uid = ? AND cve_id = ? AND (status = ? OR status IS NULL) AND deleted_at IS NULL",
			insight.ClusterID, "vulnerability", insight.ResourceUID, insight.CVEID, "active")

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
		queryResolved := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("cluster_id = ? AND insight_type = ? AND resource_uid = ? AND cve_id = ? AND status IN (?, ?) AND deleted_at IS NULL",
			insight.ClusterID, "vulnerability", insight.ResourceUID, insight.CVEID, "resolved", "dismissed")

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
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("cluster_id = ? AND insight_type = ? AND resource_uid = ? AND cve_id = ? AND deleted_at IS NULL",
			insight.ClusterID, insight.InsightType, insight.ResourceUID, cveKey).First(&existingKey).Error == nil {
			wasResolvedOrDismissed := existingKey.Status == "resolved" || existingKey.Status == "dismissed"
			// RP-5: respect active exception policies — keep dismissed if exempted.
			if existingKey.Status == "dismissed" && isExempted(tx, existingKey.ClusterID, insight.ResourceUID, cveKey, insight.InsightType) {
				log.Printf("[InsightManager] Keeping insight ID=%d dismissed (exception policy active, cluster_id=%s, resource_uid=%s, type=%s, cve_id=%s)",
					existingKey.ID, existingKey.ClusterID, insight.ResourceUID, insight.InsightType, cveKey)
				return nil
			}
			if existingKey.Status != "acknowledged" {
				existingKey.Status = "active"
			}
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
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("cluster_id = ? AND insight_type = ? AND resource_uid = ? AND deleted_at IS NULL AND (cve_id IS NULL OR cve_id = '')",
			insight.ClusterID, insight.InsightType, insight.ResourceUID).First(&existingEmptyCVE).Error == nil {
			wasResolvedOrDismissed := existingEmptyCVE.Status == "resolved" || existingEmptyCVE.Status == "dismissed"
			if existingEmptyCVE.Status == "dismissed" && isExempted(tx, existingEmptyCVE.ClusterID, insight.ResourceUID, "", insight.InsightType) {
				log.Printf("[InsightManager] Keeping insight ID=%d dismissed (exception policy active, cluster_id=%s, resource_uid=%s, type=%s, empty cve_id)",
					existingEmptyCVE.ID, existingEmptyCVE.ClusterID, insight.ResourceUID, insight.InsightType)
				return nil
			}
			if existingEmptyCVE.Status != "acknowledged" {
				existingEmptyCVE.Status = "active"
			}
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
	keyQuery := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
		"cluster_id = ? AND resource_uid = ? AND insight_type = ? AND title = ? AND deleted_at IS NULL",
		insight.ClusterID, insight.ResourceUID, insight.InsightType, insight.Title,
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

// mergeInsightAfterUniqueConflict loads the row matching the DB unique index,
// including soft-deleted rows, and applies the same merge semantics as createOrUpdateInsightTx.
// Used when tx.Create hits a unique violation (race or legacy row shape).
func (m *InsightManager) mergeInsightAfterUniqueConflict(tx *gorm.DB, insight *models.Insight) error {
	var existing models.Insight
	q := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("cluster_id = ? AND resource_uid = ? AND insight_type = ?", insight.ClusterID, insight.ResourceUID, insight.InsightType)
	if insight.CVEID == "" {
		q = q.Where("(cve_id IS NULL OR cve_id = '')")
	} else {
		q = q.Where("cve_id = ?", insight.CVEID)
	}
	if err := q.First(&existing).Error; err != nil {
		return err
	}
	if existing.DeletedAt.Valid {
		if existing.Status == "dismissed" && isExempted(tx, existing.ClusterID, insight.ResourceUID, insight.CVEID, insight.InsightType) {
			return nil
		}
		existing.DeletedAt = gorm.DeletedAt{}
		existing.Status = "active"
		existing.ResolvedAt = nil
		existing.DetectedAt = time.Now().UTC()
		existing.Title = insight.Title
		existing.Description = insight.Description
		existing.Recommendation = insight.Recommendation
		existing.Severity = insight.Severity
		existing.ResourceType = insight.ResourceType
		existing.ResourceNamespace = insight.ResourceNamespace
		existing.ResourceName = insight.ResourceName
		existing.UpdatedAt = time.Now().UTC()
		if err := tx.Unscoped().Save(&existing).Error; err != nil {
			return fmt.Errorf("restore soft-deleted insight after unique conflict: %w", err)
		}
		insight.ID = existing.ID
		return nil
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
	if existing.Status != "acknowledged" {
		existing.Status = "active"
	}
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

// scheduleRiskScoreCalculation only scores cluster-qualified Pods. RBAC and
// other resource insights have no Pod risk score, and a UID alone is ambiguous.
func (m *InsightManager) scheduleRiskScoreCalculation(insight *models.Insight) {
	if insight == nil || !strings.EqualFold(insight.ResourceType, "Pod") {
		return
	}
	id, err := resourceidentity.New(insight.ClusterID, insight.ResourceUID)
	if err != nil {
		return
	}
	go m.runRiskScoreCalculationForIdentity(context.Background(), id)
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
