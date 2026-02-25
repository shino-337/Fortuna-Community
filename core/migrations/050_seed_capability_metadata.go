package migrations

import (
	"encoding/json"
	"log"

	"gorm.io/gorm"
)

// Migration050_SeedCapabilityMetadata seeds initial capability metadata
func Migration050_SeedCapabilityMetadata(db *gorm.DB) error {
	log.Println("Running migration 050: Seed capability_metadata")
	if !shouldRunSeedMigrations() {
		log.Println("[Migration 050] seed disabled (FORTUNA_ENABLE_SEED_DATA not set); set FORTUNA_ENABLE_SEED_DATA=true to populate Capability Catalog")
		return nil
	}

	// Check if table exists
	if !db.Migrator().HasTable("capability_metadata") {
		log.Println("[Migration 050] capability_metadata table does not exist, skipping seed")
		return nil
	}

	metadata := []map[string]interface{}{
		{
			"capability_id":              "ESC_PRIV_POD",
			"domain":                     "ESC",
			"category":                   "Privilege Escalation",
			"description":                "Pod has a container with privileged mode enabled",
			"severity_base":              "CRITICAL",
			"confidence_base":            0.7,
			"preconditions":              []string{},
			"produces_attack_steps":      []string{"NODE_FS_WRITE", "NODE_KERNEL_ACCESS"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": true,
		},
		{
			"capability_id":              "ESC_HOSTPID_POD",
			"domain":                     "ESC",
			"category":                   "Privilege Escalation",
			"description":                "Pod has hostPID enabled; can access the host process namespace",
			"severity_base":              "HIGH",
			"confidence_base":            0.6,
			"preconditions":              []string{},
			"produces_attack_steps":      []string{"PROC_NAMESPACE_ACCESS"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": true,
		},
		{
			"capability_id":              "ESC_HOSTIPC_POD",
			"domain":                     "ESC",
			"category":                   "Privilege Escalation",
			"description":                "Pod has hostIPC enabled; can access the host IPC namespace",
			"severity_base":              "HIGH",
			"confidence_base":            0.6,
			"preconditions":              []string{},
			"produces_attack_steps":      []string{"IPC_NAMESPACE_ACCESS"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": true,
		},
		{
			"capability_id":              "ESC_HOSTPATH_NODE",
			"domain":                     "ESC",
			"category":                   "Privilege Escalation",
			"description":                "Pod has hostPath mount; can access the node filesystem",
			"severity_base":              "CRITICAL",
			"confidence_base":            0.8,
			"preconditions":              []string{"ESC_PRIV_POD"},
			"produces_attack_steps":      []string{"NODE_FS_WRITE", "NODE_CRED_DUMP"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": true,
		},
		{
			"capability_id":              "ESC_RUNTIME_PROC_ROOT",
			"domain":                     "ESC",
			"category":                   "Container Escape",
			"description":                "Runtime signal confirms access to /proc/1/root (proc root pivot)",
			"severity_base":              "CRITICAL",
			"confidence_base":            0.9,
			"preconditions":              []string{"ESC_HOSTPATH_NODE", "ESC_PRIV_POD"},
			"produces_attack_steps":      []string{"NODE_FS_WRITE", "NODE_PERSISTENCE"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": false, // Already runtime-confirmed
		},
		{
			"capability_id":              "ESC_RUNTIME_PROBE",
			"domain":                     "ESC",
			"category":                   "Container Escape",
			"description":                "Static risk plus runtime signal indicates escape potential",
			"severity_base":              "HIGH",
			"confidence_base":            0.5,
			"preconditions":              []string{"ESC_HOSTPID_POD", "ESC_HOSTIPC_POD", "ESC_HOSTPATH_NODE"},
			"produces_attack_steps":      []string{"PROC_ROOT_PIVOT"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": true,
		},
		{
			"capability_id":              "ESC_RUNTIME_ACTIVE",
			"domain":                     "ESC",
			"category":                   "Container Escape",
			"description":                "Active runtime escape confirmed (high confidence)",
			"severity_base":              "CRITICAL",
			"confidence_base":            0.95,
			"preconditions":              []string{"ESC_RUNTIME_PROBE"},
			"produces_attack_steps":      []string{"NODE_FS_WRITE", "NODE_PERSISTENCE", "KUBELET_CRED_ACCESS"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": false, // Already runtime-confirmed
		},
		{
			"capability_id":              "ID_TOKEN_POD",
			"domain":                     "ID",
			"category":                   "Credential Access",
			"description":                "Pod can steal ServiceAccount token",
			"severity_base":              "MEDIUM",
			"confidence_base":            0.5,
			"preconditions":              []string{},
			"produces_attack_steps":      []string{"RBAC_ABUSE"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": true,
		},
		{
			"capability_id":              "NET_HOSTNETWORK",
			"domain":                     "NET",
			"category":                   "Network",
			"description":                "Pod uses hostNetwork; can access the host network",
			"severity_base":              "MEDIUM",
			"confidence_base":            0.4,
			"preconditions":              []string{},
			"produces_attack_steps":      []string{"NETWORK_SNIFFING"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": true,
		},
		{
			"capability_id":              "API_RBAC_WRITE_CLUSTER",
			"domain":                     "API",
			"category":                   "RBAC Abuse",
			"description":                "Pod has RBAC write access to the Kubernetes API",
			"severity_base":              "HIGH",
			"confidence_base":            0.7,
			"preconditions":              []string{"ID_TOKEN_POD"},
			"produces_attack_steps":      []string{"RBAC_ESCALATION", "RESOURCE_MANIPULATION"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": true,
		},
		{
			"capability_id":              "CTRL_CONTROL_PLANE_POD",
			"domain":                     "CTRL",
			"category":                   "Control Plane",
			"description":                "Pod runs in control plane namespace (kube-system)",
			"severity_base":              "MEDIUM",
			"confidence_base":            0.5,
			"preconditions":              []string{},
			"produces_attack_steps":      []string{"CONTROL_PLANE_ACCESS"},
			"expires_with_instance":      true,
			"supports_runtime_promotion": true,
		},
	}

	for _, m := range metadata {
		// Use raw SQL to insert with JSONB arrays
		preconditions := m["preconditions"].([]string)
		attackSteps := m["produces_attack_steps"].([]string)

		preconditionsJSON, err := json.Marshal(preconditions)
		if err != nil {
			log.Printf("[Migration 050] Failed to marshal preconditions for %s: %v", m["capability_id"], err)
			continue
		}
		attackStepsJSON, err := json.Marshal(attackSteps)
		if err != nil {
			log.Printf("[Migration 050] Failed to marshal attack steps for %s: %v", m["capability_id"], err)
			continue
		}

		// Use raw SQL with JSONB casting
		sql := `
			INSERT INTO capability_metadata (
				capability_id, domain, category, description, severity_base, 
				confidence_base, preconditions, produces_attack_steps, 
				expires_with_instance, supports_runtime_promotion
			) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10)
			ON CONFLICT (capability_id) DO UPDATE SET
				domain = EXCLUDED.domain,
				category = EXCLUDED.category,
				description = EXCLUDED.description,
				severity_base = EXCLUDED.severity_base,
				confidence_base = EXCLUDED.confidence_base,
				preconditions = EXCLUDED.preconditions,
				produces_attack_steps = EXCLUDED.produces_attack_steps,
				expires_with_instance = EXCLUDED.expires_with_instance,
				supports_runtime_promotion = EXCLUDED.supports_runtime_promotion,
				updated_at = CURRENT_TIMESTAMP
		`

		if err := db.Exec(sql,
			m["capability_id"],
			m["domain"],
			m["category"],
			m["description"],
			m["severity_base"],
			m["confidence_base"],
			string(preconditionsJSON),
			string(attackStepsJSON),
			m["expires_with_instance"],
			m["supports_runtime_promotion"],
		).Error; err != nil {
			log.Printf("[Migration 050] Failed to seed capability %s: %v", m["capability_id"], err)
			continue
		}
	}

	log.Println("[Migration 050] ✅ Completed successfully")
	return nil
}
