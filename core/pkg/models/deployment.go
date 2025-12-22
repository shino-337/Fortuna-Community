package models

import (
	"time"

	"gorm.io/gorm"
)

// Deployment represents a Kubernetes Deployment
type Deployment struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ClusterID string         `gorm:"not null;index:idx_deployments_cluster_id" json:"clusterId"`
	UID       string         `gorm:"not null;uniqueIndex:idx_deployments_uid" json:"uid"`
	Name      string         `gorm:"not null;index:idx_deployments_name" json:"name"`
	Namespace string         `gorm:"not null;index:idx_deployments_namespace" json:"namespace"`

	// Replica status
	Replicas            int32 `json:"replicas"`
	ReadyReplicas       int32 `json:"readyReplicas"`
	AvailableReplicas   int32 `json:"availableReplicas"`
	UnavailableReplicas int32 `json:"unavailableReplicas"`
	UpdatedReplicas     int32 `json:"updatedReplicas"`

	// Deployment strategy
	Strategy string `json:"strategy"` // RollingUpdate, Recreate

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
	DeletedAt gorm.DeletedAt `gorm:"index:idx_deployments_deleted_at" json:"-"`

	// Relations
	Cluster Cluster `gorm:"foreignKey:ClusterID;constraint:OnDelete:CASCADE" json:"cluster,omitempty"`
}

// TableName specifies the table name
func (Deployment) TableName() string {
	return "deployments"
}

// DeploymentCondition represents a condition in deployment status
type DeploymentCondition struct {
	Type               string    `json:"type"`               // Available, Progressing, ReplicaFailure
	Status             string    `json:"status"`             // True, False, Unknown
	LastUpdateTime     time.Time `json:"lastUpdateTime"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
}

// PodTemplateInfo contains simplified pod template information
type PodTemplateInfo struct {
	Containers []ContainerInfo `json:"containers"`
	Volumes    []VolumeInfo    `json:"volumes,omitempty"`
}

// ContainerInfo contains container information
type ContainerInfo struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Resources ResourceRequirements `json:"resources,omitempty"`
}

// VolumeInfo contains volume information
type VolumeInfo struct {
	Name string `json:"name"`
	Type string `json:"type"` // ConfigMap, Secret, PVC, etc.
}

// ResourceRequirements contains resource limits and requests
type ResourceRequirements struct {
	Limits   map[string]string `json:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty"`
}

