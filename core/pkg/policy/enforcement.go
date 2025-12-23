package policy

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// EnforcementResult represents the result of policy enforcement
type EnforcementResult struct {
	Allowed    bool
	Action     string
	Message    string
	Violation  *models.PolicyViolation
	Remediated bool
}

// EnforcementService handles policy enforcement actions
type EnforcementService struct {
	db                 *gorm.DB
	evaluator          *Evaluator
	violationService   *ViolationService
	remediationService *RemediationService
}

// NewEnforcementService creates a new enforcement service
func NewEnforcementService(db *gorm.DB, evaluator *Evaluator) *EnforcementService {
	return &EnforcementService{
		db:                 db,
		evaluator:          evaluator,
		violationService:   NewViolationService(db),
		remediationService: NewRemediationService(db),
	}
}

// EnforcePolicy enforces a policy based on the action type
func (s *EnforcementService) EnforcePolicy(
	ctx context.Context,
	instance *models.PolicyInstance,
	resource map[string]interface{},
	resourceType, resourceUID, resourceName, namespace, clusterID string,
) (*EnforcementResult, error) {
	// Check context first (before any operations)
	select {
	case <-ctx.Done():
		if ctx.Err() == context.Canceled {
			return nil, fmt.Errorf("enforcement cancelled: %w", ctx.Err())
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("enforcement timeout: %w", ctx.Err())
		}
		return nil, ctx.Err()
	default:
	}

	// Check if instance is enabled (early return for disabled instances)
	if !instance.Enabled {
		return &EnforcementResult{
			Allowed: true,
			Action:  "skipped",
			Message: "Policy instance is disabled",
		}, nil
	}

	// Determine action (instance override or template default)
	action := instance.Action
	if action == "" {
		// Get template default action
		var template models.PolicyTemplate
		if err := s.db.Where("template_id = ? AND version = ?", instance.TemplateID, instance.TemplateVersion).
			First(&template).Error; err != nil {
			return nil, fmt.Errorf("failed to get template: %w", err)
		}
		action = template.DefaultAction
		if action == "" {
			action = "alert" // Default fallback
		}
	}

	// Convert resource to Resource struct for evaluator
	resourceObj := &Resource{
		Type:      resourceType,
		UID:       resourceUID,
		Name:      resourceName,
		Namespace: namespace,
		ClusterID: clusterID,
		Spec:      resource,
		Labels:    extractLabels(resource),
	}

	// Evaluate policy using evaluator (fast path) - use provided context
	violations, err := s.evaluator.EvaluateFast(ctx, resourceObj)
	if err != nil {
		// Check if context was cancelled or timed out
		if ctx.Err() == context.Canceled {
			return nil, fmt.Errorf("evaluation cancelled: %w", ctx.Err())
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("evaluation timeout: %w", ctx.Err())
		}
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	// Check if this specific instance violates
	violates := false
	for _, violation := range violations {
		if violation.InstanceID == uint(instance.ID) {
			violates = true
			break
		}
	}

	if !violates {
		return &EnforcementResult{
			Allowed: true,
			Action:  action,
			Message: "Policy check passed",
		}, nil
	}

	// Policy violation detected
	result := &EnforcementResult{
		Allowed: false,
		Action:  action,
		Message: s.getMessage(instance),
	}

	// ✅ VERIFY: Instance still enabled before recording (prevents race condition)
	var currentInstance models.PolicyInstance
	if err := s.db.WithContext(ctx).First(&currentInstance, instance.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to verify instance: %w", err)
	}

	if !currentInstance.Enabled {
		// Instance was disabled during evaluation
		return &EnforcementResult{
			Allowed: true,
			Action:  "skipped",
			Message: "Policy instance was disabled during evaluation",
		}, nil
	}

	// ✅ FIX #3: Check for existing active violation before recording (prevent duplicates)
	existingViolation, err := s.findActiveViolation(ctx, currentInstance.ID, resourceUID, clusterID)
	if err != nil {
		log.Printf("[Enforcement] Failed to check existing violation: %v", err)
	} else if existingViolation != nil {
		// Update existing violation timestamp instead of creating duplicate
		existingViolation.DetectedAt = timePtr(time.Now())
		if err := s.db.WithContext(ctx).Save(existingViolation).Error; err != nil {
			log.Printf("[Enforcement] Failed to update existing violation: %v", err)
		} else {
			result.Violation = existingViolation
			log.Printf("[Enforcement] Updated existing violation %d for resource %s", existingViolation.ID, resourceUID)
		}
	} else {
		// No existing violation - record new one
		violation, err := s.violationService.RecordViolation(ctx, &currentInstance, resourceType, resourceUID, resourceName, namespace, clusterID, action)
		if err != nil {
			// Check if context was cancelled
			if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
				return nil, fmt.Errorf("violation recording cancelled or timeout: %w", ctx.Err())
			}
			log.Printf("[Enforcement] Failed to record violation: %v", err)
		} else {
			result.Violation = violation
		}
	}

	// Apply enforcement action
	switch action {
	case "block":
		result.Allowed = false
		result.Message = fmt.Sprintf("Policy violation: %s", result.Message)
		return result, nil

	case "warn":
		result.Allowed = true
		log.Printf("[Enforcement] WARNING: Policy violation detected but allowing resource: %s", result.Message)
		return result, nil

	case "audit":
		result.Allowed = true
		log.Printf("[Enforcement] AUDIT: Policy violation recorded: %s", result.Message)
		return result, nil

	case "remediate":
		// Use verified instance for remediation
		remediated, err := s.remediationService.RemediateResource(ctx, &currentInstance, resource, resourceType, resourceName, namespace, clusterID)
		if err != nil {
			// Check if context was cancelled
			if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
				return nil, fmt.Errorf("remediation cancelled or timeout: %w", ctx.Err())
			}
			log.Printf("[Enforcement] Remediation failed: %v", err)
			result.Allowed = false
			result.Message = fmt.Sprintf("Remediation failed: %v", err)
			return result, nil
		}
		result.Remediated = remediated
		result.Allowed = remediated
		if remediated {
			result.Message = "Resource remediated successfully"
		} else {
			result.Message = "Remediation attempted but failed"
		}
		return result, nil

	default:
		// Unknown action - default to warn
		result.Allowed = true
		log.Printf("[Enforcement] Unknown action '%s', defaulting to warn", action)
		return result, nil
	}
}

// getMessage gets the violation message
func (s *EnforcementService) getMessage(instance *models.PolicyInstance) string {
	if instance.CustomMessage != "" {
		return instance.CustomMessage
	}

	// Get template message
	var template models.PolicyTemplate
	if err := s.db.Where("template_id = ? AND version = ?", instance.TemplateID, instance.TemplateVersion).
		First(&template).Error; err == nil && template.Description != "" {
		return template.Description
	}

	return fmt.Sprintf("Policy violation: %s", instance.InstanceName)
}

// BlockResource blocks a resource (returns error)
func (s *EnforcementService) BlockResource(message string) error {
	return fmt.Errorf("resource blocked by policy: %s", message)
}

// WarnResource logs a warning but allows the resource
func (s *EnforcementService) WarnResource(message string) {
	log.Printf("[Enforcement] WARNING: %s", message)
}

// AuditResource records an audit log
func (s *EnforcementService) AuditResource(message string) {
	log.Printf("[Enforcement] AUDIT: %s", message)
}

// extractLabels extracts labels from resource
func extractLabels(resource map[string]interface{}) map[string]string {
	labels := make(map[string]string)

	if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
		if labelsMap, ok := metadata["labels"].(map[string]interface{}); ok {
			for k, v := range labelsMap {
				if str, ok := v.(string); ok {
					labels[k] = str
				}
			}
		}
	}

	return labels
}

// findActiveViolation finds an existing active violation for the same resource and instance
func (s *EnforcementService) findActiveViolation(
	ctx context.Context,
	instanceID uint,
	resourceUID string,
	clusterID string,
) (*models.PolicyViolation, error) {
	var violation models.PolicyViolation

	err := s.db.WithContext(ctx).
		Where("instance_id = ? AND resource_uid = ? AND cluster_id = ? AND status = ?",
			instanceID, resourceUID, clusterID, "active").
		First(&violation).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil // No existing violation
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query existing violation: %w", err)
	}

	return &violation, nil
}
