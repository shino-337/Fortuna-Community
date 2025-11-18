package migrations

import (
	"fmt"
	"log"
	"os"

	"gorm.io/gorm"

	"github.com/ksam/core/internal/auth"
	"github.com/ksam/core/pkg/models"
)

// RunMigrations runs all database migrations
func RunMigrations(db *gorm.DB) error {
	log.Println("Running database migrations...")

	// Run migrations in order
	migrations := []func(*gorm.DB) error{
		Migration001_InitialSchema,
		Migration002_AddUsers,
		Migration003_AddUserToAuditLogs,
	}

	for i, migration := range migrations {
		if err := migration(db); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	log.Println("All migrations completed successfully")
	return nil
}

// Migration001_InitialSchema creates initial tables
func Migration001_InitialSchema(db *gorm.DB) error {
	log.Println("Running migration 001: Initial schema")

	// Focus on ServiceAccount only for now
	return db.AutoMigrate(
		&models.Cluster{},
		&models.ServiceAccount{},
		&models.AuditLog{},
	)
}

// Migration002_AddUsers creates users table
func Migration002_AddUsers(db *gorm.DB) error {
	log.Println("Running migration 002: Add users table")

	return db.AutoMigrate(&models.User{})
}

// Migration003_AddUserToAuditLogs adds user_id to audit_logs
func Migration003_AddUserToAuditLogs(db *gorm.DB) error {
	log.Println("Running migration 003: Add user_id to audit_logs")

	// Check if column already exists
	if db.Migrator().HasColumn(&models.AuditLog{}, "user_id") {
		log.Println("Column user_id already exists, skipping")
		return nil
	}

	// Add user_id column
	if err := db.Migrator().AddColumn(&models.AuditLog{}, "user_id"); err != nil {
		return err
	}

	// Add foreign key constraint
	if err := db.Migrator().CreateConstraint(&models.AuditLog{}, "UserID"); err != nil {
		// Constraint might already exist, ignore error
		log.Printf("Warning: Could not create constraint: %v", err)
	}

	return nil
}

// CreateDefaultAdmin creates a default admin user if it doesn't exist
func CreateDefaultAdmin(db *gorm.DB, username, password, email string) error {
	var user models.User
	result := db.Where("username = ?", username).First(&user)

	if result.Error == gorm.ErrRecordNotFound {
		// User doesn't exist, create it
		hashedPassword, err := auth.HashPassword(password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		user = models.User{
			Username: username,
			Email:    email,
			Password: hashedPassword,
			Role:     models.RoleAdmin,
			Active:   true,
		}

		if err := db.Create(&user).Error; err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}

		log.Printf("Created default admin user: %s", username)
	} else if result.Error != nil {
		return fmt.Errorf("failed to check for admin user: %w", result.Error)
	} else {
		log.Printf("Admin user already exists: %s", username)
	}

	return nil
}

// RunPostMigrations runs migrations that should run after schema migrations
func RunPostMigrations(db *gorm.DB) error {
	// Create default admin user if environment variables are set
	adminUsername := os.Getenv("KSAM_ADMIN_USERNAME")
	adminPassword := os.Getenv("KSAM_ADMIN_PASSWORD")
	adminEmail := os.Getenv("KSAM_ADMIN_EMAIL")

	if adminUsername != "" && adminPassword != "" {
		if adminEmail == "" {
			adminEmail = adminUsername + "@ksam.local"
		}
		if err := CreateDefaultAdmin(db, adminUsername, adminPassword, adminEmail); err != nil {
			return fmt.Errorf("failed to create default admin: %w", err)
		}
	}

	return nil
}
