package models

import (
	"time"

	"gorm.io/gorm"
)

// Cluster represents a Kubernetes cluster (SSOT from agent).
// ID is immutable; Name, Source, K8sVersion, Distribution are mutable (updated on sync).
type Cluster struct {
	ID           string         `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"not null" json:"name"`
	Source       string         `json:"source"`        // "auto" | "env"
	K8sVersion   string         `json:"k8sVersion,omitempty"`
	Distribution string         `json:"distribution,omitempty"` // "eks" | "gke" | "aks" | "kubeadm" | "unknown"
	Region       string         `json:"region"`
	Endpoint     string         `json:"endpoint"`
	Kubeconfig   string         `gorm:"type:text" json:"-"`
	Status       string         `gorm:"default:active" json:"status"`
	LastSync     time.Time      `json:"lastSync"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

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
	PodSecurityContext      string         `gorm:"type:jsonb" json:"podSecurityContext"`
	ContainerSecurityContexts string       `gorm:"type:jsonb" json:"containerSecurityContexts"`
	VolumeMounts            string         `gorm:"type:jsonb" json:"volumeMounts"`
	Volumes                 string         `gorm:"type:jsonb" json:"volumes"`
	Tolerations             string         `gorm:"type:jsonb" json:"tolerations"`
	Affinity                string         `gorm:"type:jsonb" json:"affinity"`
	HostNetwork             bool           `gorm:"default:false;column:host_network" json:"hostNetwork"`
	HostPID                 bool           `gorm:"default:false;column:host_pid" json:"hostPID"`
	HostIPC                 bool           `gorm:"default:false;column:host_ipc" json:"hostIPC"`
	AutomountServiceAccountToken *bool     `gorm:"column:automount_service_account_token" json:"automountServiceAccountToken"`
	NodeName                string         `gorm:"type:varchar(255);index" json:"nodeName"`
	NodeID         *uint          `gorm:"index" json:"nodeId"`            // Reference to nodes table
	Phase                   string         `gorm:"type:varchar(32);default:''" json:"phase"` // Kubernetes pod status: Running, Pending, Succeeded, Failed, Unknown
	PodIP                  string         `gorm:"type:varchar(45);column:pod_ip" json:"podIP,omitempty"`           // Pod IP address (IPv4/IPv6)
	StartTime              NullTime       `gorm:"type:timestamp with time zone;column:start_time" json:"startTime,omitempty"` // When pod started (status.startTime)
	RestartCount           int            `gorm:"default:0;column:restart_count" json:"restartCount"`              // Total container restarts
	OwnerKind              string         `gorm:"type:varchar(64);column:owner_kind" json:"ownerKind,omitempty"`   // Deployment / StatefulSet / Job / CronJob
	OwnerName              string         `gorm:"type:varchar(255);column:owner_name" json:"ownerName,omitempty"`  // Controller name
	ReplicaSetName         string         `gorm:"type:varchar(255);column:replica_set_name" json:"replicaSetName,omitempty"` // If from Deployment
	QoSClass               string         `gorm:"type:varchar(32);column:qos_class" json:"qosClass,omitempty"`     // Guaranteed / Burstable / BestEffort
	SpecHash               string         `gorm:"type:varchar(64);column:spec_hash" json:"specHash,omitempty"`     // SHA256 hex of canonical spec; PCE only when changed
	LastEvaluatedHash      string         `gorm:"type:varchar(64);column:last_evaluated_hash" json:"lastEvaluatedHash,omitempty"` // Set after PCE success; risk reflects this spec
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
