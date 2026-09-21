package capability

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"gorm.io/gorm"
)

// InferAttackStepsForIdentity is the cluster-qualified attack-step inference path.
func (asi *AttackStepInference) InferAttackStepsForIdentity(ctx context.Context, id resourceidentity.Identity) error {
	if err := id.Validate(); err != nil {
		return err
	}
	var exploitedCaps []models.PodCapability
	if err := asi.db.WithContext(ctx).
		Where("cluster_id = ? AND pod_uid = ? AND state = ?", id.ClusterID, id.ResourceUID, string(StateExploited)).
		Find(&exploitedCaps).Error; err != nil {
		return err
	}
	for i := range exploitedCaps {
		if err := asi.generateStepsFromCapabilityForIdentity(ctx, id, &exploitedCaps[i]); err != nil {
			log.Printf("[AttackStepInference] failed to generate scoped steps for %s/%s capability %s: %v", id.ClusterID, id.ResourceUID, exploitedCaps[i].CapabilityID, err)
		}
	}
	return nil
}

func (asi *AttackStepInference) generateStepsFromCapabilityForIdentity(ctx context.Context, id resourceidentity.Identity, cap *models.PodCapability) error {
	var metadata models.CapabilityMetadata
	if err := asi.db.WithContext(ctx).Where("capability_id = ?", cap.CapabilityID).First(&metadata).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if len(metadata.ProducesAttackSteps) == 0 {
		return nil
	}

	now := time.Now()
	for _, stepID := range metadata.ProducesAttackSteps {
		var existing models.PodAttackStep
		err := asi.db.WithContext(ctx).
			Where("cluster_id = ? AND pod_uid = ? AND step_id = ?", id.ClusterID, id.ResourceUID, stepID).
			First(&existing).Error
		if err == nil {
			if cap.Confidence > existing.Confidence {
				existing.Confidence = cap.Confidence
				existing.UpdatedAt = now
				asi.updateStepEvidence(ctx, &existing, cap)
				if err := asi.db.WithContext(ctx).Save(&existing).Error; err != nil {
					return err
				}
			}
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}

		step := models.PodAttackStep{
			ClusterID:   id.ClusterID,
			PodUID:      id.ResourceUID,
			StepID:      stepID,
			Category:    getStepCategory(stepID),
			Confidence:  cap.Confidence,
			Description: getStepDescription(stepID),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		evidence, _ := json.Marshal(map[string]interface{}{
			"capability_id":    cap.CapabilityID,
			"capability_state": cap.State,
			"generated_at":     now.Format(time.RFC3339),
		})
		step.Evidence = string(evidence)
		if err := asi.db.WithContext(ctx).Create(&step).Error; err != nil {
			return err
		}
	}
	return nil
}
