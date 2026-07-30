package capability

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// AttackStepInference generates attack steps from exploited capabilities
type AttackStepInference struct {
	db *gorm.DB
}

// NewAttackStepInference creates a new attack step inference engine
func NewAttackStepInference(db *gorm.DB) *AttackStepInference {
	return &AttackStepInference{db: db}
}

// InferAttackSteps generates attack steps for a pod based on exploited capabilities
// Only generates steps for capabilities in "exploited" state
func (asi *AttackStepInference) InferAttackSteps(ctx context.Context, podUID string) error {
	// Get all exploited capabilities for this pod
	var exploitedCaps []models.PodCapability
	if err := asi.db.WithContext(ctx).
		Where("pod_uid = ? AND state = ?", podUID, string(StateExploited)).
		Find(&exploitedCaps).Error; err != nil {
		return err
	}

	if len(exploitedCaps) == 0 {
		return nil // No exploited capabilities, no attack steps
	}

	log.Printf("[AttackStepInference] Found %d exploited capabilities for pod %s", len(exploitedCaps), podUID)

	// For each exploited capability, get metadata and generate attack steps
	for _, cap := range exploitedCaps {
		if err := asi.generateStepsFromCapability(ctx, podUID, &cap); err != nil {
			log.Printf("[AttackStepInference] Failed to generate steps for capability %s: %v", cap.CapabilityID, err)
			continue
		}
	}

	return nil
}

// generateStepsFromCapability generates attack steps from a single exploited capability
func (asi *AttackStepInference) generateStepsFromCapability(ctx context.Context, podUID string, cap *models.PodCapability) error {
	// Get capability metadata
	var metadata models.CapabilityMetadata
	if err := asi.db.WithContext(ctx).
		Where("capability_id = ?", cap.CapabilityID).
		First(&metadata).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// No metadata, skip
			return nil
		}
		return err
	}

	// Check if capability produces attack steps
	if len(metadata.ProducesAttackSteps) == 0 {
		return nil // No attack steps defined
	}

	// Generate attack step for each step ID
	now := time.Now()
	for _, stepID := range metadata.ProducesAttackSteps {
		// Check if step already exists
		var existing models.PodAttackStep
		err := asi.db.WithContext(ctx).
			Where("pod_uid = ? AND step_id = ?", podUID, stepID).
			First(&existing).Error

		if err == nil {
			// Step already exists, update confidence if higher
			if cap.Confidence > existing.Confidence {
				existing.Confidence = cap.Confidence
				existing.UpdatedAt = now
				asi.updateStepEvidence(ctx, &existing, cap)
				if err := asi.db.WithContext(ctx).Save(&existing).Error; err != nil {
					log.Printf("[AttackStepInference] Failed to update step %s: %v", stepID, err)
				}
			}
			continue
		} else if err != gorm.ErrRecordNotFound {
			return err
		}

		// Create new attack step
		step := models.PodAttackStep{
			PodUID:      podUID,
			StepID:      stepID,
			Category:    getStepCategory(stepID),
			Confidence:  cap.Confidence,
			Description: getStepDescription(stepID),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		// Set evidence
		evidence := map[string]interface{}{
			"capability_id": cap.CapabilityID,
			"capability_state": cap.State,
			"generated_at": now.Format(time.RFC3339),
		}
		evidenceJSON, _ := json.Marshal(evidence)
		step.Evidence = string(evidenceJSON)

		// Create step
		if err := asi.db.WithContext(ctx).Create(&step).Error; err != nil {
			log.Printf("[AttackStepInference] Failed to create step %s: %v", stepID, err)
			continue
		}

		log.Printf("[AttackStepInference] ✅ Generated attack step %s for pod %s (from capability %s)",
			stepID, podUID, cap.CapabilityID)
	}

	return nil
}

// updateStepEvidence updates evidence for existing step
func (asi *AttackStepInference) updateStepEvidence(ctx context.Context, step *models.PodAttackStep, cap *models.PodCapability) {
	var evidence map[string]interface{}
	if step.Evidence != "" {
		json.Unmarshal([]byte(step.Evidence), &evidence)
	} else {
		evidence = make(map[string]interface{})
	}

	// Add capability info
	evidence["capability_id"] = cap.CapabilityID
	evidence["capability_state"] = cap.State
	evidence["updated_at"] = time.Now().Format(time.RFC3339)

	evidenceJSON, _ := json.Marshal(evidence)
	step.Evidence = string(evidenceJSON)
}

// getStepCategory returns category for attack step ID
func getStepCategory(stepID string) string {
	categories := map[string]string{
		"NODE_FS_WRITE":        "FILESYSTEM",
		"NODE_KERNEL_ACCESS":   "KERNEL",
		"PROC_NAMESPACE_ACCESS": "PROCESS",
		"IPC_NAMESPACE_ACCESS":  "IPC",
		"NODE_CRED_DUMP":       "CREDENTIALS",
		"NODE_PERSISTENCE":     "PERSISTENCE",
		"KUBELET_CRED_ACCESS":  "CREDENTIALS",
		"PROC_ROOT_PIVOT":      "ESCAPE",
		"RBAC_ABUSE":           "RBAC",
		"NETWORK_SNIFFING":     "NETWORK",
		"RBAC_ESCALATION":      "RBAC",
		"RESOURCE_MANIPULATION": "RBAC",
		"CONTROL_PLANE_ACCESS":  "CONTROL_PLANE",
	}

	if cat, ok := categories[stepID]; ok {
		return cat
	}
	return "UNKNOWN"
}

// getStepDescription returns description for attack step ID
func getStepDescription(stepID string) string {
	descriptions := map[string]string{
		"NODE_FS_WRITE":        "Write access to node filesystem",
		"NODE_KERNEL_ACCESS":   "Access to kernel resources",
		"PROC_NAMESPACE_ACCESS": "Access to process namespace",
		"IPC_NAMESPACE_ACCESS":  "Access to IPC namespace",
		"NODE_CRED_DUMP":       "Dump credentials from node",
		"NODE_PERSISTENCE":     "Establish persistence on node",
		"KUBELET_CRED_ACCESS":  "Access kubelet credentials",
		"PROC_ROOT_PIVOT":      "Proc root pivot (container escape)",
		"RBAC_ABUSE":           "Abuse RBAC permissions",
		"NETWORK_SNIFFING":     "Sniff network traffic",
		"RBAC_ESCALATION":      "Escalate RBAC privileges",
		"RESOURCE_MANIPULATION": "Manipulate Kubernetes resources",
		"CONTROL_PLANE_ACCESS":  "Access control plane components",
	}

	if desc, ok := descriptions[stepID]; ok {
		return desc
	}
	return "Attack step: " + stepID
}
