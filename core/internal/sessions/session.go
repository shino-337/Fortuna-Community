package sessions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

var (
	// ErrSessionNotFound is returned when the session row is missing.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionRevoked indicates the session was revoked server-side.
	ErrSessionRevoked = errors.New("session revoked")
	// ErrSessionExpired indicates JWT/session expiry.
	ErrSessionExpired = errors.New("session expired")
	// ErrSessionStale indicates the session was issued before the user's latest password change.
	ErrSessionStale = errors.New("session issued before password change")
)

// TableExists reports whether user_sessions has been migrated.
func TableExists(db *gorm.DB) bool {
	return db != nil && db.Migrator().HasTable(&models.UserSession{})
}

// CreateLoginSession inserts a new interactive session row.
func CreateLoginSession(db *gorm.DB, userID uint, hours int, authMethod, ip, ua string) (string, error) {
	if db == nil {
		return "", errors.New("nil db")
	}
	id := uuid.NewString()
	now := time.Now()
	sum := sha256.Sum256([]byte(ua + "|" + ip))
	fp := hex.EncodeToString(sum[:8])
	s := models.UserSession{
		ID:                id,
		UserID:            userID,
		IssuedAt:          now,
		ExpiresAt:         now.Add(time.Duration(hours) * time.Hour),
		LastActivityAt:    now,
		SourceIP:          ip,
		UserAgent:         ua,
		DeviceFingerprint: fp,
		AuthMethod:        authMethod,
	}
	if err := db.Create(&s).Error; err != nil {
		return "", err
	}
	return id, nil
}

// ValidateActiveSession returns the session if it is usable for the given user.
func ValidateActiveSession(db *gorm.DB, userID uint, sessionID string) (*models.UserSession, error) {
	if db == nil || sessionID == "" {
		return nil, ErrSessionNotFound
	}
	var s models.UserSession
	if err := db.Where("id = ? AND user_id = ?", sessionID, userID).First(&s).Error; err != nil {
		return nil, ErrSessionNotFound
	}
	now := time.Now()
	if s.RevokedAt != nil {
		return nil, ErrSessionRevoked
	}
	if !s.ExpiresAt.After(now) {
		return nil, ErrSessionExpired
	}
	var user models.User
	if err := db.Select("id", "password_changed_at").First(&user, userID).Error; err == nil &&
		user.PasswordChangedAt != nil &&
		s.IssuedAt.Before(*user.PasswordChangedAt) {
		return nil, ErrSessionStale
	}
	return &s, nil
}

// TouchActivity updates last_activity_at at most once per minGap (best-effort).
func TouchActivity(db *gorm.DB, sessionID string, minGap time.Duration) {
	if db == nil || sessionID == "" {
		return
	}
	cutoff := time.Now().Add(-minGap)
	_ = db.Model(&models.UserSession{}).
		Where("id = ? AND last_activity_at < ?", sessionID, cutoff).
		Update("last_activity_at", time.Now()).Error
}

// Revoke marks a session revoked (idempotent).
func Revoke(db *gorm.DB, sessionID string) error {
	if db == nil {
		return nil
	}
	now := time.Now()
	return db.Model(&models.UserSession{}).Where("id = ?", sessionID).Update("revoked_at", now).Error
}

// RevokeAllForUser revokes every active session for a user.
func RevokeAllForUser(db *gorm.DB, userID uint) error {
	if db == nil {
		return nil
	}
	now := time.Now()
	return db.Model(&models.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

// RevokeAllForUserExcept revokes every active session for a user except keepSessionID.
func RevokeAllForUserExcept(db *gorm.DB, userID uint, keepSessionID string) error {
	if db == nil {
		return nil
	}
	now := time.Now()
	q := db.Model(&models.UserSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID)
	if keepSessionID != "" {
		q = q.Where("id <> ?", keepSessionID)
	}
	return q.Update("revoked_at", now).Error
}

// RefreshIssueTime moves a kept session to the supplied issue time so password-change
// invalidation can keep the current browser session while revoking older sessions.
func RefreshIssueTime(db *gorm.DB, userID uint, sessionID string, issuedAt time.Time) error {
	if db == nil || sessionID == "" {
		return nil
	}
	return db.Model(&models.UserSession{}).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Updates(map[string]interface{}{
			"issued_at":        issuedAt,
			"last_activity_at": issuedAt,
		}).Error
}
