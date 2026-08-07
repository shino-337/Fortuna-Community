package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/internal/sessions"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

// LoginRequest represents login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Username  string `json:"username" binding:"required,min=3,max=50"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=12"`
	Role      string `json:"role"`
	ScopeJSON string `json:"scopeJson"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Token     string       `json:"token"`
	User      *models.User `json:"user"`
	ExpiresAt time.Time    `json:"expiresAt"`
}

// Login handles user login
func Login(db *gorm.DB, jwtSecret string, tokenExpirationHours int) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := ensureUsersDeletedAtColumn(db); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate users schema"})
			return
		}

		// Find user
		var user models.User
		if err := db.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				LogPlatformSecurityAudit(db, c, "login_failed", "auth", "login", map[string]interface{}{
					"username": req.Username,
					"reason":   "user_not_found",
				})
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Verify password
		if !auth.CheckPasswordHash(req.Password, user.Password) {
			LogPlatformSecurityAudit(db, c, "login_failed", "auth", "login", map[string]interface{}{
				"username": req.Username,
				"reason":   "bad_password",
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Inactive accounts return the same client-visible response as other credential failures.
		if !user.Active {
			LogPlatformSecurityAudit(db, c, "login_failed", "auth", "login", map[string]interface{}{
				"username": req.Username,
				"reason":   "inactive_account",
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		normalized := authorization.NormalizeRole(user.Role)
		perms := authorization.ToStrings(authorization.PermissionsForUser(user.Role))
		sid := ""
		if sessions.TableExists(db) {
			var err error
			sid, err = sessions.CreateLoginSession(db, user.ID, tokenExpirationHours, "password", c.ClientIP(), c.Request.UserAgent())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
				return
			}
		}
		token, err := auth.GenerateToken(user.ID, user.Username, normalized, perms, sid, jwtSecret, tokenExpirationHours)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		// Update last login
		user.LastLogin = time.Now()
		db.Save(&user)

		out := user
		out.Password = ""
		out.Permissions = perms
		opScope := authorization.OperationalScopeFromDocument(user.ScopeJSON)

		// Return response
		expiresAt := time.Now().Add(time.Duration(tokenExpirationHours) * time.Hour)
		c.JSON(http.StatusOK, gin.H{
			"token":            token,
			"user":             &out,
			"expiresAt":        expiresAt,
			"operationalScope": opScope,
		})
	}
}

// Register handles user registration (admin only)
func Register(db *gorm.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := ensureUsersDeletedAtColumn(db); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate users schema"})
			return
		}

		if err := auth.ValidatePasswordStrength(req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if username or email already exists. ErrRecordNotFound is expected
		// for a new user; other database errors must fail the request.
		var existingUser models.User
		err := db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error
		switch {
		case err == nil:
			c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
			return
		case errors.Is(err, gorm.ErrRecordNotFound):
			// No matching user; continue with registration.
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check existing user"})
			return
		}

		// Hash password
		hashedPassword, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		role, err := normalizeStoredUserRole(req.Role)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if u, exists := c.Get("user"); exists {
			if actor, ok := u.(*models.User); ok && actor != nil {
				an := authorization.NormalizeRole(actor.Role)
				if an == models.RoleUserAdmin && (role == models.RoleAdmin || role == models.RoleClusterAdmin) {
					c.JSON(http.StatusForbidden, gin.H{"error": "cannot create users with admin or cluster_admin role"})
					return
				}
				if an != models.RoleAdmin && strings.TrimSpace(req.ScopeJSON) != "" && strings.TrimSpace(req.ScopeJSON) != "{}" {
					c.JSON(http.StatusForbidden, gin.H{"error": "only platform administrators may set cluster scope"})
					return
				}
				if an != models.RoleAdmin && an != models.RoleUserAdmin {
					c.JSON(http.StatusForbidden, gin.H{"error": "insufficient privileges to register users"})
					return
				}
			}
		}
		scopeJSON := strings.TrimSpace(req.ScopeJSON)
		if scopeJSON == "" {
			scopeJSON = "{}"
		}
		if !json.Valid([]byte(scopeJSON)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scopeJson must be valid JSON"})
			return
		}

		// Create user
		user := models.User{
			Username:  req.Username,
			Email:     req.Email,
			Password:  hashedPassword,
			Role:      role,
			ScopeJSON: scopeJSON,
			Active:    true,
		}

		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		// Don't return password
		user.Password = ""
		user.Permissions = authorization.ToStrings(authorization.PermissionsForUser(user.Role))

		LogPlatformSecurityAudit(db, c, "user_create", "user", fmt.Sprintf("%d", user.ID), map[string]interface{}{
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		})

		c.JSON(http.StatusCreated, gin.H{"user": user})
	}
}

// GetCurrentUser returns current authenticated user
func GetCurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			return
		}

		userObj := user.(*models.User)
		userObj.Password = "" // Don't expose password
		out := *userObj
		out.Permissions = authorization.ToStrings(authorization.PermissionsForUser(userObj.Role))

		opScope := authorization.OperationalScopeFromDocument(userObj.ScopeJSON)
		c.JSON(http.StatusOK, gin.H{
			"user":             &out,
			"normalized_role":  authorization.NormalizeRole(userObj.Role),
			"operationalScope": opScope,
		})
	}
}

// ChangePassword handles password change
func ChangePassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		type ChangePasswordRequest struct {
			OldPassword string `json:"oldPassword" binding:"required"`
			NewPassword string `json:"newPassword" binding:"required,min=12"`
		}

		var req ChangePasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := auth.ValidatePasswordStrength(req.NewPassword); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			return
		}

		userObj := user.(*models.User)

		// Verify old password
		if !auth.CheckPasswordHash(req.OldPassword, userObj.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid old password"})
			return
		}

		// Hash new password
		hashedPassword, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		// Update password
		now := time.Now()
		userObj.Password = hashedPassword
		userObj.MustChangePassword = false
		userObj.BootstrapCredential = false
		userObj.PasswordChangedAt = &now
		if err := db.Save(userObj).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if sessions.TableExists(db) {
			currentSID := c.GetString(middleware.CtxJWTSessionID)
			if currentSID != "" {
				if err := sessions.RefreshIssueTime(db, userObj.ID, currentSID, now); err != nil {
					log.Printf("[ChangePassword] failed to refresh current session for user %d: %v", userObj.ID, err)
				}
			}
			if err := sessions.RevokeAllForUserExcept(db, userObj.ID, currentSID); err != nil {
				log.Printf("[ChangePassword] failed to revoke other sessions for user %d: %v", userObj.ID, err)
			}
		}

		out := *userObj
		out.Password = ""
		out.Permissions = authorization.ToStrings(authorization.PermissionsForUser(out.Role))
		c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully", "user": &out})
	}
}

// GetUsers returns list of users (admin only). Password is never exposed.
func GetUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable("users") {
			c.JSON(http.StatusOK, gin.H{"users": []map[string]interface{}{}, "total": 0})
			return
		}
		if err := ensureUsersDeletedAtColumn(db); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate users schema"})
			return
		}
		var users []models.User
		if err := db.Where("deleted_at IS NULL").Order("created_at DESC").Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list := make([]map[string]interface{}, 0, len(users))
		for _, u := range users {
			list = append(list, map[string]interface{}{
				"id":                  u.ID,
				"username":            u.Username,
				"email":               u.Email,
				"role":                u.Role,
				"permissions":         authorization.ToStrings(authorization.PermissionsForUser(u.Role)),
				"scopeJson":           u.ScopeJSON,
				"operationalScope":    authorization.OperationalScopeFromDocument(u.ScopeJSON),
				"active":              u.Active,
				"mustChangePassword":  u.MustChangePassword,
				"bootstrapCredential": u.BootstrapCredential,
				"passwordChangedAt":   u.PasswordChangedAt,
				"lastLogin":           u.LastLogin,
				"createdAt":           u.CreatedAt,
				"updatedAt":           u.UpdatedAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{"users": list, "total": len(list)})
	}
}

func normalizeStoredUserRole(role string) (string, error) {
	role = strings.TrimSpace(role)
	if role == "" {
		return models.RoleOperator, nil
	}
	switch {
	case strings.EqualFold(role, models.RoleUser):
		return models.RoleOperator, nil
	case strings.EqualFold(role, models.RoleOperator):
		return models.RoleOperator, nil
	case strings.EqualFold(role, models.RoleViewer):
		return models.RoleViewer, nil
	case strings.EqualFold(role, models.RoleAdmin):
		return models.RoleAdmin, nil
	case strings.EqualFold(role, models.RoleClusterAdmin):
		return models.RoleClusterAdmin, nil
	case strings.EqualFold(role, models.RoleUserAdmin):
		return models.RoleUserAdmin, nil
	default:
		return "", fmt.Errorf("invalid role")
	}
}

func actorMaySetUserRole(actor *models.User, newRole string) bool {
	if actor == nil {
		return true
	}
	an := authorization.NormalizeRole(actor.Role)
	if an == models.RoleAdmin {
		return true
	}
	if an == models.RoleUserAdmin {
		return newRole != models.RoleAdmin && newRole != models.RoleClusterAdmin
	}
	return false
}

func countActiveAdmins(db *gorm.DB) (int64, error) {
	var n int64
	err := db.Model(&models.User{}).
		Where("deleted_at IS NULL AND active = ?", true).
		Where("LOWER(role) = ?", models.RoleAdmin).
		Count(&n).Error
	return n, err
}

// PatchUser updates role, active, and/or scope bindings (RBAC governance). Prevents removing the last active admin.
func PatchUser(db *gorm.DB) gin.HandlerFunc {
	type patchBody struct {
		Role      *string `json:"role"`
		Active    *bool   `json:"active"`
		ScopeJSON *string `json:"scopeJson"`
	}
	return func(c *gin.Context) {
		id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil || id64 == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}
		id := uint(id64)
		actor := c.MustGet("user").(*models.User)
		granted := middleware.GrantedPermissions(c)
		var body patchBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.Role == nil && body.Active == nil && body.ScopeJSON == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no updates: provide role, active, and/or scopeJson"})
			return
		}
		if err := ensureUsersDeletedAtColumn(db); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate users schema"})
			return
		}
		var target models.User
		if err := db.Where("deleted_at IS NULL").First(&target, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if authorization.NormalizeRole(actor.Role) == models.RoleUserAdmin && authorization.NormalizeRole(target.Role) == models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "user administrators cannot modify admin accounts"})
			return
		}
		if authorization.NormalizeRole(actor.Role) == models.RoleUserAdmin && actor.ID == target.ID && body.Role != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "user administrators cannot change their own Fortuna role"})
			return
		}
		before := map[string]any{
			"role":      target.Role,
			"active":    target.Active,
			"scopeJson": target.ScopeJSON,
		}
		prevNorm := authorization.NormalizeRole(target.Role)

		if body.Role != nil {
			if !authorization.HasPermission(granted, authorization.PermissionUsersRoleAssign) {
				c.JSON(http.StatusForbidden, gin.H{"error": "requires users.role.assign"})
				return
			}
			nr, err := normalizeStoredUserRole(*body.Role)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if !actorMaySetUserRole(actor, nr) {
				c.JSON(http.StatusForbidden, gin.H{"error": "cannot assign this role"})
				return
			}
			if prevNorm == models.RoleAdmin && nr != models.RoleAdmin {
				activeAdmins, err := countActiveAdmins(db)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				if target.Active && activeAdmins <= 1 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "cannot demote the last active admin"})
					return
				}
			}
			target.Role = nr
		}
		if body.Active != nil {
			if !authorization.HasPermission(granted, authorization.PermissionUsersUpdate) {
				c.JSON(http.StatusForbidden, gin.H{"error": "requires users.update"})
				return
			}
			if !*body.Active && prevNorm == models.RoleAdmin {
				activeAdmins, err := countActiveAdmins(db)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				if target.Active && activeAdmins <= 1 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "cannot deactivate the last active admin"})
					return
				}
			}
			target.Active = *body.Active
		}
		if body.ScopeJSON != nil {
			if !authorization.HasPermission(granted, authorization.PermissionUsersUpdate) {
				c.JSON(http.StatusForbidden, gin.H{"error": "requires users.update"})
				return
			}
			if authorization.NormalizeRole(actor.Role) != models.RoleAdmin {
				c.JSON(http.StatusForbidden, gin.H{"error": "only platform administrators may modify scope bindings"})
				return
			}
			raw := strings.TrimSpace(*body.ScopeJSON)
			if raw == "" {
				raw = "{}"
			}
			if !json.Valid([]byte(raw)) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "scopeJson must be valid JSON"})
				return
			}
			target.ScopeJSON = raw
		}
		if err := db.Save(&target).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		target.Password = ""
		target.Permissions = authorization.ToStrings(authorization.PermissionsForUser(target.Role))
		LogPlatformSecurityAudit(db, c, "user_update", "user", fmt.Sprintf("%d", target.ID), map[string]interface{}{
			"username": target.Username,
			"role":     target.Role,
			"active":   target.Active,
		})
		after := map[string]any{
			"role":      target.Role,
			"active":    target.Active,
			"scopeJson": target.ScopeJSON,
		}
		tid := target.ID
		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID),
			"user_rbac_change",
			"user",
			fmt.Sprintf("%d", target.ID),
			"success",
			"medium",
			"jwt",
			before,
			after,
			nil,
			&tid,
		)
		securityaudit.Append(db, &ev)
		c.JSON(http.StatusOK, gin.H{"user": target})
	}
}

// DeleteUser soft-deletes a user (requires users.delete). Cannot delete self or the only admin.
func DeleteUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil || id64 == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}
		id := uint(id64)
		actor := c.MustGet("user").(*models.User)
		if actor.ID == id {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete your own account"})
			return
		}
		if err := ensureUsersDeletedAtColumn(db); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate users schema"})
			return
		}
		var target models.User
		if err := db.Where("deleted_at IS NULL").First(&target, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if authorization.NormalizeRole(actor.Role) == models.RoleUserAdmin && authorization.NormalizeRole(target.Role) == models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "user administrators cannot delete admin accounts"})
			return
		}
		if authorization.NormalizeRole(target.Role) == models.RoleAdmin {
			var adminCount int64
			if err := db.Model(&models.User{}).Where("deleted_at IS NULL").Where("LOWER(role) = ?", models.RoleAdmin).Count(&adminCount).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if adminCount <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete the only admin user"})
				return
			}
		}
		if err := db.Delete(&target).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		LogPlatformSecurityAudit(db, c, "user_delete", "user", fmt.Sprintf("%d", id), map[string]interface{}{"username": target.Username})
		c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
	}
}

func ensureUsersDeletedAtColumn(db *gorm.DB) error {
	if !db.Migrator().HasTable("users") {
		return nil
	}
	if db.Migrator().HasColumn(&models.User{}, "DeletedAt") {
		return nil
	}
	dialect := db.Dialector.Name()
	var addSQL string
	switch dialect {
	case "sqlite":
		addSQL = "ALTER TABLE users ADD COLUMN deleted_at datetime"
	default:
		addSQL = "ALTER TABLE users ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE"
	}
	if err := db.Exec(addSQL).Error; err != nil {
		le := strings.ToLower(err.Error())
		if strings.Contains(le, "duplicate") || strings.Contains(le, "already exists") {
			return nil
		}
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at)").Error; err != nil {
		return err
	}
	return nil
}
