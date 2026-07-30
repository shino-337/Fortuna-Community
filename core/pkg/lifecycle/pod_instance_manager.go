package lifecycle

import (
	"context"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// PodInstanceManager manages pod instance lifecycle
type PodInstanceManager struct {
	db *gorm.DB
}

// NewPodInstanceManager creates a new pod instance manager
func NewPodInstanceManager(db *gorm.DB) *PodInstanceManager {
	return &PodInstanceManager{db: db}
}

// EnsureActiveInstance ensures a pod instance exists and is active
func (m *PodInstanceManager) EnsureActiveInstance(ctx context.Context, podUID, namespace, name string) error {
	var instance models.PodInstance
	err := m.db.WithContext(ctx).Where("pod_uid = ?", podUID).First(&instance).Error

	if err == gorm.ErrRecordNotFound {
		// Create new instance
		instance = models.PodInstance{
			PodUID:     podUID,
			Namespace:  namespace,
			Name:       name,
			Generation: 1,
			StartedAt:  time.Now(),
			Status:     "active",
		}
		return m.db.WithContext(ctx).Create(&instance).Error
	} else if err != nil {
		return err
	}

	// If instance exists but is terminated, reactivate it
	if instance.Status == "terminated" {
		instance.Status = "active"
		instance.TerminatedAt = nil
		instance.StartedAt = time.Now()
		instance.Generation++
		return m.db.WithContext(ctx).Save(&instance).Error
	}

	// Instance is already active, update name/namespace if changed
	if instance.Name != name || instance.Namespace != namespace {
		instance.Name = name
		instance.Namespace = namespace
		return m.db.WithContext(ctx).Save(&instance).Error
	}

	return nil
}

// TerminateInstance marks a pod instance as terminated
func (m *PodInstanceManager) TerminateInstance(ctx context.Context, podUID string) error {
	now := time.Now()
	return m.db.WithContext(ctx).Model(&models.PodInstance{}).
		Where("pod_uid = ? AND status = 'active'", podUID).
		Updates(map[string]interface{}{
			"status":       "terminated",
			"terminated_at": now,
		}).Error
}

// GetActiveInstances returns all active pod instances
func (m *PodInstanceManager) GetActiveInstances(ctx context.Context) ([]models.PodInstance, error) {
	var instances []models.PodInstance
	err := m.db.WithContext(ctx).
		Where("status = ?", "active").
		Find(&instances).Error
	return instances, err
}

// IsActive checks if a pod instance is active
func (m *PodInstanceManager) IsActive(ctx context.Context, podUID string) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Model(&models.PodInstance{}).
		Where("pod_uid = ? AND status = ?", podUID, "active").
		Count(&count).Error
	return count > 0, err
}
