package lifecycle

import (
	"context"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
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


// EnsureActiveInstanceForIdentity ensures one canonical Pod instance exists and is active.
func (m *PodInstanceManager) EnsureActiveInstanceForIdentity(ctx context.Context, id resourceidentity.Identity, namespace, name string) error {
	if err := id.Validate(); err != nil {
		return err
	}
	var instance models.PodInstance
	err := m.db.WithContext(ctx).
		Where("cluster_id = ? AND pod_uid = ?", id.ClusterID, id.ResourceUID).
		First(&instance).Error
	if err == gorm.ErrRecordNotFound {
		instance = models.PodInstance{
			ClusterID: id.ClusterID,
			PodUID: id.ResourceUID,
			Namespace: namespace,
			Name: name,
			Generation: 1,
			StartedAt: time.Now(),
			Status: "active",
		}
		return m.db.WithContext(ctx).Create(&instance).Error
	}
	if err != nil {
		return err
	}
	if instance.Status == "terminated" {
		instance.Status = "active"
		instance.TerminatedAt = nil
		instance.StartedAt = time.Now()
		instance.Generation++
		return m.db.WithContext(ctx).Save(&instance).Error
	}
	if instance.Name != name || instance.Namespace != namespace {
		instance.Name = name
		instance.Namespace = namespace
		return m.db.WithContext(ctx).Save(&instance).Error
	}
	return nil
}

// TerminateInstanceForIdentity terminates only the requested cluster-qualified Pod instance.
func (m *PodInstanceManager) TerminateInstanceForIdentity(ctx context.Context, id resourceidentity.Identity) error {
	if err := id.Validate(); err != nil {
		return err
	}
	now := time.Now()
	return m.db.WithContext(ctx).Model(&models.PodInstance{}).
		Where("cluster_id = ? AND pod_uid = ? AND status = 'active'", id.ClusterID, id.ResourceUID).
		Updates(map[string]interface{}{"status":"terminated","terminated_at":now}).Error
}

// IsActiveForIdentity checks active state without collapsing Pod UID across clusters.
func (m *PodInstanceManager) IsActiveForIdentity(ctx context.Context, id resourceidentity.Identity) (bool, error) {
	if err := id.Validate(); err != nil {
		return false, err
	}
	var count int64
	err := m.db.WithContext(ctx).Model(&models.PodInstance{}).
		Where("cluster_id = ? AND pod_uid = ? AND status = ?", id.ClusterID, id.ResourceUID, "active").
		Count(&count).Error
	return count > 0, err
}
