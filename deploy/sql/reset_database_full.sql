-- Full database reset for clean redeploy (dev/test only).
-- WARNING: Drops all Fortuna tables. Core will re-run migrations on next start.
-- Usage:
--   kubectl cp deploy/sql/reset_database_full.sql fortuna/<postgres-pod>:/tmp/
--   kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -f /tmp/reset_database_full.sql
--
-- Alternatively use clear_all_cluster_data.sql to keep schema and only delete data.
-- Keep in sync with core/pkg/models/*.go (TableName functions).
-- Last updated: 2026-04-14 (attack_paths, exception_policies, sbom_processing_state)

BEGIN;

-- Drop tables in dependency order (child first). Schema will be recreated by Core migrations.

-- Malware detection (migration 113)
DROP TABLE IF EXISTS malware_matches CASCADE;
DROP TABLE IF EXISTS malware_packages CASCADE;

-- CVE / vulnerability data
DROP TABLE IF EXISTS package_vulnerabilities CASCADE;
DROP TABLE IF EXISTS cve_file_metadata CASCADE;
DROP TABLE IF EXISTS cve_matches CASCADE;
DROP TABLE IF EXISTS cves CASCADE;

-- OSV mirror (migration 037+)
DROP TABLE IF EXISTS osv_ranges CASCADE;
DROP TABLE IF EXISTS osv_packages CASCADE;
DROP TABLE IF EXISTS osv_vulnerabilities CASCADE;
DROP TABLE IF EXISTS go_module_aliases CASCADE;
DROP TABLE IF EXISTS go_module_alias CASCADE;
DROP TABLE IF EXISTS mirror_states CASCADE;
DROP TABLE IF EXISTS mirror_state CASCADE;

-- SBOM pipeline
DROP TABLE IF EXISTS sbom_match_runs CASCADE;
DROP TABLE IF EXISTS sbom_processing_state CASCADE;
DROP TABLE IF EXISTS sbom_components CASCADE;
DROP TABLE IF EXISTS sboms CASCADE;
DROP TABLE IF EXISTS image_scan_results CASCADE;
DROP TABLE IF EXISTS pod_image_scans CASCADE;

-- Risk / insights / unified scorer persisted paths
DROP TABLE IF EXISTS attack_paths CASCADE;
DROP TABLE IF EXISTS risk_scores CASCADE;
DROP TABLE IF EXISTS risk_rule_histories CASCADE;
DROP TABLE IF EXISTS risk_rules_history CASCADE;
DROP TABLE IF EXISTS risk_rules CASCADE;
DROP TABLE IF EXISTS insights CASCADE;

-- Policy engine + exception policies (RP-5)
DROP TABLE IF EXISTS policy_violations CASCADE;
DROP TABLE IF EXISTS policy_instances CASCADE;
DROP TABLE IF EXISTS exception_policies CASCADE;
DROP TABLE IF EXISTS policy_templates CASCADE;
DROP TABLE IF EXISTS policies CASCADE;
DROP TABLE IF EXISTS policy_evaluation_costs CASCADE;

-- Runtime (events, signals, behavior, incidents)
DROP TABLE IF EXISTS runtime_incidents CASCADE;
DROP TABLE IF EXISTS runtime_behavior_facts CASCADE;
DROP TABLE IF EXISTS runtime_signals CASCADE;
DROP TABLE IF EXISTS runtime_events CASCADE;
DROP TABLE IF EXISTS runtime_signal_step_mappings CASCADE;
DROP TABLE IF EXISTS asset_security_states CASCADE;
DROP TABLE IF EXISTS asset_security_state CASCADE;
DROP TABLE IF EXISTS audit_anchor CASCADE;

-- Pod detail / metrics
DROP TABLE IF EXISTS pod_capabilities CASCADE;
DROP TABLE IF EXISTS pod_attack_steps CASCADE;
DROP TABLE IF EXISTS pod_instances CASCADE;
DROP TABLE IF EXISTS pod_risk_profiles CASCADE;
DROP TABLE IF EXISTS pod_runtime_metrics CASCADE;
DROP TABLE IF EXISTS pod_processes CASCADE;
DROP TABLE IF EXISTS pod_network_connections CASCADE;
DROP TABLE IF EXISTS k8s_events CASCADE;
DROP TABLE IF EXISTS promotion_rules CASCADE;
DROP TABLE IF EXISTS capability_metadata CASCADE;

-- K8s resources
DROP TABLE IF EXISTS agents CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS events_index CASCADE;
DROP TABLE IF EXISTS deployments CASCADE;
DROP TABLE IF EXISTS replicasets CASCADE;
DROP TABLE IF EXISTS pods CASCADE;
DROP TABLE IF EXISTS service_accounts CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS cluster_roles CASCADE;
DROP TABLE IF EXISTS role_bindings CASCADE;
DROP TABLE IF EXISTS cluster_role_bindings CASCADE;
DROP TABLE IF EXISTS nodes CASCADE;
DROP TABLE IF EXISTS namespaces CASCADE;
DROP TABLE IF EXISTS clusters CASCADE;

-- Misc
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS error_logs CASCADE;
DROP TABLE IF EXISTS certificate_rotation_history CASCADE;

-- Migration bookkeeping: must clear or Core skips every migration while tables may be gone (CrashLoop).
DROP TABLE IF EXISTS schema_migrations CASCADE;

COMMIT;

-- Note: users table is kept (not dropped above) so admin credentials can survive a table-only reset.
-- After this script, restart Core; it re-runs migrations from version 1 because schema_migrations was dropped.
