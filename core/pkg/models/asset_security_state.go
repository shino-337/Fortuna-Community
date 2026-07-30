package models

import "time"

// AssetSecurityState is the minimal unified runtime security snapshot per asset.
// P0 implementation keeps compatibility by projecting to object.fortuna.*.
type AssetSecurityState struct {
	ID uint `gorm:"primaryKey" json:"id"`

	AssetType string `gorm:"type:varchar(32);not null;index" json:"assetType"` // currently "pod"
	PodUID    string `gorm:"type:varchar(255);not null;uniqueIndex" json:"podUid"`

	Namespace string `gorm:"type:varchar(255);not null;index" json:"namespace"`
	ClusterID string `gorm:"type:varchar(255);not null;index" json:"clusterId"`

	// Identity / privilege snapshot (minimal)
	HostNetwork bool `gorm:"default:false" json:"hostNetwork"`
	HostPID     bool `gorm:"column:host_pid;default:false" json:"hostPID"`
	HostIPC     bool `gorm:"column:host_ipc;default:false" json:"hostIPC"`

	ServiceAccountBoundToClusterAdmin bool `gorm:"default:false" json:"serviceAccountBoundToClusterAdmin"`

	// Runtime projection (minimal)
	SignalTotal24h         int64      `gorm:"column:signal_total_24h;default:0" json:"signalTotal24h"`
	HasSuspiciousExec      bool       `gorm:"default:false" json:"hasSuspiciousExec"`
	HasNetworkQueueAnomaly bool       `gorm:"default:false" json:"hasNetworkQueueAnomaly"`
	HasEscapeRelated       bool       `gorm:"default:false" json:"hasEscapeRelated"`
	LastRuntimeActivityAt  *time.Time `json:"lastRuntimeActivityAt,omitempty"`

	// Future extension (Layer 4 backbone)
	RuntimeSignalsByType  string `gorm:"type:jsonb;not null;default:'{}'" json:"runtimeSignalsByType"`
	EffectiveCapabilities string `gorm:"type:jsonb;not null;default:'[]'" json:"effectiveCapabilities"`

	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func (AssetSecurityState) TableName() string {
	return "asset_security_state"
}
