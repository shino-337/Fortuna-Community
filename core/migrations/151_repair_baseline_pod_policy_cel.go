package migrations

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

const (
	legacyNoPrivilegedCEL     = "has(object.spec) && has(object.spec.containers) && object.spec.containers.exists(c, has(c.securityContext) && has(c.securityContext.privileged) && c.securityContext.privileged == true)"
	legacyNoHostNamespacesCEL = "has(object.spec) && ((has(object.spec.hostNetwork) && object.spec.hostNetwork == true) || (has(object.spec.hostPID) && object.spec.hostPID == true) || (has(object.spec.hostIPC) && object.spec.hostIPC == true))"
)

// Migration151_RepairBaselinePodPolicyCEL preserves immutable template versions:
// create corrected 1.0.1 system versions, repoint every legacy instance, then
// retire only the exact broken 1.0.0 system templates. User-edited templates are
// left untouched rather than silently reinterpreted.
func Migration151_RepairBaselinePodPolicyCEL(db *gorm.DB) error {
	targets := []struct {
		id, legacy, corrected, baselineInstance string
	}{
		{"k8s-no-privileged-container", legacyNoPrivilegedCEL, baselineNoPrivilegedCEL, "baseline-no-privileged-container"},
		{"k8s-no-host-namespace-sharing", legacyNoHostNamespacesCEL, baselineNoHostNamespacesCEL, "baseline-no-host-namespace-sharing"},
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, target := range targets {
			var old models.PolicyTemplate
			err := tx.Where("template_id = ? AND version = ?", target.id, "1.0.0").First(&old).Error
			legacyFound := err == nil
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("load legacy policy %s: %w", target.id, err)
			}
			if legacyFound && (!old.IsSystem || old.CreatedBy != "system" || old.CELExpression != target.legacy) {
				log.Printf("Migration 151: preserving non-stock policy template %s:1.0.0", target.id)
				continue
			}

			var replacement models.PolicyTemplate
			err = tx.Where("template_id = ? AND version = ?", target.id, baselinePodPolicyVersion).First(&replacement).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if !legacyFound {
					return fmt.Errorf("corrected policy %s:%s is missing", target.id, baselinePodPolicyVersion)
				}
				replacement = old
				replacement.ID = 0
				replacement.CreatedAt = time.Time{}
				replacement.UpdatedAt = time.Time{}
				replacement.DeletedAt = gorm.DeletedAt{}
				replacement.Version = baselinePodPolicyVersion
				replacement.CELExpression = target.corrected
				replacement.CELProgramCache = nil
				if err := tx.Create(&replacement).Error; err != nil {
					return fmt.Errorf("create corrected policy %s: %w", target.id, err)
				}
			} else if err != nil {
				return fmt.Errorf("load corrected policy %s: %w", target.id, err)
			} else if replacement.CELExpression != target.corrected {
				return fmt.Errorf("corrected policy %s:%s has unexpected CEL expression", target.id, baselinePodPolicyVersion)
			}

			if err := tx.Unscoped().Model(&models.PolicyInstance{}).
				Where("template_id = ? AND template_version = ?", target.id, "1.0.0").
				Update("template_version", baselinePodPolicyVersion).Error; err != nil {
				return fmt.Errorf("repoint policy instances %s: %w", target.id, err)
			}
			var baseline models.PolicyInstance
			if err := tx.Where("instance_name = ? AND template_id = ? AND template_version = ?", target.baselineInstance, target.id, baselinePodPolicyVersion).First(&baseline).Error; err == nil {
				if len(baseline.ResourceTypes) == 0 {
					if err := tx.Model(&baseline).Update("resource_types", models.StringArray{"Pod"}).Error; err != nil {
						return fmt.Errorf("scope baseline instance %s: %w", target.baselineInstance, err)
					}
				}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("load baseline instance %s: %w", target.baselineInstance, err)
			}
			if legacyFound {
				if err := tx.Delete(&old).Error; err != nil {
					return fmt.Errorf("retire broken policy %s: %w", target.id, err)
				}
			}
			log.Printf("Migration 151: repaired policy %s and repointed legacy instances", target.id)
		}
		return nil
	})
}
