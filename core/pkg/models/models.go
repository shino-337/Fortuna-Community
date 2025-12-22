package models

import (
	"time"

	"gorm.io/gorm"
)

// Cluster represents a Kubernetes cluster
type Cluster struct {
	ID         string         `gorm:"primaryKey" json:"id"`
	Name       string         `gorm:"not null" json:"name"`
	Region     string         `json:"region"`                      // Region field from IMPLEMENTATION_GUIDE
	Endpoint   string         `json:"endpoint"`
	Kubeconfig string         `gorm:"type:text" json:"-"`           // Kubeconfig content (not exposed in JSON for security)
	Status     string         `gorm:"default:active" json:"status"` // active, inactive, error
	LastSync   time.Time      `json:"lastSync"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	ServiceAccounts     []ServiceAccount     `gorm:"foreignKey:ClusterID" json:"serviceAccounts,omitempty"`
	RoleBindings        []RoleBinding        `gorm:"foreignKey:ClusterID" json:"roleBindings,omitempty"`
	ClusterRoleBindings []ClusterRoleBinding `gorm:"foreignKey:ClusterID" json:"clusterRoleBindings,omitempty"`
	Roles               []Role               `gorm:"foreignKey:ClusterID" json:"roles,omitempty"`
	ClusterRoles        []ClusterRole        `gorm:"foreignKey:ClusterID" json:"clusterRoles,omitempty"`
	Deployments         []Deployment         `gorm:"foreignKey:ClusterID" json:"deployments,omitempty"`
	ReplicaSets         []ReplicaSet         `gorm:"foreignKey:ClusterID" json:"replicaSets,omitempty"`
}

// ServiceAccount represents a Kubernetes ServiceAccount
type ServiceAccount struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	ClusterID  string         `gorm:"not null;index" json:"clusterId"`
	Name       string         `gorm:"not null;index" json:"name"`
	Namespace  string         `gorm:"not null;index" json:"namespace"`
	UID        string         `gorm:"not null;index" json:"uid"`
	Labels     string         `gorm:"type:jsonb" json:"labels"`      // JSON string
	Secrets    string         `gorm:"type:jsonb" json:"secrets"`     // JSON array
	LinkedPods string         `gorm:"type:jsonb" json:"linkedPods"`  // JSON array of pod UIDs
	LastUsed   *time.Time     `json:"lastUsed"`                      // Last usage timestamp
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Cluster Cluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
}

// RoleBinding represents a Kubernetes RoleBinding
type RoleBinding struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ClusterID string         `gorm:"not null;index" json:"clusterId"`
	Name      string         `gorm:"not null;index" json:"name"`
	Namespace string         `gorm:"not null;index" json:"namespace"`
	UID       string         `gorm:"not null;index" json:"uid"`
	RoleRef   string         `gorm:"type:jsonb" json:"roleRef"`  // JSON
	Subjects  string         `gorm:"type:jsonb" json:"subjects"` // JSON array
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Cluster Cluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
}

// ClusterRoleBinding represents a Kubernetes ClusterRoleBinding
type ClusterRoleBinding struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ClusterID string         `gorm:"not null;index" json:"clusterId"`
	Name      string         `gorm:"not null;index" json:"name"`
	UID       string         `gorm:"not null;index" json:"uid"`
	RoleRef   string         `gorm:"type:jsonb" json:"roleRef"`  // JSON
	Subjects  string         `gorm:"type:jsonb" json:"subjects"` // JSON array
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Cluster Cluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
}

// Role represents a Kubernetes Role
type Role struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ClusterID string         `gorm:"not null;index" json:"clusterId"`
	Name      string         `gorm:"not null;index" json:"name"`
	Namespace string         `gorm:"not null;index" json:"namespace"`
	UID       string         `gorm:"not null;index" json:"uid"`
	Rules     string         `gorm:"type:jsonb" json:"rules"` // JSON array
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Cluster Cluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
}

// ClusterRole represents a Kubernetes ClusterRole
type ClusterRole struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ClusterID string         `gorm:"not null;index" json:"clusterId"`
	Name      string         `gorm:"not null;index" json:"name"`
	UID       string         `gorm:"not null;index" json:"uid"`
	Rules     string         `gorm:"type:jsonb" json:"rules"` // JSON array
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Cluster Cluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
}

// Pod represents a Kubernetes Pod (to track ServiceAccount usage)
type Pod struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	ClusterID      string         `gorm:"not null;index" json:"clusterId"`
	Name           string         `gorm:"not null;index" json:"name"`
	Namespace      string         `gorm:"not null;index" json:"namespace"`
	ServiceAccount string         `gorm:"not null;index" json:"serviceAccount"`
	UID            string         `gorm:"not null;index" json:"uid"`
	Containers     string         `gorm:"type:jsonb" json:"containers"`     // JSON array of container info
	ImageDigests   string         `gorm:"type:jsonb" json:"imageDigests"` // JSON array of image digests
	NodeID         *uint          `gorm:"index" json:"nodeId"`            // Reference to nodes table
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Cluster Cluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
	Node    *Node   `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ClusterID  string    `gorm:"index" json:"clusterId"`
	UserID     uint      `gorm:"index" json:"userId"`
	Action     string    `gorm:"not null;index" json:"action"`   // create, update, delete
	Resource   string    `gorm:"not null;index" json:"resource"` // serviceaccount, rolebinding, etc.
	ResourceID string    `gorm:"index" json:"resourceId"`
	Details    string    `gorm:"type:jsonb" json:"details"` // JSON
	User       string    `json:"user"`                      // Username for backward compatibility
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"createdAt"`

	Cluster Cluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
	UserObj User    `gorm:"foreignKey:UserID" json:"userObj,omitempty"`
}

// TableName overrides table names
func (Cluster) TableName() string {
	return "clusters"
}

func (ServiceAccount) TableName() string {
	return "service_accounts"
}

func (RoleBinding) TableName() string {
	return "role_bindings"
}

func (ClusterRoleBinding) TableName() string {
	return "cluster_role_bindings"
}

func (Role) TableName() string {
	return "roles"
}

func (ClusterRole) TableName() string {
	return "cluster_roles"
}

func (Pod) TableName() string {
	return "pods"
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
