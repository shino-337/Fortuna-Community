package models

import (
	"time"

	"gorm.io/gorm"
)

// ReplicaSet represents a Kubernetes ReplicaSet
type ReplicaSet struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ClusterID string         `gorm:"not null;index:idx_replicasets_cluster_id" json:"clusterId"`
	UID       string         `gorm:"not null;uniqueIndex:idx_replicasets_uid" json:"uid"`
	Name      string         `gorm:"not null;index:idx_replicasets_name" json:"name"`
	Namespace string         `gorm:"not null;index:idx_replicasets_namespace" json:"namespace"`

	// Replica status
	Replicas              int32 `json:"replicas"`
	ReadyReplicas        int32 `json:"readyReplicas"`
	AvailableReplicas    int32 `json:"availableReplicas"`
	FullyLabeledReplicas int32 `json:"fullyLabeledReplicas"`

	// Owner reference
	OwnerKind string `gorm:"index:idx_replicasets_owner" json:"ownerKind,omitempty"`
	OwnerName string `gorm:"index:idx_replicasets_owner" json:"ownerName,omitempty"`
	OwnerUID  string `json:"ownerUid,omitempty"`

	// Metadata (stored as JSON)
	Labels      string `gorm:"type:jsonb;default:'{}'" json:"labels"`
	Annotations string `gorm:"type:jsonb;default:'{}'" json:"annotations"`
	Selector    string `gorm:"type:jsonb;default:'{}'" json:"selector"`

	// Pod template
	Template string `gorm:"type:jsonb;default:'{}'" json:"template"`

	// Status conditions
	Conditions string `gorm:"type:jsonb;default:'[]'" json:"conditions"`

	// Timestamps
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index:idx_replicasets_deleted_at" json:"-"`

	// Relations
	Cluster Cluster `gorm:"foreignKey:ClusterID;constraint:OnDelete:CASCADE" json:"cluster,omitempty"`
}

// TableName specifies the table name
func (ReplicaSet) TableName() string {
	return "replicasets"
}

