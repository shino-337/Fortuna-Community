#!/bin/bash
set -e

echo "=========================================="
echo "Initializing Apache AGE Extension"
echo "=========================================="

# Create AGE extension
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Load AGE
    LOAD 'age';
    
    -- Create extension
    CREATE EXTENSION IF NOT EXISTS age;
    
    -- Set search path
    SET search_path = ag_catalog, "$user", public;
    
    -- Create graph
    SELECT create_graph('ksam_graph');
    
    -- Create vertex labels
    SELECT create_vlabel('ksam_graph', 'ServiceAccount');
    SELECT create_vlabel('ksam_graph', 'Pod');
    SELECT create_vlabel('ksam_graph', 'Role');
    SELECT create_vlabel('ksam_graph', 'ClusterRole');
    SELECT create_vlabel('ksam_graph', 'RoleBinding');
    SELECT create_vlabel('ksam_graph', 'ClusterRoleBinding');
    SELECT create_vlabel('ksam_graph', 'Namespace');
    SELECT create_vlabel('ksam_graph', 'Node');
    SELECT create_vlabel('ksam_graph', 'Secret');
    SELECT create_vlabel('ksam_graph', 'ConfigMap');
    
    -- Create edge labels
    SELECT create_elabel('ksam_graph', 'USES_SERVICE_ACCOUNT');
    SELECT create_elabel('ksam_graph', 'BINDS_TO');
    SELECT create_elabel('ksam_graph', 'GRANTS_ROLE');
    SELECT create_elabel('ksam_graph', 'RUNS_ON');
    SELECT create_elabel('ksam_graph', 'IN_NAMESPACE');
    SELECT create_elabel('ksam_graph', 'HAS_PERMISSION');
    SELECT create_elabel('ksam_graph', 'MOUNTS_SECRET');
    SELECT create_elabel('ksam_graph', 'MOUNTS_CONFIGMAP');
    
    -- Verify
    SELECT * FROM ag_catalog.ag_graph WHERE name = 'ksam_graph';
    
    RAISE NOTICE 'Apache AGE initialized successfully';
EOSQL

echo "✅ Apache AGE extension initialized"


