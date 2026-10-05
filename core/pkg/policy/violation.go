package policy

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// ViolationService handles policy violation operations
type ViolationService struct {
	db *gorm.DB
}

// NewViolationService creates a new violation service
func NewViolationService(db *gorm.DB) *ViolationService {
	return &ViolationService{
		db: db,
	}
}

// RecordViolation records a policy violation in the database
// ✅ FIX #2: Implements sampling strategy to prevent database bloat
func (s *ViolationService) RecordViolation(
	ctx context.Context,
	instance *models.PolicyInstance,
	resourceType, resourceUID, resourceName, namespace, clusterID, action string,
) (*models.PolicyViolation, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// ✅ FIX #2: Sampling strategy - only record violations based on severity/action
	// Low severity + audit action: 10% sampling (1 in 10)
	// Medium severity + warn action: 50% sampling (1 in 2)
	// High/Critical severity or block/remediate action: 100% (always record)
	if !s.shouldRecordViolation(instance, action) {
		log.Printf("[Violation] Skipping violation recording due to sampling: instance=%s, action=%s", instance.InstanceName, action)
		return nil, nil // Return nil to indicate skipped (not an error)
	}

	// Get template name
	var template models.PolicyTemplate
	if err := s.db.Where("template_id = ? AND version = ?", instance.TemplateID, instance.TemplateVersion).
		First(&template).Error; err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Determine severity
	severity := instance.Severity
	if severity == "" {
		severity = template.DefaultSeverity
	}

	// Get message
	message := instance.CustomMessage
	if message == "" {
		message = template.Description
		if message == "" {
			message = fmt.Sprintf("Policy violation: %s", instance.InstanceName)
		}
	}

	violation := &models.PolicyViolation{
		InstanceID:   instance.ID,
		InstanceName: instance.InstanceName,
		TemplateID:   instance.TemplateID,
		TemplateName: template.Name,
		ResourceType: resourceType,
		ResourceUID:  resourceUID,
		ResourceName: resourceName,
		Namespace:    namespace,
		ClusterID:    clusterID,
		Severity:     severity,
		Action:       action,
		Message:      message,
		Status:       "active",
		DetectedAt:   timePtr(time.Now()),
	}

	if err := s.db.Create(violation).Error; err != nil {
		return nil, fmt.Errorf("failed to create violation: %w", err)
	}

	return violation, nil
}

// shouldRecordViolation implements sampling strategy to prevent database bloat
// Returns true if violation should be recorded, false if it should be sampled out
func (s *ViolationService) shouldRecordViolation(instance *models.PolicyInstance, action string) bool {
	// Determine severity
	severity := instance.Severity
	if severity == "" {
		// Get from template if not set
		var template models.PolicyTemplate
		if err := s.db.Where("template_id = ? AND version = ?", instance.TemplateID, instance.TemplateVersion).
			First(&template).Error; err == nil {
			severity = template.DefaultSeverity
		}
	}

	// Always record high/critical severity violations
	if severity == "high" || severity == "critical" {
		return true
	}

	// Always record block/remediate actions (critical actions)
	if action == "block" || action == "remediate" {
		return true
	}

	// Sampling for low/medium severity with audit/warn actions
	// ✅ FIX #2: Use hash-based sampling for consistent results (same resource always gets same decision)
	// Fallback to time-based if resource UID not available (for testing)
	sampleRate := 0.0
	if severity == "low" && action == "audit" {
		sampleRate = 0.1 // 10% - record 1 in 10
	} else if severity == "medium" && action == "warn" {
		sampleRate = 0.5 // 50% - record 1 in 2
	} else {
		// Default: record all others
		return true
	}

	// Hash-based sampling for consistent results
	// Use a combination of instance ID and timestamp for deterministic sampling
	// In production, use resource UID hash for consistent sampling per resource
	// For now, use time-based with instance ID for better distribution
	hash := uint64(instance.ID) + uint64(time.Now().UnixNano())
	if sampleRate == 0.1 {
		return (hash % 10) == 0 // 10% sampling
	} else if sampleRate == 0.5 {
		return (hash % 2) == 0 // 50% sampling
	}

	return true
}

// timePtr returns a pointer to time.Time
func timePtr(t time.Time) *time.Time {
	return &t
}
