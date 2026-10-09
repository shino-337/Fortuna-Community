package models

import "time"

// GoModuleAlias maps an old/alias Go module path to the canonical one used in OSV (P2 alias resolver).
// Example: github.com/coreos/etcd -> go.etcd.io/etcd.
type GoModuleAlias struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Alias     string `gorm:"type:varchar(512);not null;uniqueIndex:idx_go_module_alias_alias" json:"alias"`
	Canonical string `gorm:"type:varchar(512);not null;index" json:"canonical"`
}

func (GoModuleAlias) TableName() string {
	return "go_module_alias"
}

// MirrorState stores version/epoch for mirror caches (e.g. OSV). When version increments, cache keys
// that include it automatically miss, so no explicit invalidation is needed (multi-worker safe).
type MirrorState struct {
	Name      string    `gorm:"primaryKey;type:varchar(64)" json:"name"` // e.g. "osv"
	Version   int64     `gorm:"not null;default:0" json:"version"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (MirrorState) TableName() string {
	return "mirror_state"
}
