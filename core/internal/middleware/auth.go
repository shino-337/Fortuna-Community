package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/internal/sessions"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

// CtxJWTSessionID holds the server-issued session UUID embedded in the JWT ("sid" claim).
const CtxJWTSessionID = "jwt_session_id"

// AuthMiddleware validates JWT token and attaches server-derived permissions (authoritative).
func AuthMiddleware(db *gorm.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		var tokenString string
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}
		if tokenString == "" && allowAuthQueryToken() && isWebSocketUpgrade(c.Request) {
			tokenString = c.Query("token")
		}
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(tokenString, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		var user models.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		if !user.Active {
			c.JSON(http.StatusForbidden, gin.H{"error": "User is inactive"})
			c.Abort()
			return
		}

		if sessions.TableExists(db) {
			sid := strings.TrimSpace(claims.SessionID)
			if sid == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "session required", "code": "session_required"})
				c.Abort()
				return
			}
			if _, err := sessions.ValidateActiveSession(db, user.ID, sid); err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "session invalidated or expired", "code": "session_invalid"})
				c.Abort()
				return
			}
			sessions.TouchActivity(db, sid, 30*time.Second)
			c.Set(CtxJWTSessionID, sid)
		} else if strings.TrimSpace(claims.SessionID) != "" {
			c.Set(CtxJWTSessionID, strings.TrimSpace(claims.SessionID))
		}

		if user.MustChangePassword && !passwordChangeAllowedPath(c.Request.Method, c.Request.URL.Path) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Password change required before continuing",
				"code":  "password_change_required",
			})
			c.Abort()
			return
		}

		normalized := authorization.NormalizeRole(user.Role)
		perms := authorization.PermissionsForUser(user.Role)

		c.Set("user", &user)
		c.Set("userID", user.ID)
		c.Set("username", user.Username)
		c.Set("role", user.Role)
		c.Set("must_change_password", user.MustChangePassword)
		c.Set(CtxNormalizedRole, normalized)
		c.Set(CtxPermissions, perms)
		c.Set("auth_source", "jwt")
		c.Set("permission_source", "server_role_map")

		c.Next()
	}
}

func passwordChangeAllowedPath(method, path string) bool {
	if method == http.MethodGet && path == "/api/v1/me" {
		return true
	}
	if method == http.MethodPost && path == "/api/v1/change-password" {
		return true
	}
	return false
}

// allowAuthQueryToken controls reading JWT from ?token= for browser WebSocket upgrades.
// It is disabled by default because URL tokens leak through logs, history, and referrers.
func allowAuthQueryToken() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("FORTUNA_ALLOW_AUTH_QUERY_TOKEN")))
	return v == "true" || v == "1"
}

func isWebSocketUpgrade(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// OptionalAuth attaches a principal when a Bearer token is present and valid; never fails.
func OptionalAuth(db *gorm.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]
		claims, err := auth.ValidateToken(tokenString, jwtSecret)
		if err != nil {
			c.Next()
			return
		}

		var user models.User
		if err := db.First(&user, claims.UserID).Error; err != nil || !user.Active {
			c.Next()
			return
		}

		normalized := authorization.NormalizeRole(user.Role)
		perms := authorization.PermissionsForUser(user.Role)

		c.Set("user", &user)
		c.Set("userID", user.ID)
		c.Set("username", user.Username)
		c.Set("role", user.Role)
		c.Set(CtxNormalizedRole, normalized)
		c.Set(CtxPermissions, perms)
		c.Set("auth_source", "jwt")
		c.Set("permission_source", "server_role_map")

		c.Next()
	}
}
