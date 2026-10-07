package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a system user
type User struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Username string `gorm:"uniqueIndex;not null" json:"username"`
	Email    string `gorm:"uniqueIndex;not null" json:"email"`
	Password string `gorm:"not null" json:"-"`        // Don't expose password in JSON
	Role     string `gorm:"default:user" json:"role"` // admin: full platform + user mgmt. cluster_admin: security for its named clusters (above operator). operator | viewer | legacy "user" (→ operator for auth).
	// ScopeJSON is optional RBAC v2 scope: `{"cluster_ids":["1","2"]}`. Empty/absent = unrestricted (migration default).
	ScopeJSON string `gorm:"column:scope_json;type:jsonb;default:'{}'" json:"scopeJson,omitempty"`
	Active    bool   `gorm:"default:true" json:"active"`
	// MustChangePassword is set for bootstrap/default credentials and blocks normal API usage until changed.
	MustChangePassword  bool           `gorm:"column:must_change_password;default:false" json:"mustChangePassword"`
	BootstrapCredential bool           `gorm:"column:bootstrap_credential;default:false" json:"bootstrapCredential,omitempty"`
	PasswordChangedAt   *time.Time     `gorm:"column:password_changed_at" json:"passwordChangedAt,omitempty"`
	LastLogin           time.Time      `json:"lastLogin"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`

	// Permissions is populated on API responses only (not persisted). Authoritative list is resolved server-side from Role.
	Permissions []string `json:"permissions,omitempty" gorm:"-"`

	// Relationships
	AuditLogs []AuditLog `gorm:"foreignKey:UserID" json:"-"`
}

// TableName overrides table name
func (User) TableName() string {
	return "users"
}

// UserRole represents user roles stored in DB (RoleUser is legacy; prefer RoleOperator for new accounts).
const (
	RoleAdmin        = "admin"
	RoleClusterAdmin = "cluster_admin" // security for its named clusters: operator plus exceptions, deletes, ServiceAccount changes
	RoleUserAdmin    = "user_admin"    // retired: has no permissions; migration 154 turns these accounts into viewers with no cluster
	RoleOperator     = "operator"
	RoleUser         = "user" // legacy alias — authorization maps to operator
	RoleViewer       = "viewer"

	// Non-human accounts. They cannot be assigned through the API.
	RoleRiskEvaluator = "risk_evaluator" // the scheduled risk evaluation job: auth.session and risk.evaluate only
	RoleSystem        = "system"         // author of agent-sync audit rows; no permissions, cannot sign in
)
