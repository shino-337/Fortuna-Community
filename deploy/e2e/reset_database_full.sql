-- Full database reset for clean redeploy (dev/test only).
-- WARNING: Drops all Fortuna tables. Core will re-run migrations on next start.
-- Usage:
--   kubectl cp deploy/e2e/reset_database_full.sql fortuna/<postgres-pod>:/tmp/
--   kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -f /tmp/reset_database_full.sql
--
-- Alternatively use clear_all_cluster_data.sql to keep schema and only delete data.

BEGIN;

-- Drop tables in dependency order (child first). Schema will be recreated by Core migrations.
DROP TABLE IF EXISTS package_vulnerabilities CASCADE;
DROP TABLE IF EXISTS cves CASCADE;
DROP TABLE IF EXISTS risk_scores CASCADE;
DROP TABLE IF EXISTS insights CASCADE;
DROP TABLE IF EXISTS pod_capabilities CASCADE;
DROP TABLE IF EXISTS pod_attack_steps CASCADE;
DROP TABLE IF EXISTS runtime_signals CASCADE;
DROP TABLE IF EXISTS runtime_events CASCADE;
DROP TABLE IF EXISTS pod_instances CASCADE;
DROP TABLE IF EXISTS pod_risk_profiles CASCADE;
DROP TABLE IF EXISTS agents CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS events_index CASCADE;
DROP TABLE IF EXISTS pod_image_scans CASCADE;
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
-- risk_rules (075); pod detail / metrics (071); promotion/capability (048,047); cve_file_metadata (027); cve_matches (mvp2)
DROP TABLE IF EXISTS risk_rules CASCADE;
DROP TABLE IF EXISTS pod_runtime_metrics CASCADE;
DROP TABLE IF EXISTS pod_processes CASCADE;
DROP TABLE IF EXISTS pod_network_connections CASCADE;
DROP TABLE IF EXISTS k8s_events CASCADE;
DROP TABLE IF EXISTS promotion_rules CASCADE;
DROP TABLE IF EXISTS capability_metadata CASCADE;
DROP TABLE IF EXISTS cve_file_metadata CASCADE;
DROP TABLE IF EXISTS cve_matches CASCADE;
DROP TABLE IF EXISTS sbom_components CASCADE;
DROP TABLE IF EXISTS sboms CASCADE;
DROP TABLE IF EXISTS image_scan_results CASCADE;
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS error_logs CASCADE;
DROP TABLE IF EXISTS policy_evaluation_costs CASCADE;
DROP TABLE IF EXISTS certificate_rotation_history CASCADE;

COMMIT;

-- Note: users, migrations version table (if any) may be kept or dropped per need.
-- Core migrations typically use CREATE TABLE IF NOT EXISTS, so re-starting Core will recreate tables.
