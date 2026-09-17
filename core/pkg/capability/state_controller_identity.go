package capability

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InitializeCapabilityForIdentity creates/refreshes a capability for one
// canonical Pod identity. It never conflicts with the same Pod UID in another cluster.
func (csc *CapabilityStateController) InitializeCapabilityForIdentity(ctx context.Context, id resourceidentity.Identity, namespace, capabilityID, group, severity string, evidence map[string]interface{}) error {
	if err := id.Validate(); err != nil {
		return err
	}
	var metadata models.CapabilityMetadata
	baseConfidence := 0.5
	mdTx := csc.db.WithContext(ctx).Where("capability_id = ?", capabilityID).Limit(1).Find(&metadata)
	if mdTx.Error == nil && mdTx.RowsAffected > 0 {
		baseConfidence = metadata.ConfidenceBase
		if severity == "" {
			severity = metadata.SeverityBase
		}
	}

	evidenceJSON, _ := json.Marshal(evidence)
	now := time.Now()
	cap := models.PodCapability{
		ClusterID:       id.ClusterID,
		PodUID:          id.ResourceUID,
		Namespace:       namespace,
		CapabilityID:    capabilityID,
		CapabilityGroup: group,
		Severity:        severity,
		State:           string(StateDetected),
		Confidence:      baseConfidence,
		CapabilityClass: "effective",
		DerivedFrom:     `{}`,
		FirstSeenAt:     &now,
		LastSeenAt:      &now,
		Evidence:        string(evidenceJSON),
		MitreTechniques: pq.StringArray{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return csc.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "cluster_id"}, {Name: "pod_uid"}, {Name: "capability_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"capability_group", "severity", "evidence", "last_seen_at", "updated_at"}),
		}).Create(&cap).Error
}

// PromoteCapabilityForIdentity applies runtime promotion rules only to evidence
// belonging to the same cluster-qualified Pod identity.
func (csc *CapabilityStateController) PromoteCapabilityForIdentity(ctx context.Context, id resourceidentity.Identity, capabilityID, signalType string, signalConfidence float64) error {
	if err := id.Validate(); err != nil {
		return err
	}
	promoted := false
	err := csc.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		didPromote, err := csc.promoteCapabilityForIdentityTx(ctx, txDB, id, capabilityID, signalType, signalConfidence)
		if err != nil {
			return err
		}
		promoted = didPromote
		return nil
	})
	if err != nil {
		return err
	}
	if promoted {
		ScheduleAttackPathRebuildForIdentity(csc.db, id)
	}
	return nil
}

func (csc *CapabilityStateController) promoteCapabilityForIdentityTx(ctx context.Context, txDB *gorm.DB, id resourceidentity.Identity, capabilityID, signalType string, signalConfidence float64) (bool, error) {
	type capabilityRow struct {
		ID           uint
		State        string
		Confidence   float64
		Evidence     string
		CapabilityID string
	}
	var cap capabilityRow
	tx := txDB.Model(&models.PodCapability{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "state", "confidence", "evidence", "capability_id").
		Where("cluster_id = ? AND pod_uid = ? AND capability_id = ?", id.ClusterID, id.ResourceUID, capabilityID).
		Limit(1).Find(&cap)
	if tx.Error != nil {
		return false, tx.Error
	}
	if tx.RowsAffected == 0 {
		return false, nil
	}

	var rules []models.PromotionRule
	if err := txDB.Where("capability_id = ? AND signal_type = ?", capabilityID, signalType).
		Order("promote_to DESC").Find(&rules).Error; err != nil {
		return false, err
	}
	if len(rules) == 0 {
		return false, nil
	}

	stateOrder := map[string]int{"detected": 1, "confirmed": 2, "exploited": 3, "chained": 4}
	currentOrder := stateOrder[cap.State]
	if currentOrder == 0 {
		currentOrder = 1
	}

	var signalCount int64
	if err := txDB.Model(&models.RuntimeSignal{}).
		Select("COALESCE(SUM(count), 0)").
		Where("cluster_id = ? AND pod_uid = ? AND signal_type = ?", id.ClusterID, id.ResourceUID, signalType).
		Scan(&signalCount).Error; err != nil {
		return false, err
	}

	var bestRule *models.PromotionRule
	for i := range rules {
		rule := &rules[i]
		targetOrder := stateOrder[rule.PromoteTo]
		if targetOrder <= currentOrder || int(signalCount) < rule.MinOccurrences {
			continue
		}
		if len(rule.RequiredCapabilities) > 0 {
			var requiredCount int64
			if err := txDB.Model(&models.PodCapability{}).
				Where("cluster_id = ? AND pod_uid = ? AND capability_id = ANY(?)", id.ClusterID, id.ResourceUID, pq.Array(rule.RequiredCapabilities)).
				Count(&requiredCount).Error; err != nil {
				return false, err
			}
			if int(requiredCount) < len(rule.RequiredCapabilities) {
				continue
			}
		}
		if bestRule == nil || targetOrder > stateOrder[bestRule.PromoteTo] {
			bestRule = rule
		}
	}
	if bestRule == nil {
		return false, nil
	}

	now := time.Now()
	newConfidence := cap.Confidence + bestRule.ConfidenceBoost
	if signalConfidence > newConfidence {
		newConfidence = signalConfidence
	}
	if newConfidence > 1.0 {
		newConfidence = 1.0
	}
	var evidence map[string]interface{}
	if cap.Evidence != "" {
		_ = json.Unmarshal([]byte(cap.Evidence), &evidence)
	}
	if evidence == nil {
		evidence = map[string]interface{}{}
	}
	evidence["promoted_by"] = signalType
	evidence["promoted_at"] = now.Format(time.RFC3339)
	evidenceJSON, _ := json.Marshal(evidence)
	updates := map[string]interface{}{
		"state":        bestRule.PromoteTo,
		"last_seen_at": now,
		"updated_at":   now,
		"confidence":   newConfidence,
		"evidence":     string(evidenceJSON),
	}
	if err := txDB.Model(&models.PodCapability{}).Where("id = ?", cap.ID).Updates(updates).Error; err != nil {
		return false, err
	}

	log.Printf("[CSC] promoted scoped capability %s for %s/%s: %s -> %s", capabilityID, id.ClusterID, id.ResourceUID, cap.State, bestRule.PromoteTo)
	if bestRule.PromoteTo == string(StateExploited) {
		asi := &AttackStepInference{db: txDB}
		if err := asi.InferAttackStepsForIdentity(ctx, id); err != nil {
			log.Printf("[CSC] scoped attack-step inference failed: %v", err)
		}
	}
	return true, nil
}

func (csc *CapabilityStateController) GetCapabilityStateForIdentity(ctx context.Context, id resourceidentity.Identity, capabilityID string) (string, error) {
	if err := id.Validate(); err != nil {
		return "", err
	}
	var cap models.PodCapability
	tx := csc.db.WithContext(ctx).
		Where("cluster_id = ? AND pod_uid = ? AND capability_id = ?", id.ClusterID, id.ResourceUID, capabilityID).
		Limit(1).Find(&cap)
	if tx.Error != nil {
		return "", tx.Error
	}
	if tx.RowsAffected == 0 {
		return "", gorm.ErrRecordNotFound
	}
	return cap.State, nil
}
