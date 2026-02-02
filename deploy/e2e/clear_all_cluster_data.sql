-- Clear all cluster-related data so Agent can re-sync with correct cluster id/name (e.g. kubernetes).
-- Run on dev/test only. Order: child tables first (pod_capabilities, etc.), then tables with cluster_id, then clusters.
-- Usage: kubectl cp deploy/e2e/clear_all_cluster_data.sql fortuna/<postgres-pod>:/tmp/ && kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -f /tmp/clear_all_cluster_data.sql

BEGIN;

-- Tables that reference pods (by pod_uid) but may not have cluster_id
DELETE FROM pod_capabilities;
DELETE FROM pod_attack_steps;
DELETE FROM runtime_signals;
DELETE FROM runtime_events;
DELETE FROM pod_instances;
DELETE FROM pod_risk_profiles;

-- Optional: clear SBOM/image data tied to pods (uncomment if you want full reset)
-- DELETE FROM sbom_components;
-- DELETE FROM sboms;
-- DELETE FROM image_scan_results;
-- DELETE FROM pod_image_scans;

-- Tables with cluster_id or cluster-scoped data
DELETE FROM risk_scores;
DELETE FROM insights;
DELETE FROM agents;
DELETE FROM audit_logs;
DELETE FROM events_index;
DELETE FROM pod_image_scans;
DELETE FROM deployments;
DELETE FROM replicasets;
DELETE FROM pods;
DELETE FROM service_accounts;
DELETE FROM roles;
DELETE FROM cluster_roles;
DELETE FROM role_bindings;
DELETE FROM cluster_role_bindings;
DELETE FROM nodes;
DELETE FROM namespaces;
DELETE FROM clusters;

COMMIT;
