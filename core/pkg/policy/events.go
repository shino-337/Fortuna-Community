package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
)

// PolicyViolationEvent is emitted by admission webhook fast-path
type PolicyViolationEvent struct {
	Type       string                 `json:"type"`
	Timestamp  int64                  `json:"timestamp"`
	Resource   map[string]interface{} `json:"resource"`
	Violations []*Violation           `json:"violations"`
	Request    map[string]interface{} `json:"request"`
}

// PolicyWorker handles async policy violation processing
// Phase 2.7: Slow path worker for violation storage, insights, alerts
type PolicyWorker struct {
	db             *gorm.DB
	violationSvc   *ViolationService
	enforcementSvc *EnforcementService
	js             nats.JetStreamContext
}

// NewPolicyWorker creates a new policy worker
func NewPolicyWorker(db *gorm.DB, js nats.JetStreamContext) *PolicyWorker {
	return &PolicyWorker{
		db:             db,
		violationSvc:   NewViolationService(db),
		enforcementSvc: NewEnforcementService(db),
		js:             js,
	}
}

// ProcessViolationEvent processes a policy violation event from the event bus
func (w *PolicyWorker) ProcessViolationEvent(ctx context.Context, eventData []byte) error {
	var event PolicyViolationEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal violation event: %w", err)
	}

	log.Printf("[PolicyWorker] Processing violation event: %d violations for %v/%v",
		len(event.Violations), event.Resource["kind"], event.Resource["name"])

	insightsToCreate := make([]*models.Insight, 0, len(event.Violations))
	for _, violation := range event.Violations {
		if violation == nil {
			continue
		}

		// Get policy instance
		var instance models.PolicyInstance
		if err := w.db.WithContext(ctx).First(&instance, violation.InstanceID).Error; err != nil {
			log.Printf("[PolicyWorker] Failed to get policy instance %d: %v", violation.InstanceID, err)
			continue
		}

		// Record violation in database (slow path - async)
		pv, err := w.violationSvc.RecordViolation(
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
		if pv == nil {
			continue // sampled out
		}

		detectedAt := time.Now()
		if pv.DetectedAt != nil {
			detectedAt = *pv.DetectedAt
		}

		// Baseline insight output for policy events (D1).
		// Use stable keys (ResourceUID + template/action) so repeated events don't explode in DB.
		title := fmt.Sprintf("Policy violation: %s (%s)", pv.TemplateName, pv.Action)
		insightsToCreate = append(insightsToCreate, &models.Insight{
			ClusterID:         pv.ClusterID,
			ResourceType:      pv.ResourceType,
			ResourceNamespace: pv.Namespace,
			ResourceName:      pv.ResourceName,
			ResourceUID:       pv.ResourceUID,
			InsightType:       "policy_violation",
			Severity:          strings.ToLower(strings.TrimSpace(pv.Severity)),
			Title:             title,
			Description:       pv.Message,
			Recommendation:    fmt.Sprintf("Review policy '%s' and update configuration to remediate the violation.", pv.TemplateName),
			Status:            "active",
			CVEID:             fmt.Sprintf("policy_instance:%d", pv.InstanceID),
			DetectedAt:        detectedAt,
			Evidence:          "{}",
			ViolatedRules:     "[]",
			Remediation:       "{}",
		})

		log.Printf("[PolicyWorker] ✅ Recorded violation: %s/%s", violation.Namespace, violation.ResourceName)
	}

	if len(insightsToCreate) > 0 {
		if err := w.db.WithContext(ctx).Create(insightsToCreate).Error; err != nil {
			return fmt.Errorf("create policy insights: %w", err)
		}
	}

	return nil
}

// ProcessRemediationEvent processes a remediation event from the event bus
// Phase 2.7: Slow path processing for remediation
func (w *PolicyWorker) ProcessRemediationEvent(ctx context.Context, eventData []byte) error {
	// TODO: Implement remediation event processing
	log.Printf("[PolicyWorker] Remediation event processing (not yet implemented)")
	return nil
}
