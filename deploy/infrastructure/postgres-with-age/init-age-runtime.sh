#!/bin/bash
set -e

echo "=========================================="
echo "Apache AGE Installation (Runtime)"
echo "=========================================="

# Check if AGE extension files exist
if [ -f "/usr/local/lib/postgresql/age.so" ] || [ -f "/usr/lib/postgresql/15/lib/age.so" ]; then
    echo "✅ AGE extension files found, creating extension..."
    
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        -- Create extension if files exist
        CREATE EXTENSION IF NOT EXISTS age;
        
        -- Load AGE
        LOAD 'age';
        
        -- Set search path
        SET search_path = ag_catalog, "$user", public;
        
        -- Create graph
        SELECT create_graph('fortuna_graph');
        
        -- Create vertex labels
        SELECT create_vlabel('fortuna_graph', 'ServiceAccount');
        SELECT create_vlabel('fortuna_graph', 'Pod');
        SELECT create_vlabel('fortuna_graph', 'Role');
        SELECT create_vlabel('fortuna_graph', 'ClusterRole');
        SELECT create_vlabel('fortuna_graph', 'RoleBinding');
        SELECT create_vlabel('fortuna_graph', 'ClusterRoleBinding');
        SELECT create_vlabel('fortuna_graph', 'Namespace');
        SELECT create_vlabel('fortuna_graph', 'Node');
        SELECT create_vlabel('fortuna_graph', 'Secret');
        SELECT create_vlabel('fortuna_graph', 'ConfigMap');
        
        -- Create edge labels
        SELECT create_elabel('fortuna_graph', 'USES_SERVICE_ACCOUNT');
        SELECT create_elabel('fortuna_graph', 'BINDS_TO');
        SELECT create_elabel('fortuna_graph', 'GRANTS_ROLE');
        SELECT create_elabel('fortuna_graph', 'RUNS_ON');
        SELECT create_elabel('fortuna_graph', 'IN_NAMESPACE');
        SELECT create_elabel('fortuna_graph', 'HAS_PERMISSION');
        SELECT create_elabel('fortuna_graph', 'MOUNTS_SECRET');
        SELECT create_elabel('fortuna_graph', 'MOUNTS_CONFIGMAP');
        
        -- Verify
        SELECT * FROM ag_catalog.ag_graph WHERE name = 'fortuna_graph';
        
        RAISE NOTICE 'Apache AGE initialized successfully';
EOSQL
    
    echo "✅ Apache AGE extension initialized"
else
    echo "⚠️  AGE extension files not found"
    echo "⚠️  Graph features will be disabled"
    echo "⚠️  To enable: Build custom Postgres image with AGE or install AGE extension"
    
    # Create a note in the database
    psql -v ON_ERROR_STOP=0 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
        -- Create a note table to track AGE status
        CREATE TABLE IF NOT EXISTS age_status (
            id SERIAL PRIMARY KEY,
            status TEXT,
            message TEXT,
            created_at TIMESTAMP DEFAULT NOW()
        );
        
        INSERT INTO age_status (status, message) VALUES 
        ('disabled', 'AGE extension not installed. Graph features disabled.');
EOSQL
fi


