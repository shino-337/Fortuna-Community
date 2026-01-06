-- Migration 002: Install Apache AGE Extension
-- This migration installs Apache AGE extension for graph database functionality

-- Enable AGE extension
-- Note: AGE extension needs to be installed in PostgreSQL first
-- For development, we'll create the extension if it exists
-- In production, AGE should be pre-installed in PostgreSQL image

-- Check if AGE is available and create extension
DO $$
BEGIN
    -- Try to create AGE extension
    -- This will fail gracefully if AGE is not installed
    CREATE EXTENSION IF NOT EXISTS age;
    
    -- Load AGE
    LOAD 'age';
    
    -- Set search path to include AGE catalog
    SET search_path = ag_catalog, "$user", public;
    
    RAISE NOTICE 'Apache AGE extension installed successfully';
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'Apache AGE extension not available. Graph features will be disabled. Error: %', SQLERRM;
END $$;

-- Create graph schema if AGE is available
DO $$
BEGIN
    -- Create the main graph
    PERFORM create_graph('fortuna_graph');
    RAISE NOTICE 'Graph fortuna_graph created successfully';
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'Could not create graph. AGE may not be installed. Error: %', SQLERRM;
END $$;

-- Create vertex labels
DO $$
BEGIN
    -- ServiceAccount vertex
    PERFORM create_vlabel('fortuna_graph', 'ServiceAccount');
    
    -- Pod vertex
    PERFORM create_vlabel('fortuna_graph', 'Pod');
    
    -- Role vertex
    PERFORM create_vlabel('fortuna_graph', 'Role');
    
    -- ClusterRole vertex
    PERFORM create_vlabel('fortuna_graph', 'ClusterRole');
    
    -- Namespace vertex
    PERFORM create_vlabel('fortuna_graph', 'Namespace');
    
    -- Cluster vertex
    PERFORM create_vlabel('fortuna_graph', 'Cluster');
    
    RAISE NOTICE 'Vertex labels created successfully';
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'Could not create vertex labels. Error: %', SQLERRM;
END $$;

-- Create edge labels
DO $$
BEGIN
    -- USES: ServiceAccount uses Role/ClusterRole
    PERFORM create_elabel('fortuna_graph', 'USES');
    
    -- MOUNTS: Pod mounts ServiceAccount
    PERFORM create_elabel('fortuna_graph', 'MOUNTS');
    
    -- BELONGS_TO: Resource belongs to Namespace
    PERFORM create_elabel('fortuna_graph', 'BELONGS_TO');
    
    -- IN_CLUSTER: Resource in Cluster
    PERFORM create_elabel('fortuna_graph', 'IN_CLUSTER');
    
    -- GRANTS: RoleBinding/ClusterRoleBinding grants permissions
    PERFORM create_elabel('fortuna_graph', 'GRANTS');
    
    -- LINKS_TO: General relationship
    PERFORM create_elabel('fortuna_graph', 'LINKS_TO');
    
    RAISE NOTICE 'Edge labels created successfully';
EXCEPTION
    WHEN OTHERS THEN
        RAISE NOTICE 'Could not create edge labels. Error: %', SQLERRM;
END $$;

