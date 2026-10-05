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
			ClusterID:  id.ClusterID,
			PodUID:     id.ResourceUID,
			Namespace:  namespace,
			Name:       name,
			Generation: 1,
			StartedAt:  time.Now(),
			Status:     "active",
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
