package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a system user
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"` // Don't expose password in JSON
	Role      string    `gorm:"default:user" json:"role"` // admin, user, viewer
	Active    bool      `gorm:"default:true" json:"active"`
	LastLogin time.Time `json:"lastLogin"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	AuditLogs []AuditLog `gorm:"foreignKey:UserID" json:"-"`
}

// TableName overrides table name
func (User) TableName() string {
	return "users"
}

// UserRole represents user roles
const (
	RoleAdmin  = "admin"
	RoleUser   = "user"
	RoleViewer = "viewer"
)

// IsAdmin checks if user is admin
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// CanEdit checks if user can edit resources
func (u *User) CanEdit() bool {
	return u.Role == RoleAdmin || u.Role == RoleUser
}

// CanView checks if user can view resources
func (u *User) CanView() bool {
	return u.Active && (u.Role == RoleAdmin || u.Role == RoleUser || u.Role == RoleViewer)
}

