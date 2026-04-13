package capability

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fortuna/core/pkg/models"
)

// CapabilityStateController (CSC) is the single owner for capability state updates
// All state transitions must go through CSC to ensure consistency
type CapabilityStateController struct {
	db *gorm.DB
}

// NewCapabilityStateController creates a new state controller
func NewCapabilityStateController(db *gorm.DB) *CapabilityStateController {
	return &CapabilityStateController{db: db}
}

// PromoteCapability promotes a capability state based on runtime signal.
// This is the ONLY way to update capability state from runtime signals.
// PCE-4: Wrapped in a serializable transaction with SELECT ... FOR UPDATE
// to prevent data races when multiple signals arrive concurrently.
func (csc *CapabilityStateController) PromoteCapability(ctx context.Context, podUID, capabilityID, signalType string, signalConfidence float64) error {
	return csc.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		return csc.promoteCapabilityTx(ctx, txDB, podUID, capabilityID, signalType, signalConfidence)
	})
}

// promoteCapabilityTx performs the actual promotion logic within a transaction.
func (csc *CapabilityStateController) promoteCapabilityTx(ctx context.Context, txDB *gorm.DB, podUID, capabilityID, signalType string, signalConfidence float64) error {
	// PCE-4: SELECT ... FOR UPDATE — serialize concurrent promotions for the
	// same (pod_uid, capability_id) row.  Any competing goroutine will block
	// on the row lock until this transaction commits or rolls back.
	type capabilityRow struct {
		ID           uint
		State        string
		Confidence   float64
		Evidence     string
		CapabilityID string
	}
	var cap capabilityRow
	tx := txDB.
		Model(&models.PodCapability{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "state", "confidence", "evidence", "capability_id").
		Where("pod_uid = ? AND capability_id = ?", podUID, capabilityID).
		Limit(1).
		Find(&cap)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		log.Printf("[CSC] Capability %s not found for pod %s, skipping promotion", capabilityID, podUID)
		return nil
	}

	// Get promotion rules for this capability + signal
	var rules []models.PromotionRule
	if err := txDB.
		Where("capability_id = ? AND signal_type = ?", capabilityID, signalType).
		Order("promote_to DESC"). // Prefer higher states first
		Find(&rules).Error; err != nil {
		return err
	}

	if len(rules) == 0 {
		// No promotion rule found, skip
		log.Printf("[CSC] No promotion rule found for capability %s + signal %s", capabilityID, signalType)
		return nil
	}

	log.Printf("[CSC] Found %d promotion rules for capability %s + signal %s", len(rules), capabilityID, signalType)

	// Find applicable rule (highest promote_to that matches conditions)
	var bestRule *models.PromotionRule
	var bestStateOrder = map[string]int{
		"detected":  1,
		"confirmed": 2,
		"exploited": 3,
		"chained":   4,
	}

	currentStateOrder := bestStateOrder[cap.State]
	if currentStateOrder == 0 {
		currentStateOrder = 1 // Default to detected
	}

	// Count signal occurrences for this pod + signal type
	var signalCount int64
	// runtime_signals are de-duped by day in SignalAdapter, so we sum the persisted counter.
	txDB.Model(&models.RuntimeSignal{}).
		Select("COALESCE(SUM(count), 0)").
		Where("pod_uid = ? AND signal_type = ?", podUID, signalType).
		Scan(&signalCount)

	log.Printf("[CSC] Signal %s occurrences for pod %s: %d", signalType, podUID, signalCount)

	for i := range rules {
		rule := &rules[i]
		ruleStateOrder := bestStateOrder[rule.PromoteTo]

		// Only promote if new state is higher than current
		if ruleStateOrder <= currentStateOrder {
			log.Printf("[CSC] Rule %s → %s skipped (current state %s is >= target)", 
				rule.SignalType, rule.PromoteTo, cap.State)
			continue
		}

		// Check if we have enough occurrences
		if int(signalCount) < rule.MinOccurrences {
			log.Printf("[CSC] Rule %s → %s skipped (need %d occurrences, have %d)", 
				rule.SignalType, rule.PromoteTo, rule.MinOccurrences, signalCount)
			continue
		}

		// Check required capabilities if any
		if len(rule.RequiredCapabilities) > 0 {
			var requiredCount int64
			txDB.Model(&models.PodCapability{}).
				Where("pod_uid = ? AND capability_id = ANY(?)", podUID, pq.Array(rule.RequiredCapabilities)).
				Count(&requiredCount)
			if int(requiredCount) < len(rule.RequiredCapabilities) {
				log.Printf("[CSC] Rule %s → %s skipped (required capabilities not met: need %d, have %d)", 
					rule.SignalType, rule.PromoteTo, len(rule.RequiredCapabilities), requiredCount)
				continue // Required capabilities not met
			}
		}

		// This rule is applicable
		if bestRule == nil || ruleStateOrder > bestStateOrder[bestRule.PromoteTo] {
			bestRule = rule
			log.Printf("[CSC] Rule %s → %s is applicable (best so far)", rule.SignalType, rule.PromoteTo)
		}
	}

	if bestRule == nil {
		// No applicable rule found
		return nil
	}

	// Promote capability state
	now := time.Now()
	updates := map[string]interface{}{
		"state":        bestRule.PromoteTo,
		"last_seen_at": now,
		"updated_at":   now,
	}

	// Boost confidence
	newConfidence := cap.Confidence + bestRule.ConfidenceBoost
	if newConfidence > 1.0 {
		newConfidence = 1.0
	}
	updates["confidence"] = newConfidence

	// Update evidence with signal info
	var evidence map[string]interface{}
	if cap.Evidence != "" {
		json.Unmarshal([]byte(cap.Evidence), &evidence)
	} else {
		evidence = make(map[string]interface{})
	}
	evidence["promoted_by"] = signalType
	evidence["promoted_at"] = now.Format(time.RFC3339)
	evidenceJSON, _ := json.Marshal(evidence)
	updates["evidence"] = string(evidenceJSON)

	// Update capability
	if err := txDB.
		Model(&models.PodCapability{}).
		Where("id = ?", cap.ID).
		Updates(updates).Error; err != nil {
		return err
	}

	log.Printf("[CSC] ✅ Promoted capability %s for pod %s: %s → %s (signal: %s, confidence: %.2f)",
		capabilityID, podUID, cap.State, bestRule.PromoteTo, signalType, newConfidence)

	// If promoted to "exploited", generate attack steps
	if bestRule.PromoteTo == string(StateExploited) {
		asi := &AttackStepInference{db: txDB}
		if err := asi.InferAttackSteps(ctx, podUID); err != nil {
			log.Printf("[CSC] ⚠️  Failed to generate attack steps after promotion: %v", err)
			// Don't fail the promotion if attack step generation fails
		}
	}

	return nil
}

// InitializeCapability creates a new capability with detected state
// This is called when static config is detected
func (csc *CapabilityStateController) InitializeCapability(ctx context.Context, podUID, namespace, capabilityID, group, severity string, evidence map[string]interface{}) error {
	// Get metadata for base confidence (avoid First() — optional row, no noisy GORM "record not found" log)
	var metadata models.CapabilityMetadata
	baseConfidence := 0.5 // Default
	mdTx := csc.db.WithContext(ctx).
		Where("capability_id = ?", capabilityID).
		Limit(1).
		Find(&metadata)
	if mdTx.Error == nil && mdTx.RowsAffected > 0 {
		baseConfidence = metadata.ConfidenceBase
		if severity == "" {
			severity = metadata.SeverityBase
		}
	}

	evidenceJSON, _ := json.Marshal(evidence)
	now := time.Now()

	cap := models.PodCapability{
		PodUID:          podUID,
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

	// Upsert capability (Phase 3: update last_seen_at on conflict)
	return csc.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "pod_uid"}, {Name: "capability_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"capability_group", "severity", "evidence", "last_seen_at", "updated_at"}),
		}).
		Create(&cap).Error
}

// GetCapabilityState returns current state of a capability
func (csc *CapabilityStateController) GetCapabilityState(ctx context.Context, podUID, capabilityID string) (string, error) {
	var cap models.PodCapability
	tx := csc.db.WithContext(ctx).
		Where("pod_uid = ? AND capability_id = ?", podUID, capabilityID).
		Limit(1).
		Find(&cap)
	if tx.Error != nil {
		return "", tx.Error
	}
	if tx.RowsAffected == 0 {
		return "", gorm.ErrRecordNotFound
	}
	return cap.State, nil
}
