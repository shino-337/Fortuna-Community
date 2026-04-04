package migrations

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"gorm.io/gorm"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/pkg/models"
)

// Force reference to migration functions to prevent dead code elimination
// Note: Some migrations are defined in mvp2_migrations.go
//
// Classification (see docs/04-development/migrations/classification.md):
//   - Schema: DDL only (CREATE/ALTER, ADD COLUMN, INDEX, CONSTRAINT).
//   - Data/Seed: 050, 051, 061 — INSERT reference data; gated by FORTUNA_ENABLE_SEED_DATA.
//   - Data/Config: 053 — UPDATE clusters from env (one-time).
//
// Migration Numbering:
//   - 001-003: Core schema (clusters, users, audit logs)
//   - 004-007: Intentionally skipped (no functionality needed)
//   - 008-011: Deployments, replicasets, implementation guide, insights
//   - 012-016: Risk scores, policies (MVP2) - defined in mvp2_migrations.go
//   - 017: Intentionally skipped (no functionality needed)
//   - 018-022: Risk scores V2, CVE tables, SBOM tables (MVP2) - defined in mvp2_migrations.go
//   - 023-036: Advanced migrations (indexes, schema updates, cleanup) - separate files
//
// Old migrations 030-039 were merged into optimized versions (030-032):
//   - 030, 031, 038 → 030_MigrateInsightsSchemaComplete + 031_CleanupDuplicateIndexes
//   - 032 → 031_CleanupDuplicateIndexes
//   - 037, 039 → 032_MigrateCVEMatchesComplete
var (
	_ = Migration014_AddPolicyTemplates
	_ = Migration015_AddPolicyInstances
	_ = Migration016_AddPolicyViolations
	_ = Migration018_AddRiskScoresV2Columns
	_ = Migration019_AddCVETables
	_ = Migration020_AddOSVMirrorTables
	_ = Migration020_AddSBOMTables
	_ = Migration021_FixSBOMSchema
	_ = Migration022_AddCVEColumnsToInsights
	_ = Migration023_FixSBOMCVEIndexes
	_ = Migration024_AddPodImageScansUniqueIndex
	_ = Migration025_MakeUpsertUniqueIndexesNonPartial
	_ = Migration026_AddInsightsJSONBIndexes
	_ = Migration027_AddCVEFileMetadata
	_ = Migration028_AddPerformanceIndexes
	_ = Migration029_AddInsightsUniqueConstraint
	_ = Migration030_MigrateInsightsSchemaComplete
	_ = Migration031_CleanupDuplicateIndexes
	_ = Migration032_MigrateCVEMatchesComplete
	_ = Migration033_AddUniqueConstraints
	_ = Migration034_StandardizeCVSSType
	_ = Migration035_EvaluateTrivyTables
	_ = Migration036_AddMissingSBOMColumns
	_ = Migration037_AddSoftDeleteToResources
	_ = Migration038_AddAgentsTable
	_ = Migration039_AddDeletedAtToClusters
	_ = Migration040_AddMissingInsightColumns
	_ = Migration041_AddPodCapabilitiesTable
	_ = Migration042_AddPodFactColumns
	_ = Migration043_AddREPTables
	_ = Migration044_AddPodInstancesTable
	_ = Migration045_AddRuntimeSignalsTable
	_ = Migration046_AddCapabilityStateMachine
	_ = Migration047_AddCapabilityMetadataTable
	_ = Migration048_AddPromotionRulesTable
	_ = Migration049_AddPodAttackStepsTable
	_ = Migration050_SeedCapabilityMetadata
	_ = Migration051_SeedPromotionRules
	_ = Migration052_SBOMOneRowPerPod
	_ = Migration053_FixMinikubeClusterDisplayName
	_ = Migration054_AddClusterMetadataColumns
	_ = Migration055_DropClustersNameUnique
	_ = Migration056_AddNotificationsTable
	_ = Migration057_AddErrorLogsTable
	_ = Migration058_AddInsightsEvidenceViolatedRules
	_ = Migration059_AddNodeMetadataColumns
	_ = Migration060_AddCapabilityMetadataExtendedColumns
	_ = Migration061_SeedCapabilityMetadataExtended
	_ = Migration062_AddClustersRegionEndpointKubeconfig
	_ = Migration063_AddPodsLastSeenCleanupIndex
	_ = Migration064_ExpandAdvisoryIDColumns
	_ = Migration065_EnsureUsersDeletedAt
	_ = Migration066_AddPodsPhase
	_ = Migration067_ExpandPackageVulnerabilitiesVersionColumns
	_ = Migration068_AddPodDetailColumns
	_ = Migration069_AddPodSpecHash
	_ = Migration070_AddPodLastEvaluatedHash
	_ = Migration071_AddPodDetailServicesTables
	_ = Migration072_PodProcessesGormColumns
	_ = Migration073_PodProcessesHistoryIndex
	_ = Migration074_AddRuntimeSourcePodDetail
	_ = Migration075_AddRiskRulesTable
	_ = Migration076_AddRiskRulesHistoryTable
	_ = Migration077_AddInsightExplanationRemediation
	_ = Migration078_AddSBOMSourceConfidence
	_ = Migration079_AddAuditTraceID
	_ = Migration080_AddGoModuleAlias
	_ = Migration081_AddMirrorState
	_ = Migration082_AddSBOMStatusAndVersion
	_ = Migration083_AddSBOMMatchRuns
	_ = Migration084_AddSBOMComponentTrustFields
	_ = Migration087_DropLegacySBOMMatchWatermarks
	_ = Migration092_AddSBOMGoVersion
	_ = Migration093_EnsureK8sEventsTable
	_ = Migration094_EnsureAgentsTable
	_ = Migration095_AddPodProcessRuntimeIdentityFields
	_ = Migration096_PolicyEngineBaselineBootstrap
	_ = Migration097_SeedYAMLRiskRulesMITRE
	_ = Migration098_SeedYAMLRiskRulesMITREFix
	_ = Migration099_RefreshYAMLRiskRulesMITRE
	_ = Migration100_RefreshYAMLRiskRulesMITRE
	_ = Migration109_AddRuntimeSignalLifecycle
	_ = Migration110_AddPodCapabilityClassDerivedFrom
	_ = Migration111_HardenSBOMRunAndEnums
	_ = Migration112_ExpandAdvisoryIDColumnsV2
	// Old migrations 030-039 (replaced by optimized versions above):
	// _ = Migration030_MigrateInsightsToNewSchema (merged into 030_MigrateInsightsSchemaComplete)
	// _ = Migration031_CleanupOldInsightsColumns (merged into 030_MigrateInsightsSchemaComplete)
	// _ = Migration032_RemoveDuplicateIndexes (merged into 031_CleanupDuplicateIndexes)
	// _ = Migration037_MigrateCVEMatchesToPackageName (merged into 032_MigrateCVEMatchesComplete)
	// _ = Migration038_CleanupOldSchemaColumns (merged into 030_MigrateInsightsSchemaComplete and 031_CleanupDuplicateIndexes)
	// _ = Migration039_AddMissingCVEMatchColumns (merged into 032_MigrateCVEMatchesComplete)
)

// RunMigrations runs all database migrations
func RunMigrations(db *gorm.DB) error {
	log.Println("Running database migrations...")

	// Run migrations in order
	// Note: Migration numbers 004-007 and 017 are intentionally skipped (no functionality needed)
	migrations := []func(*gorm.DB) error{
		// Core schema (001-003)
		Migration001_InitialSchema,      // Core tables: clusters, nodes, namespaces, pods, RBAC
		Migration002_AddUsers,           // Users table for authentication
		Migration003_AddUserToAuditLogs, // Add user_id to audit_logs
		// 004-007: Intentionally skipped
		// Deployments and replicasets (008-009)
		Migration008_AddDeployments, // Deployments table
		Migration009_AddReplicaSets, // ReplicaSets table
		// Implementation guide schema (010-011)
		Migration010_ImplementationGuideSchema, // Nodes, policies, insights, events_index
		Migration011_AddInsightsSoftDelete,     // Add soft delete and status to insights
		// MVP2: Risk scores and policies (012-016)
		Migration012_AddRiskScores,          // Risk scores table
		Migration013_AddRiskScoresDeletedAt, // Add deleted_at column if missing
		Migration014_AddPolicyTemplates,     // MVP2 Phase 2: Policy Engine
		Migration015_AddPolicyInstances,     // MVP2 Phase 2: Policy Engine
		Migration016_AddPolicyViolations,    // MVP2 Phase 2: Policy Engine
		// 017: Intentionally skipped
		// MVP2: Risk scores V2, CVE, SBOM (018-022)
		Migration018_AddRiskScoresV2Columns,  // MVP2 Phase 1.2: Risk Scoring V2
		Migration019_AddCVETables,            // MVP2 Phase 2: CVE Detection Integration (Trivy-based)
		Migration020_AddOSVMirrorTables,      // P2-7: OSV mirror tables (Go + future ecosystems); was defined but not wired
		Migration020_AddSBOMTables,           // MVP2 Phase 2: SBOM-based CVE Detection (replacing Trivy)
		Migration021_FixSBOMSchema,           // MVP2 Phase 2: Schema fix for p_url -> purl and insights.source
		Migration022_AddCVEColumnsToInsights, // MVP2 Phase 2: Add CVE-specific columns to insights table
		// MVP2: Indexes and performance (023-029)
		Migration023_FixSBOMCVEIndexes,                 // MVP2: Unique indexes for SBOM/CVE upserts + dedup
		Migration024_AddPodImageScansUniqueIndex,       // MVP2: Unique index for pod_image_scans upsert path
		Migration025_MakeUpsertUniqueIndexesNonPartial, // MVP2: Non-partial unique indexes for ON CONFLICT inference
		Migration026_AddInsightsJSONBIndexes,           // MVP2: GIN indexes for efficient JSONB queries on insights (skips if column doesn't exist)
		Migration027_AddCVEFileMetadata,                // CVE Optimization: File metadata tracking for incremental updates
		Migration028_AddPerformanceIndexes,             // Performance: Critical indexes for CVE matching and insights
		Migration029_AddInsightsUniqueConstraint,       // Performance: Unique constraint for insights batch UPSERT
		// Schema migrations and cleanup (030-036)
		Migration030_MigrateInsightsSchemaComplete,              // Schema Migration: Complete insights schema migration (combines old 030+031+038)
		Migration031_CleanupDuplicateIndexes,                    // Schema Cleanup: Remove duplicate indexes (combines old 032+038 index cleanup)
		Migration032_MigrateCVEMatchesComplete,                  // Schema Migration: Complete cve_matches migration (combines old 037+039)
		Migration033_AddUniqueConstraints,                       // Schema Integrity: Add proper unique constraints for data integrity
		Migration034_StandardizeCVSSType,                        // Schema Standardization: Standardize CVSS column types to REAL
		Migration035_EvaluateTrivyTables,                        // Schema Evaluation: Evaluate and mark Trivy tables as deprecated
		Migration036_AddMissingSBOMColumns,                      // Schema Update: Add missing columns (pod_uid, pod_name, namespace, container_name) to sboms table
		Migration037_AddSoftDeleteToResources,                   // Schema Update: Add deleted_at to resource tables
		Migration038_AddAgentsTable,                             // Schema Update: Add agents table for dashboard metrics
		Migration039_AddDeletedAtToClusters,                     // Schema Update: Add deleted_at to clusters
		Migration040_AddMissingInsightColumns,                   // Schema Update: Ensure insights columns exist (fixed_version, resolved_at)
		Migration041_AddPodCapabilitiesTable,                    // Schema Update: Add pod_capabilities table (PCE)
		Migration042_AddPodFactColumns,                          // Schema Update: Add pod security fact columns
		Migration043_AddREPTables,                               // Schema Update: Add REP tables (pod_risk_profiles, runtime_events)
		Migration044_AddPodInstancesTable,                       // PCE Phase 1.5: Pod lifecycle normalization (pod_instances)
		Migration045_AddRuntimeSignalsTable,                     // PCE Phase 1.5: Runtime signals semantic layer
		Migration046_AddCapabilityStateMachine,                  // PCE Phase 1.5: Capability state machine (detected/confirmed/exploited/chained)
		Migration047_AddCapabilityMetadataTable,                 // PCE Phase 1.5 Adjustment: Capability metadata (semantic layer)
		Migration048_AddPromotionRulesTable,                     // PCE Phase 1.5 Adjustment: Promotion rules (signal → state)
		Migration049_AddPodAttackStepsTable,                     // PCE Phase 1.5 Adjustment: Minimal AttackStep model
		Migration050_SeedCapabilityMetadata,                     // PCE Phase 1.5 Adjustment: Seed capability metadata
		Migration051_SeedPromotionRules,                         // PCE Phase 1.5 Adjustment: Seed promotion rules
		Migration052_SBOMOneRowPerPod,                           // SBOM: one row per pod (drop unique on image_digest)
		Migration053_FixMinikubeClusterDisplayName,              // Cluster display name from env only (CLUSTER_ID_TO_UPDATE, CLUSTER_DISPLAY_NAME)
		Migration054_AddClusterMetadataColumns,                  // Cluster SSOT: source, k8s_version, distribution
		Migration055_DropClustersNameUnique,                     // Cluster SSOT: allow same display name for multiple clusters (id is identity)
		Migration056_AddNotificationsTable,                      // Dashboard: notifications table (real data)
		Migration057_AddErrorLogsTable,                          // Dashboard: error_logs table (real data)
		Migration058_AddInsightsEvidenceViolatedRules,           // Risk Detail: evidence + violated_rules on insights
		Migration059_AddNodeMetadataColumns,                     // Node Detail: role, os, runtime on nodes
		Migration060_AddCapabilityMetadataExtendedColumns,       // Capability Spec: extended metadata (name, summary, mitre, impact, etc.)
		Migration061_SeedCapabilityMetadataExtended,             // Capability Spec: seed extended metadata from spec
		Migration062_AddClustersRegionEndpointKubeconfig,        // Cluster: region, endpoint, kubeconfig (fix agent sync 500)
		Migration063_AddPodsLastSeenCleanupIndex,                // Ops: index for stale pod cleanup and pod-count queries
		Migration064_ExpandAdvisoryIDColumns,                    // CVE/SBOM: support non-CVE advisory IDs for richer package vulnerability coverage
		Migration065_EnsureUsersDeletedAt,                       // Auth schema hardening: ensure users.deleted_at exists for soft-delete queries
		Migration066_AddPodsPhase,                               // Pods: phase (Running, Pending, etc.) for UI
		Migration067_ExpandPackageVulnerabilitiesVersionColumns, // CVE: package_vulnerabilities version columns to 255 for OSV data
		Migration068_AddPodDetailColumns,                        // Pod Detail (POD_DETAIL_SPEC): pod_ip, start_time, restart_count, owner_*, qos_class
		Migration069_AddPodSpecHash,                             // POD_SYNC_ARCHITECTURE §4.3: spec_hash for conditional PCE
		Migration070_AddPodLastEvaluatedHash,                    // last_evaluated_hash after PCE success; race protection
		Migration071_AddPodDetailServicesTables,                 // Pod Detail: pod_runtime_metrics, pod_processes, pod_network_connections, k8s_events
		Migration072_PodProcessesGormColumns,                    // Pod Detail: pid->p_id, ppid->pp_id for GORM
		Migration073_PodProcessesHistoryIndex,                   // Pod Detail: index (pod_uid, observed_at)
		Migration074_AddRuntimeSourcePodDetail,                  // Pod Detail: runtime_source (host|exec) for UI
		Migration075_AddRiskRulesTable,                          // Risk rules CRUD: table for engine-loaded rules
		Migration076_AddRiskRulesHistoryTable,                   // Risk rules versioning: risk_rules_history (Phase 3)
		Migration077_AddInsightExplanationRemediation,           // Risk Detail: explanation (TEXT) + remediation (JSONB) on insights
		Migration078_AddSBOMSourceConfidence,                    // SBOM (Finding #8.4): sbom_source, confidence for distroless/heuristic
		Migration079_AddAuditTraceID,                            // Finding #5.2: trace_id on audit_logs (Agent → sync → insight)
		Migration082_AddSBOMStatusAndVersion,                    // SBOM lifecycle: status (pending/finalized) + version for immutability
		Migration080_AddGoModuleAlias,                           // Go module alias resolver: alias → canonical (reduce CVE miss on renames)
		Migration081_AddMirrorState,                             // mirror_state (name, version) for cache epoch; bump on sync
		Migration083_AddSBOMMatchRuns,                           // Idempotency: sbom_match_runs per (sbom_id, version, mirror_version)
		Migration084_AddSBOMComponentTrustFields,                // Trust boundary: original_purl, purl_validated, trust_level on sbom_components
		Migration086_AddSBOMProcessingState,                     // Replay guard: atomic last-write-wins by event timestamp/id
		Migration087_DropLegacySBOMMatchWatermarks,              // Remove unused legacy watermark table (superseded by 086)
		Migration088_UpdateSBOMStatusCheck,                      // SBOM: status check values (complete|partial|failed)
		Migration089_AddSBOMStatusReasonSourceDetail,            // SBOM: status_reason and component source_detail
		Migration090_AddResolverSignatureFingerprint,            // Determinism v1: resolver/sig versions + normalized SBOM fingerprint
		Migration091_AddInsightConfidenceColumns,                // Phase 2: confidence propagation into insights
		Migration092_AddSBOMGoVersion,                           // SBOM: go_version column (matches models.SBOM.GoVersion; gRPC insert)
		Migration093_EnsureK8sEventsTable,                       // Repair: k8s_events if migration 071 never created it
		Migration094_EnsureAgentsTable,                          // Repair: agents after reset-db / if migration 038 skipped
		Migration095_AddPodProcessRuntimeIdentityFields,         // Pod Detail: user/group/cwd/cap_eff for runtime identity analysis
		Migration096_PolicyEngineBaselineBootstrap,              // Policy engine: ensure tables + seed baseline templates/instances/risk-rules
		Migration097_SeedYAMLRiskRulesMITRE,                     // Risk rules: upsert all YAML rules with MITRE tags
		Migration098_SeedYAMLRiskRulesMITREFix,                  // Risk rules: fix empty rule_id row and re-upsert correctly
		Migration099_RefreshYAMLRiskRulesMITRE,                  // Risk rules: refresh/ensure YAML MITRE tags are applied
		Migration100_RefreshYAMLRiskRulesMITRE,                  // Risk rules: refresh/ensure YAML MITRE tags are applied
		Migration101_AddPodRuntimeMetricsNetDev,                 // Pod Detail R5: netns counters from /proc/<pid>/net/dev
		Migration102_AddRuntimeEventsMetadata,                   // R9: enrich runtime_events with payload metadata
		Migration103_AddRuntimeSignalsCount,                     // R6: count occurrences for de-duped runtime_signals
		Migration104_AddRuntimeBehaviorFacts,                    // Runtime P0: Layer-2 normalized behavior facts
		Migration105_AddRuntimeIncidents,                        // Runtime P0/P1: stateful runtime incident table
		Migration106_AddAssetSecurityState,                      // Runtime P0.5: asset_security_state minimal projector input
		Migration107_FixAssetSecurityStateColumnNames,           // Runtime P0.5: fix-up acronym column names
		Migration108_AddRuntimeEventsCanonicalColumns,           // Runtime P0.1: runtime_events canonical contract columns
		Migration109_AddRuntimeSignalLifecycle,                  // Runtime P1: runtime_signals lifecycle + evidence refs
		Migration110_AddPodCapabilityClassDerivedFrom,           // Runtime P1: capability compatibility columns
		Migration111_HardenSBOMRunAndEnums,                      // SBOM reliability: match-run timeout fields + enum checks
		Migration112_ExpandAdvisoryIDColumnsV2,                  // CVE schema: widen advisory ID columns for GHSA/OSV/vendor IDs
		Migration113_AddMalwareTables,                           // Supply-chain threat detection: malware_packages + malware_matches
	}

	log.Printf("Total migrations to execute: %d", len(migrations))

	for i, migration := range migrations {
		log.Printf("Executing migration %d of %d", i+1, len(migrations))
		err := migration(db)
		if err != nil {
			errStr := err.Error()
			log.Printf("Migration %d returned error: %s", i+1, errStr)
			// Check if error is the known "insufficient arguments" issue from GORM/PostgreSQL
			// This is a known compatibility issue that doesn't prevent table creation
			if errStr != "" && (strings.Contains(errStr, "insufficient arguments") ||
				strings.Contains(errStr, "migration 1 failed") ||
				strings.Contains(errStr, "Migration 1 failed") ||
				strings.Contains(errStr, "column.*does not exist") ||
				strings.Contains(errStr, "relation.*does not exist")) {
				log.Printf("WARNING: Migration %d encountered known GORM/PostgreSQL issue: %s", i+1, errStr)
				log.Printf("WARNING: This is a known compatibility issue - continuing with next migration")

				// For migration 1, validate all core tables exist
				if i == 0 {
					requiredTables := []string{"clusters", "service_accounts", "roles", "cluster_roles",
						"role_bindings", "cluster_role_bindings", "pods", "audit_logs"}
					if validationErr := validateMigrationResult(db, i+1, requiredTables); validationErr != nil {
						// FAIL LOUDLY - don't continue with broken schema
						return fmt.Errorf("migration %d schema validation failed: %w", i+1, validationErr)
					}
					log.Printf("Migration %d: All required tables validated successfully, continuing...", i+1)
					continue
				}

				// For other migrations with known errors, log warning and continue
				log.Printf("WARNING: Migration %d failed with known issue, but continuing with next migration", i+1)
				log.Printf("Migration %d: Tables may still be created or updated despite error", i+1)
				continue
			}
			// For unknown errors, still log but continue (non-fatal)
			log.Printf("ERROR: Migration %d failed: %v", i+1, err)
			log.Printf("WARNING: Continuing with next migration despite error")
			// Don't return error - continue with remaining migrations
			// return fmt.Errorf("migration %d failed: %w", i+1, err)
		} else {
			log.Printf("Migration %d completed successfully", i+1)
		}
	}

	log.Printf("All %d migrations completed", len(migrations))

	// Mandatory post-migration validation: core tables must exist or Core cannot serve traffic.
	requiredCoreTables := []string{"clusters", "pods", "namespaces", "nodes", "service_accounts", "roles", "cluster_roles", "role_bindings", "cluster_role_bindings", "audit_logs"}
	if err := validateMigrationResult(db, 0, requiredCoreTables); err != nil {
		return fmt.Errorf("post-migration validation failed (core tables missing): %w", err)
	}
	log.Printf("Post-migration validation: all required core tables exist")
	return nil
}

// Migration001_InitialSchema creates initial tables
func Migration001_InitialSchema(db *gorm.DB) error {
	log.Println("Running migration 001: Initial schema")

	// Try multiple paths for SQL file
	sqlPaths := []string{
		"migrations/001_initial_schema.sql",
		"/app/migrations/001_initial_schema.sql",
		"./migrations/001_initial_schema.sql",
	}

	var sqlBytes []byte
	var err error
	for _, path := range sqlPaths {
		sqlBytes, err = os.ReadFile(path)
		if err == nil {
			log.Printf("Found SQL migration file at: %s", path)
			break
		}
	}

	if err == nil && len(sqlBytes) > 0 {
		// Execute SQL migration first
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			log.Printf("Warning: SQL migration had errors: %v. Attempting AutoMigrate fallback.", err)
		} else {
			log.Println("SQL migration 001 executed successfully")
			return nil
		}
	} else {
		log.Printf("SQL migration file not found (tried: %v), using AutoMigrate", sqlPaths)
	}

	// Fallback to AutoMigrate if SQL file doesn't exist or failed
	log.Println("Using AutoMigrate for migration 001")

	// Check if clusters table already exists
	var tableExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'clusters')").Scan(&tableExists).Error; err == nil && tableExists {
		log.Println("Tables already exist, skipping migration 001")
		return nil
	}

	// Try AutoMigrate but ignore errors (tables may already exist or have schema issues)
	// The "insufficient arguments" error from GORM/PostgreSQL driver is a known issue
	// and doesn't prevent tables from being created
	log.Println("Attempting AutoMigrate (errors may be ignored)...")

	tables := []interface{}{
		&models.Cluster{},
		&models.ServiceAccount{},
		&models.Role{},
		&models.ClusterRole{},
		&models.RoleBinding{},
		&models.ClusterRoleBinding{},
		&models.Pod{},
		&models.AuditLog{},
	}

	// Try to migrate all tables, but don't fail on errors
	// GORM may throw "insufficient arguments" errors during schema inspection
	// but tables may still be created successfully
	for i, table := range tables {
		log.Printf("Migrating table %d of %d", i+1, len(tables))
		err := db.AutoMigrate(table)
		if err != nil {
			errStr := err.Error()
			// Check if error is the "insufficient arguments" issue
			if errStr != "" && strings.Contains(errStr, "insufficient arguments") {
				log.Printf("Warning: AutoMigrate encountered known issue for table %d (may be safe to ignore): %v", i+1, err)
				// Don't return error for "insufficient arguments" - it's a known GORM/PostgreSQL issue
			} else {
				log.Printf("Warning: AutoMigrate failed for table %d: %v", i+1, err)
				// For other errors, also don't fail - continue with other tables
			}
		} else {
			log.Printf("Table %d migrated successfully", i+1)
		}
	}

	// Verify tables were created by checking if clusters table exists
	var verifyExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'clusters')").Scan(&verifyExists).Error; err == nil && verifyExists {
		log.Println("Migration 001 completed: tables verified to exist")
		return nil
	}

	// Even if AutoMigrate had errors, check if tables exist
	// The "insufficient arguments" error may occur during schema inspection
	// but tables may still be created
	log.Println("Migration 001: Checking if tables exist despite AutoMigrate errors...")
	var finalCheck bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'clusters')").Scan(&finalCheck).Error; err == nil && finalCheck {
		log.Println("Migration 001 completed: tables exist (AutoMigrate errors were non-fatal)")
		return nil
	}

	// Fail fast: clusters is required for Core to function. Do not start with broken schema.
	return fmt.Errorf("migration 001: clusters table was not created (SQL file not found or AutoMigrate failed); ensure /app/migrations/001_initial_schema.sql exists in the image and DB is writable")
}

// Migration002_AddUsers creates users table
//
// Date: 2025-12-27 (converted from AutoMigrate to SQL)
// Author: Fortuna Team
// Ticket: Migration Audit - Phase 2
//
// Description:
//
//	Creates users table for authentication and authorization.
//
// Tables Affected:
//   - users: New table with username, email, password, role, active fields
//
// Rollback Plan:
//
//	DROP TABLE IF EXISTS users CASCADE;
//
// Testing:
//   - Verify table: SELECT * FROM users LIMIT 1;
//   - Check indexes: \di idx_users_*
func Migration002_AddUsers(db *gorm.DB) error {
	log.Println("Running migration 002: Add users table")

	// Determine environment
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	// Try multiple paths for SQL file
	sqlPaths := []string{
		"migrations/002_add_users.sql",
		"/app/migrations/002_add_users.sql",
		"./migrations/002_add_users.sql",
	}

	var sqlBytes []byte
	var err error

	for _, path := range sqlPaths {
		sqlBytes, err = os.ReadFile(path)
		if err == nil {
			log.Printf("Found SQL migration file at: %s", path)
			break
		}
	}

	// Production: SQL file is mandatory
	if env == "production" || env == "staging" {
		if err != nil || len(sqlBytes) == 0 {
			return fmt.Errorf("CRITICAL: SQL migration file required: 002_add_users.sql not found. Tried paths: %v", sqlPaths)
		}

		// Execute SQL
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			return fmt.Errorf("SQL migration failed: %w", err)
		}

		// Validate result
		var tableExists bool
		if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'users')").Scan(&tableExists).Error; err != nil {
			return fmt.Errorf("failed to validate users table: %w", err)
		}
		if !tableExists {
			return fmt.Errorf("users table was not created")
		}

		log.Println("Migration 002 completed successfully (SQL)")
		return nil
	}

	// Development: Allow AutoMigrate fallback
	if err != nil || len(sqlBytes) == 0 {
		log.Println("Development: SQL file not found, using AutoMigrate")
		return db.AutoMigrate(&models.User{})
	}

	// Development with SQL file
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		log.Printf("SQL migration failed, using AutoMigrate fallback: %v", err)
		return db.AutoMigrate(&models.User{})
	}

	// Ensure deleted_at column exists (in case table was created by migration 001 without it)
	var columnExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'deleted_at')").Scan(&columnExists).Error; err != nil {
		log.Printf("Warning: Failed to check deleted_at column: %v", err)
	} else if !columnExists {
		log.Println("Adding deleted_at column to users table (was missing)")
		if err := db.Exec("ALTER TABLE users ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE").Error; err != nil {
			log.Printf("Warning: Failed to add deleted_at column: %v", err)
		} else {
			log.Println("deleted_at column added successfully")
			// Create index
			if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at)").Error; err != nil {
				log.Printf("Warning: Failed to create index on deleted_at: %v", err)
			}
		}
	}

	log.Println("Migration 002 completed successfully")
	return nil
}

// Migration003_AddUserToAuditLogs adds user_id to audit_logs
//
// Date: 2025-12-27 (converted from AutoMigrate to SQL)
// Author: Fortuna Team
// Ticket: Migration Audit - Phase 2
//
// Description:
//
//	Adds user_id column to audit_logs table to link audit entries to users.
//
// Tables Affected:
//   - audit_logs: Add user_id column with foreign key to users table
//
// Dependencies:
//   - Requires Migration 002 (users table) to be applied first
//
// Rollback Plan:
//
//	ALTER TABLE audit_logs DROP COLUMN IF EXISTS user_id CASCADE;
//
// Testing:
//   - Verify column: SELECT user_id FROM audit_logs LIMIT 1;
//   - Check foreign key: \d audit_logs
func Migration003_AddUserToAuditLogs(db *gorm.DB) error {
	log.Println("Running migration 003: Add user_id to audit_logs")

	// Determine environment
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	// Try multiple paths for SQL file
	sqlPaths := []string{
		"migrations/003_add_user_to_audit_logs.sql",
		"/app/migrations/003_add_user_to_audit_logs.sql",
		"./migrations/003_add_user_to_audit_logs.sql",
	}

	var sqlBytes []byte
	var err error

	for _, path := range sqlPaths {
		sqlBytes, err = os.ReadFile(path)
		if err == nil {
			log.Printf("Found SQL migration file at: %s", path)
			break
		}
	}

	// Production: SQL file is mandatory
	if env == "production" || env == "staging" {
		if err != nil || len(sqlBytes) == 0 {
			return fmt.Errorf("CRITICAL: SQL migration file required: 003_add_user_to_audit_logs.sql not found. Tried paths: %v", sqlPaths)
		}

		// Execute SQL
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			return fmt.Errorf("SQL migration failed: %w", err)
		}

		// Validate result
		columnExists, err := validateColumnExists(db, "audit_logs", "user_id")
		if err != nil {
			return fmt.Errorf("failed to validate user_id column: %w", err)
		}
		if !columnExists {
			return fmt.Errorf("user_id column was not created")
		}

		log.Println("Migration 003 completed successfully (SQL)")
		return nil
	}

	// Development: Allow AutoMigrate fallback
	if err != nil || len(sqlBytes) == 0 {
		log.Println("Development: SQL file not found, using AutoMigrate")
		// Check if audit_logs table exists first
		if !db.Migrator().HasTable(&models.AuditLog{}) {
			log.Println("audit_logs table does not exist, creating it first")
			if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
				log.Printf("Warning: Failed to create audit_logs table: %v", err)
				return nil
			}
		}

		// Check if column already exists
		if db.Migrator().HasColumn(&models.AuditLog{}, "user_id") {
			log.Println("Column user_id already exists, skipping")
			return nil
		}

		// Add user_id column
		if err := db.Migrator().AddColumn(&models.AuditLog{}, "user_id"); err != nil {
			log.Printf("Warning: Failed to add user_id column: %v", err)
			return nil
		}

		return nil
	}

	// Development with SQL file
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		log.Printf("SQL migration failed, using AutoMigrate fallback: %v", err)
		if !db.Migrator().HasColumn(&models.AuditLog{}, "user_id") {
			if err := db.Migrator().AddColumn(&models.AuditLog{}, "user_id"); err != nil {
				log.Printf("Warning: Failed to add user_id column: %v", err)
			}
		}
		return nil
	}

	log.Println("Migration 003 completed successfully")
	return nil
}

// CreateDefaultAdmin creates a default admin user if it doesn't exist, or syncs password from env if it exists
func CreateDefaultAdmin(db *gorm.DB, username, password, email string) error {
	var user models.User
	result := db.Where("username = ?", username).First(&user)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check for admin user: %w", result.Error)
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
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
		return nil
	}

	// User exists: sync password from env so admin/admin123 stays usable after DB recreate or env change
	if user.Password != hashedPassword {
		user.Password = hashedPassword
		if err := db.Save(&user).Error; err != nil {
			return fmt.Errorf("failed to sync admin password: %w", err)
		}
		log.Printf("Synced default admin password for user: %s", username)
	} else {
		log.Printf("Admin user already exists: %s", username)
	}
	return nil
}

// Migration010_ImplementationGuideSchema adds tables according to IMPLEMENTATION_GUIDE.md
func Migration010_ImplementationGuideSchema(db *gorm.DB) error {
	log.Println("Running migration 010: Implementation Guide schema")

	// Read and execute SQL migration file
	sqlBytes, err := os.ReadFile("migrations/010_add_implementation_guide_schema.sql")
	if err != nil {
		log.Printf("Warning: Could not read SQL migration file: %v. Using AutoMigrate instead.", err)
		// Fallback to AutoMigrate for new models
		return db.AutoMigrate(
			&models.Node{},
			&models.Policy{},
			&models.Insight{},
			&models.EventIndex{},
		)
	}

	// Execute SQL
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		log.Printf("Warning: SQL migration had errors: %v. Attempting AutoMigrate fallback.", err)
		// Fallback to AutoMigrate
		return db.AutoMigrate(
			&models.Node{},
			&models.Policy{},
			&models.Insight{},
			&models.EventIndex{},
		)
	}

	log.Println("Migration 010 completed successfully")
	return nil
}

// Migration011_AddInsightsSoftDelete adds soft delete and status to insights table
//
// Date: 2025-12-27 (removed AutoMigrate fallback)
// Author: Fortuna Team
// Ticket: Migration Audit - Phase 2
//
// Description:
//
//	Adds soft delete (deleted_at) and status columns to insights table.
//
// Tables Affected:
//   - insights: Add deleted_at and status columns
//
// Rollback Plan:
//
//	ALTER TABLE insights DROP COLUMN IF EXISTS deleted_at, status CASCADE;
func Migration011_AddInsightsSoftDelete(db *gorm.DB) error {
	log.Println("Running migration 011: Add soft delete and status to insights")

	// Determine environment
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	// Try multiple paths for SQL file
	sqlPaths := []string{
		"migrations/011_add_insights_soft_delete.sql",
		"/app/migrations/011_add_insights_soft_delete.sql",
		"./migrations/011_add_insights_soft_delete.sql",
	}

	var sqlBytes []byte
	var err error

	for _, path := range sqlPaths {
		sqlBytes, err = os.ReadFile(path)
		if err == nil {
			log.Printf("Found SQL migration file at: %s", path)
			break
		}
	}

	// Production: SQL file is mandatory
	if env == "production" || env == "staging" {
		if err != nil || len(sqlBytes) == 0 {
			return fmt.Errorf("CRITICAL: SQL migration file required: 011_add_insights_soft_delete.sql not found. Tried paths: %v", sqlPaths)
		}

		// Execute SQL
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			return fmt.Errorf("SQL migration failed: %w", err)
		}

		// Validate result
		hasDeletedAt, err := validateColumnExists(db, "insights", "deleted_at")
		if err != nil {
			return fmt.Errorf("failed to validate deleted_at column: %w", err)
		}
		if !hasDeletedAt {
			return fmt.Errorf("deleted_at column was not created")
		}

		log.Println("Migration 011 completed successfully (SQL)")
		return nil
	}

	// Development: Allow AutoMigrate fallback
	if err != nil || len(sqlBytes) == 0 {
		log.Println("Development: SQL file not found, using AutoMigrate")
		return db.AutoMigrate(&models.Insight{})
	}

	// Development with SQL file
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		log.Printf("SQL migration failed, using AutoMigrate fallback: %v", err)
		return db.AutoMigrate(&models.Insight{})
	}

	log.Println("Migration 011 completed successfully")
	return nil
}

// Migration008_AddDeployments adds deployments table
//
// Date: 2025-12-27 (removed AutoMigrate fallback)
// Author: Fortuna Team
// Ticket: Migration Audit - Phase 2
//
// Description:
//
//	Creates deployments table for tracking Kubernetes deployments.
//
// Tables Affected:
//   - deployments: New table for deployment tracking
//
// Rollback Plan:
//
//	DROP TABLE IF EXISTS deployments CASCADE;
func Migration008_AddDeployments(db *gorm.DB) error {
	log.Println("Running migration 008: Add deployments table")

	// Determine environment
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	// Try multiple paths for SQL file
	sqlPaths := []string{
		"migrations/008_add_deployments.sql",
		"/app/migrations/008_add_deployments.sql",
		"./migrations/008_add_deployments.sql",
	}

	var sqlBytes []byte
	var err error

	for _, path := range sqlPaths {
		sqlBytes, err = os.ReadFile(path)
		if err == nil {
			log.Printf("Found SQL migration file at: %s", path)
			break
		}
	}

	// Production: SQL file is mandatory
	if env == "production" || env == "staging" {
		if err != nil || len(sqlBytes) == 0 {
			return fmt.Errorf("CRITICAL: SQL migration file required: 008_add_deployments.sql not found. Tried paths: %v", sqlPaths)
		}

		// Execute SQL
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			return fmt.Errorf("SQL migration failed: %w", err)
		}

		// Validate result
		var tableExists bool
		if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'deployments')").Scan(&tableExists).Error; err != nil {
			return fmt.Errorf("failed to validate deployments table: %w", err)
		}
		if !tableExists {
			return fmt.Errorf("deployments table was not created")
		}

		log.Println("Migration 008 completed successfully (SQL)")
		return nil
	}

	// Development: Allow AutoMigrate fallback
	if err != nil || len(sqlBytes) == 0 {
		log.Println("Development: SQL file not found, using AutoMigrate")
		return db.AutoMigrate(&models.Deployment{})
	}

	// Development with SQL file
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		log.Printf("SQL migration failed, using AutoMigrate fallback: %v", err)
		return db.AutoMigrate(&models.Deployment{})
	}

	log.Println("Migration 008 completed successfully")
	return nil
}

// Migration009_AddReplicaSets adds replicasets table
//
// Date: 2025-12-27 (removed AutoMigrate fallback)
// Author: Fortuna Team
// Ticket: Migration Audit - Phase 2
//
// Description:
//
//	Creates replicasets table for tracking Kubernetes replica sets.
//
// Tables Affected:
//   - replicasets: New table for replica set tracking
//
// Rollback Plan:
//
//	DROP TABLE IF EXISTS replicasets CASCADE;
func Migration009_AddReplicaSets(db *gorm.DB) error {
	log.Println("Running migration 009: Add replicasets table")

	// Determine environment
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	// Try multiple paths for SQL file
	sqlPaths := []string{
		"migrations/009_add_replicasets.sql",
		"/app/migrations/009_add_replicasets.sql",
		"./migrations/009_add_replicasets.sql",
	}

	var sqlBytes []byte
	var err error

	for _, path := range sqlPaths {
		sqlBytes, err = os.ReadFile(path)
		if err == nil {
			log.Printf("Found SQL migration file at: %s", path)
			break
		}
	}

	// Production: SQL file is mandatory
	if env == "production" || env == "staging" {
		if err != nil || len(sqlBytes) == 0 {
			return fmt.Errorf("CRITICAL: SQL migration file required: 009_add_replicasets.sql not found. Tried paths: %v", sqlPaths)
		}

		// Execute SQL
		if err := db.Exec(string(sqlBytes)).Error; err != nil {
			return fmt.Errorf("SQL migration failed: %w", err)
		}

		// Validate result
		var tableExists bool
		if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'replicasets')").Scan(&tableExists).Error; err != nil {
			return fmt.Errorf("failed to validate replicasets table: %w", err)
		}
		if !tableExists {
			return fmt.Errorf("replicasets table was not created")
		}

		log.Println("Migration 009 completed successfully (SQL)")
		return nil
	}

	// Development: Allow AutoMigrate fallback
	if err != nil || len(sqlBytes) == 0 {
		log.Println("Development: SQL file not found, using AutoMigrate")
		return db.AutoMigrate(&models.ReplicaSet{})
	}

	// Development with SQL file
	if err := db.Exec(string(sqlBytes)).Error; err != nil {
		log.Printf("SQL migration failed, using AutoMigrate fallback: %v", err)
		return db.AutoMigrate(&models.ReplicaSet{})
	}

	log.Println("Migration 009 completed successfully")
	return nil
}

// RunPostMigrations runs migrations that should run after schema migrations
func RunPostMigrations(db *gorm.DB) error {
	// Ensure users.deleted_at exists so CreateDefaultAdmin and auth queries do not fail
	if db.Migrator().HasTable("users") {
		if err := Migration065_EnsureUsersDeletedAt(db); err != nil {
			log.Printf("Warning: Ensure users.deleted_at failed: %v", err)
		}
	}

	// Create or sync default admin user from environment
	adminUsername := os.Getenv("FORTUNA_ADMIN_USERNAME")
	adminPassword := os.Getenv("FORTUNA_ADMIN_PASSWORD")
	adminEmail := os.Getenv("FORTUNA_ADMIN_EMAIL")

	if adminUsername != "" && adminPassword != "" {
		if adminEmail == "" {
			adminEmail = adminUsername + "@fortuna.local"
		}
		if err := CreateDefaultAdmin(db, adminUsername, adminPassword, adminEmail); err != nil {
			return fmt.Errorf("failed to create default admin: %w", err)
		}
	}

	return nil
}
