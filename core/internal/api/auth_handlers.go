package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/pkg/models"
)

// LoginRequest represents login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role"`
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
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Check if user is active
		if !user.Active {
			c.JSON(http.StatusForbidden, gin.H{"error": "User account is inactive"})
			return
		}

		// Verify password
		if !auth.CheckPasswordHash(req.Password, user.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Generate token
		token, err := auth.GenerateToken(user.ID, user.Username, user.Role, jwtSecret, tokenExpirationHours)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		// Update last login
		user.LastLogin = time.Now()
		db.Save(&user)

		// Return response
		expiresAt := time.Now().Add(time.Duration(tokenExpirationHours) * time.Hour)
		c.JSON(http.StatusOK, LoginResponse{
			Token:     token,
			User:      &user,
			ExpiresAt: expiresAt,
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

		// Check if username already exists
		var existingUser models.User
		if err := db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
			return
		}

		// Hash password
		hashedPassword, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		// Set default role if not provided
		role := req.Role
		if role == "" {
			role = models.RoleUser
		}

		// Validate role
		if role != models.RoleAdmin && role != models.RoleUser && role != models.RoleViewer {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
			return
		}

		// Create user
		user := models.User{
			Username: req.Username,
			Email:    req.Email,
			Password: hashedPassword,
			Role:     role,
			Active:   true,
		}

		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		// Don't return password
		user.Password = ""

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

		c.JSON(http.StatusOK, gin.H{"user": userObj})
	}
}

// ChangePassword handles password change
func ChangePassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		type ChangePasswordRequest struct {
			OldPassword string `json:"oldPassword" binding:"required"`
			NewPassword string `json:"newPassword" binding:"required,min=8"`
		}

		var req ChangePasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
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
		userObj.Password = hashedPassword
		if err := db.Save(userObj).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
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
				"id":        u.ID,
				"username":  u.Username,
				"email":     u.Email,
				"role":      u.Role,
				"active":    u.Active,
				"lastLogin": u.LastLogin,
				"createdAt": u.CreatedAt,
				"updatedAt": u.UpdatedAt,
			})
		}
		c.JSON(http.StatusOK, gin.H{"users": list, "total": len(list)})
	}
}

func ensureUsersDeletedAtColumn(db *gorm.DB) error {
	if !db.Migrator().HasTable("users") {
		return nil
	}

	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = CURRENT_SCHEMA()
			  AND table_name = 'users'
			  AND column_name = 'deleted_at'
		)
	`).Scan(&exists).Error; err != nil {
		return err
	}
	if exists {
		return nil
	}

	if err := db.Exec("ALTER TABLE users ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE").Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil
		}
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at)").Error; err != nil {
		return err
	}
	return nil
}
