package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// validateMigrationResult validates that required tables/columns exist after migration
func validateMigrationResult(db *gorm.DB, migrationNum int, requiredTables []string) error {
	for _, tableName := range requiredTables {
		var exists bool
		query := "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = $1)"
		if err := db.Raw(query, tableName).Scan(&exists).Error; err != nil {
			return fmt.Errorf("failed to check table %s: %w", tableName, err)
		}
		if !exists {
			return fmt.Errorf("migration %d: required table %s was not created", migrationNum, tableName)
		}
		log.Printf("Migration %d: ✅ Verified table %s exists", migrationNum, tableName)
	}
	return nil
}

// validateColumnExists checks if a column exists in a table
func validateColumnExists(db *gorm.DB, tableName, columnName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_schema = CURRENT_SCHEMA()
			AND table_name = $1 
			AND column_name = $2
		)
	`
	if err := db.Raw(query, tableName, columnName).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("failed to check column %s.%s: %w", tableName, columnName, err)
	}
	return exists, nil
}

// validateIndexExists checks if an index exists
func validateIndexExists(db *gorm.DB, indexName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE schemaname = CURRENT_SCHEMA()
			AND indexname = $1
		)
	`
	if err := db.Raw(query, indexName).Scan(&exists).Error; err != nil {
		return false, fmt.Errorf("failed to check index %s: %w", indexName, err)
	}
	return exists, nil
}

