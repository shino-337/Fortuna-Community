package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ksam/core/pkg/models"
	"gorm.io/gorm"
)

// ViolationEvent represents a policy violation event for async processing
// Phase 2.7: Slow path event structure
type ViolationEvent struct {
	Type       string                 `json:"type"`
	Timestamp  int64                  `json:"timestamp"`
	Violations []*Violation           `json:"violations"`
	Resource   map[string]interface{} `json:"resource"`
	Request    map[string]interface{} `json:"request"`
}

// PolicyWorker handles async policy violation processing
// Phase 2.7: Slow path worker for violation storage, insights, alerts
type PolicyWorker struct {
	db            *gorm.DB
	violationSvc  *ViolationService
	enforcementSvc *EnforcementService
}

// NewPolicyWorker creates a new policy worker
func NewPolicyWorker(db *gorm.DB, evaluator *Evaluator) *PolicyWorker {
	return &PolicyWorker{
		db:            db,
		violationSvc:  NewViolationService(db),
		enforcementSvc: NewEnforcementService(db, evaluator),
	}
}

// ProcessViolationEvent processes a violation event from the event bus
// Phase 2.7: Slow path processing - violation storage, insights, alerts
func (w *PolicyWorker) ProcessViolationEvent(ctx context.Context, eventData []byte) error {
	var event ViolationEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal violation event: %w", err)
	}

	log.Printf("[PolicyWorker] Processing violation event: %d violations for %s/%s",
		len(event.Violations), event.Resource["type"], event.Resource["name"])

	// Process each violation
	for _, violation := range event.Violations {
		// Get policy instance from database
		var instance models.PolicyInstance
		if err := w.db.WithContext(ctx).Where("id = ?", violation.InstanceID).First(&instance).Error; err != nil {
			log.Printf("[PolicyWorker] Failed to get instance %d: %v", violation.InstanceID, err)
			continue
		}

		// Record violation in database (slow path - async)
		_, err := w.violationSvc.RecordViolation(
			ctx,
			&instance,
			violation.ResourceType,
			violation.ResourceUID,
			violation.ResourceName,
			violation.Namespace,
			violation.ClusterID,
			violation.Action,
		)
		if err != nil {
			log.Printf("[PolicyWorker] Failed to record violation: %v", err)
			continue
		}

		log.Printf("[PolicyWorker] ✅ Recorded violation: %s/%s", violation.Namespace, violation.ResourceName)
	}

	// TODO: Generate insights (future enhancement)
	// TODO: Send alerts (future enhancement)

	return nil
}

// ProcessRemediationEvent processes a remediation event from the event bus
// Phase 2.7: Slow path processing for remediation
func (w *PolicyWorker) ProcessRemediationEvent(ctx context.Context, eventData []byte) error {
	// TODO: Implement remediation event processing
	log.Printf("[PolicyWorker] Remediation event processing (not yet implemented)")
	return nil
}

