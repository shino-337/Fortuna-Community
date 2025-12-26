package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration034_StandardizeCVSSType standardizes CVSS column types across tables
func Migration034_StandardizeCVSSType(db *gorm.DB) error {
	log.Println("[Migration 034] Starting: Standardize CVSS type")

	// Standardize to REAL (float32) for consistency
	// PostgreSQL REAL = 4 bytes, suitable for CVSS scores (0.0-10.0)

	// 1. Check and update cve_matches.cvss_score
	log.Println("[Migration 034] Checking cve_matches.cvss_score type...")
	var currentType string
	if err := db.Raw(`
		SELECT data_type 
		FROM information_schema.columns 
		WHERE table_name = 'cve_matches' 
		AND column_name = 'cvss_score'
	`).Scan(&currentType).Error; err != nil {
		log.Printf("[Migration 034] ⚠️  Error checking cve_matches.cvss_score type: %v", err)
	} else {
		if currentType != "real" {
			log.Printf("[Migration 034] Converting cve_matches.cvss_score from %s to real...", currentType)
			if err := db.Exec(`
				ALTER TABLE cve_matches 
				ALTER COLUMN cvss_score TYPE real USING cvss_score::real;
			`).Error; err != nil {
				log.Printf("[Migration 034] ⚠️  Error converting cve_matches.cvss_score: %v", err)
			} else {
				log.Println("[Migration 034] ✅ Converted cve_matches.cvss_score to real")
			}
		} else {
			log.Println("[Migration 034] ✅ cve_matches.cvss_score already has type real")
		}
	}

	// 2. Check and update insights.cvss (new column)
	log.Println("[Migration 034] Checking insights.cvss type...")
	var hasCVSSColumn bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'insights' 
			AND column_name = 'cvss'
		)
	`).Scan(&hasCVSSColumn).Error; err != nil {
		log.Printf("[Migration 034] ⚠️  Error checking insights.cvss column: %v", err)
	}

	if hasCVSSColumn {
		var cvssType string
		if err := db.Raw(`
			SELECT data_type 
			FROM information_schema.columns 
			WHERE table_name = 'insights' 
			AND column_name = 'cvss'
		`).Scan(&cvssType).Error; err != nil {
			log.Printf("[Migration 034] ⚠️  Error checking insights.cvss type: %v", err)
		} else {
			if cvssType != "real" {
				log.Printf("[Migration 034] Converting insights.cvss from %s to real...", cvssType)
				if err := db.Exec(`
					ALTER TABLE insights 
					ALTER COLUMN cvss TYPE real USING cvss::real;
				`).Error; err != nil {
					log.Printf("[Migration 034] ⚠️  Error converting insights.cvss: %v", err)
				} else {
					log.Println("[Migration 034] ✅ Converted insights.cvss to real")
				}
			} else {
				log.Println("[Migration 034] ✅ insights.cvss already has type real")
			}
		}
	} else {
		log.Println("[Migration 034] ℹ️  insights.cvss column does not exist (may have been dropped)")
	}

	// 3. Check and update insights.cvss_score (old column, may be dropped)
	log.Println("[Migration 034] Checking insights.cvss_score (old column)...")
	var hasCVSSScoreColumn bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'insights' 
			AND column_name = 'cvss_score'
		)
	`).Scan(&hasCVSSScoreColumn).Error; err != nil {
		log.Printf("[Migration 034] ⚠️  Error checking insights.cvss_score column: %v", err)
	}

	if hasCVSSScoreColumn {
		log.Println("[Migration 034] ⚠️  insights.cvss_score (old column) still exists - should be dropped in Migration 031")
		// Don't convert, just log - this should have been dropped
	} else {
		log.Println("[Migration 034] ✅ insights.cvss_score (old column) does not exist (correctly dropped)")
	}

	// 4. Check cves.cvss_score (if table exists)
	log.Println("[Migration 034] Checking cves.cvss_score type...")
	var hasCVEsTable bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'cves'
		)
	`).Scan(&hasCVEsTable).Error; err != nil {
		log.Printf("[Migration 034] ⚠️  Error checking cves table: %v", err)
	}

	if hasCVEsTable {
		var hasCVSSColumn bool
		if err := db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns 
				WHERE table_name = 'cves' 
				AND column_name = 'cvss_score'
			)
		`).Scan(&hasCVSSColumn).Error; err != nil {
			log.Printf("[Migration 034] ⚠️  Error checking cves.cvss_score column: %v", err)
		} else if hasCVSSColumn {
			var cvssType string
			if err := db.Raw(`
				SELECT data_type 
				FROM information_schema.columns 
				WHERE table_name = 'cves' 
				AND column_name = 'cvss_score'
			`).Scan(&cvssType).Error; err != nil {
				log.Printf("[Migration 034] ⚠️  Error checking cves.cvss_score type: %v", err)
			} else {
				if cvssType != "real" {
					log.Printf("[Migration 034] Converting cves.cvss_score from %s to real...", cvssType)
					if err := db.Exec(`
						ALTER TABLE cves 
						ALTER COLUMN cvss_score TYPE real USING cvss_score::real;
					`).Error; err != nil {
						log.Printf("[Migration 034] ⚠️  Error converting cves.cvss_score: %v", err)
					} else {
						log.Println("[Migration 034] ✅ Converted cves.cvss_score to real")
					}
				} else {
					log.Println("[Migration 034] ✅ cves.cvss_score already has type real")
				}
			}
		}
	} else {
		log.Println("[Migration 034] ℹ️  cves table does not exist (not using Trivy)")
	}

	log.Println("[Migration 034] ✅ Completed successfully")
	return nil
}

