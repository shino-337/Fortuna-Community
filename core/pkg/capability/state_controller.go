package capability

import (
	"context"

	"gorm.io/gorm"
)

// CapabilityStateController (CSC) is the single owner for capability state updates.
// Cluster-qualified methods in state_controller_identity.go are the production path.
type CapabilityStateController struct {
	db *gorm.DB
}

// NewCapabilityStateController creates a new state controller.
func NewCapabilityStateController(db *gorm.DB) *CapabilityStateController {
	return &CapabilityStateController{db: db}
}

// PromoteCapability remains as a fail-closed compatibility wrapper for callers
// that have not yet been converted to pass an explicit cluster-qualified identity.
func (csc *CapabilityStateController) PromoteCapability(ctx context.Context, podUID, capabilityID, signalType string, signalConfidence float64) error {
	id, err := resolveUniquePodIdentity(ctx, csc.db, podUID)
	if err != nil {
		return err
	}
	return csc.PromoteCapabilityForIdentity(ctx, id, capabilityID, signalType, signalConfidence)
}
