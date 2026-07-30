-- Clear all cluster-related data so Agent can re-sync with correct cluster id/name.
-- Run on dev/test only. Keeps schema intact (no DROP) and skips tables that do not
-- exist in the current migration state.
-- Usage:
--   kubectl cp deploy/e2e/clear_all_cluster_data.sql fortuna/<postgres-pod>:/tmp/
--   kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -f /tmp/clear_all_cluster_data.sql

DO $$
DECLARE
  tables text[] := ARRAY[
    -- Malware detection matches tied to pods/SBOMs.
    'malware_matches',

    -- Tables that reference pods by pod_uid and runtime/security state.
    'pod_capabilities',
    'pod_attack_steps',
    'attack_paths',
    'runtime_incidents',
    'runtime_behavior_facts',
    'runtime_signals',
    'runtime_events',
    'asset_security_states',
    'asset_security_state',
    'pod_instances',
    'pod_risk_profiles',
    'pod_runtime_metrics',
    'pod_processes',
    'pod_network_connections',

    -- SBOM / CVE matches tied to pods.
    'sbom_match_runs',
    'sbom_processing_state',
    'cve_matches',
    'sbom_components',
    'sboms',
    'image_scan_results',
    'pod_image_scans',

    -- Tables with cluster_id or cluster-scoped data.
    'risk_scores',
    'risk_rule_histories',
    'insights',
    'agents',
    'audit_logs',
    'events_index',
    'k8s_events',
    'deployments',
    'replicasets',
    'pods',
    'service_accounts',
    'roles',
    'cluster_roles',
    'role_bindings',
    'cluster_role_bindings',
    'nodes',
    'namespaces',
    'clusters',

    -- Policy violations and exception policies.
    'policy_violations',
    'policy_instances',
    'exception_policies'
  ];
  t text;
BEGIN
  FOREACH t IN ARRAY tables LOOP
    IF to_regclass('public.' || t) IS NOT NULL THEN
      EXECUTE format('TRUNCATE TABLE public.%I RESTART IDENTITY CASCADE', t);
      RAISE NOTICE 'cleared table %', t;
    ELSE
      RAISE NOTICE 'skipped missing table %', t;
    END IF;
  END LOOP;
END $$;
