package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/internal/sessions"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

// ListUserSessions returns sessions for the current user, or (admin + users.read) all sessions with optional ?user_id=.
func ListUserSessions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sessions.TableExists(db) {
			c.JSON(http.StatusOK, gin.H{"sessions": []any{}, "message": "session table not migrated"})
			return
		}
		actor := c.MustGet("user").(*models.User)
		q := db.Model(&models.UserSession{}).Order("issued_at DESC").Limit(200)
		uidParam := strings.TrimSpace(c.Query("user_id"))
		if uidParam != "" {
			if !authorization.HasPermission(middleware.GrantedPermissions(c), authorization.PermissionUsersRead) {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
			id64, err := strconv.ParseUint(uidParam, 10, 32)
			if err != nil || id64 == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
				return
			}
			q = q.Where("user_id = ?", uint(id64))
		} else if authorization.NormalizeRole(actor.Role) != models.RoleAdmin {
			q = q.Where("user_id = ?", actor.ID)
		}
		var rows []models.UserSession
		if err := q.Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]gin.H, 0, len(rows))
		for _, s := range rows {
			out = append(out, gin.H{
				"id":               s.ID,
				"userId":           s.UserID,
				"issuedAt":         s.IssuedAt,
				"expiresAt":        s.ExpiresAt,
				"revokedAt":        s.RevokedAt,
				"lastActivityAt":   s.LastActivityAt,
				"sourceIp":         s.SourceIP,
				"authMethod":       s.AuthMethod,
				"deviceFingerprint": s.DeviceFingerprint,
			})
		}
		c.JSON(http.StatusOK, gin.H{"sessions": out})
	}
}

// RevokeUserSession revokes a single session (own, or admin with users.read for another user).
func RevokeUserSession(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sessions.TableExists(db) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "sessions not available"})
			return
		}
		sid := strings.TrimSpace(c.Param("id"))
		if sid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
			return
		}
		actor := c.MustGet("user").(*models.User)
		var s models.UserSession
		if err := db.Where("id = ?", sid).First(&s).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		if s.UserID != actor.ID {
			if !authorization.HasPermission(middleware.GrantedPermissions(c), authorization.PermissionUsersRead) {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
		if err := sessions.Revoke(db, sid); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID),
			"session_revoke",
			"session",
			sid,
			"success",
			"medium",
			"jwt",
			map[string]any{"revoked": false},
			map[string]any{"revoked": true},
			nil,
			&s.UserID,
		)
		securityaudit.Append(db, &ev)
		c.JSON(http.StatusOK, gin.H{"message": "session revoked"})
	}
}

// RevokeAllUserSessions revokes sessions for self, or (admin) another user via ?user_id=.
func RevokeAllUserSessions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sessions.TableExists(db) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "sessions not available"})
			return
		}
		actor := c.MustGet("user").(*models.User)
		targetUID := actor.ID
		if q := strings.TrimSpace(c.Query("user_id")); q != "" {
			if !authorization.HasPermission(middleware.GrantedPermissions(c), authorization.PermissionSessionsRevokeAll) {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
			id64, err := strconv.ParseUint(q, 10, 32)
			if err != nil || id64 == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
				return
			}
			targetUID = uint(id64)
		} else {
			if !authorization.HasPermission(middleware.GrantedPermissions(c), authorization.PermissionSessionsRevoke) {
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
		if err := sessions.RevokeAllForUser(db, targetUID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		tid := targetUID
		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID),
			"sessions_revoke_all",
			"user",
			strconv.FormatUint(uint64(targetUID), 10),
			"success",
			"high",
			"jwt",
			nil,
			map[string]any{"at": time.Now().UTC()},
			nil,
			&tid,
		)
		securityaudit.Append(db, &ev)
		c.JSON(http.StatusOK, gin.H{"message": "sessions revoked"})
	}
}
