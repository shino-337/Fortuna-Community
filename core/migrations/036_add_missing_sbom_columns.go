package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration036_AddMissingSBOMColumns adds missing columns to sboms table
// The Go model has PodUID, PodName, Namespace, ContainerName but database may not
func Migration036_AddMissingSBOMColumns(db *gorm.DB) error {
	log.Println("[Migration 036] Starting: Add missing SBOM columns")

	// Check and add pod_uid column
	var hasPodUID bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sboms' 
			AND column_name = 'pod_uid'
		)
	`).Scan(&hasPodUID).Error; err != nil {
		log.Printf("[Migration 036] ⚠️  Error checking pod_uid column: %v", err)
	}

	if !hasPodUID {
		log.Println("[Migration 036] Adding pod_uid column to sboms...")
		if err := db.Exec(`
			ALTER TABLE sboms 
			ADD COLUMN IF NOT EXISTS pod_uid VARCHAR(255);
		`).Error; err != nil {
			log.Printf("[Migration 036] ⚠️  Error adding pod_uid: %v", err)
		} else {
			log.Println("[Migration 036] ✅ Added pod_uid column to sboms")
			
			// Add index
			if err := db.Exec(`
				CREATE INDEX IF NOT EXISTS idx_sboms_pod_uid 
				ON sboms(pod_uid) 
				WHERE deleted_at IS NULL;
			`).Error; err != nil {
				log.Printf("[Migration 036] ⚠️  Error creating index: %v", err)
			}
		}
	} else {
		log.Println("[Migration 036] ✅ pod_uid column already exists")
	}

	// Check and add pod_name column
	var hasPodName bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sboms' 
			AND column_name = 'pod_name'
		)
	`).Scan(&hasPodName).Error; err != nil {
		log.Printf("[Migration 036] ⚠️  Error checking pod_name column: %v", err)
	}

	if !hasPodName {
		log.Println("[Migration 036] Adding pod_name column to sboms...")
		if err := db.Exec(`
			ALTER TABLE sboms 
			ADD COLUMN IF NOT EXISTS pod_name VARCHAR(255);
		`).Error; err != nil {
			log.Printf("[Migration 036] ⚠️  Error adding pod_name: %v", err)
		} else {
			log.Println("[Migration 036] ✅ Added pod_name column to sboms")
		}
	} else {
		log.Println("[Migration 036] ✅ pod_name column already exists")
	}

	// Check and add namespace column (may be named differently)
	var hasNamespace bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sboms' 
			AND column_name IN ('namespace', 'pod_namespace')
		)
	`).Scan(&hasNamespace).Error; err != nil {
		log.Printf("[Migration 036] ⚠️  Error checking namespace column: %v", err)
	}

	if !hasNamespace {
		log.Println("[Migration 036] Adding namespace column to sboms...")
		if err := db.Exec(`
			ALTER TABLE sboms 
			ADD COLUMN IF NOT EXISTS namespace VARCHAR(255);
		`).Error; err != nil {
			log.Printf("[Migration 036] ⚠️  Error adding namespace: %v", err)
		} else {
			log.Println("[Migration 036] ✅ Added namespace column to sboms")
			
			// Add index
			if err := db.Exec(`
				CREATE INDEX IF NOT EXISTS idx_sboms_namespace 
				ON sboms(namespace) 
				WHERE deleted_at IS NULL;
			`).Error; err != nil {
				log.Printf("[Migration 036] ⚠️  Error creating index: %v", err)
			}
		}
	} else {
		log.Println("[Migration 036] ✅ namespace column already exists")
	}

	// Check and add container_name column
	var hasContainerName bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sboms' 
			AND column_name = 'container_name'
		)
	`).Scan(&hasContainerName).Error; err != nil {
		log.Printf("[Migration 036] ⚠️  Error checking container_name column: %v", err)
	}

	if !hasContainerName {
		log.Println("[Migration 036] Adding container_name column to sboms...")
		if err := db.Exec(`
			ALTER TABLE sboms 
			ADD COLUMN IF NOT EXISTS container_name VARCHAR(255);
		`).Error; err != nil {
			log.Printf("[Migration 036] ⚠️  Error adding container_name: %v", err)
		} else {
			log.Println("[Migration 036] ✅ Added container_name column to sboms")
		}
	} else {
		log.Println("[Migration 036] ✅ container_name column already exists")
	}

	log.Println("[Migration 036] ✅ Completed successfully")
	return nil
}

