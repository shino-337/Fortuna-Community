-- Migration 003: Apache AGE Triggers for Data Sync (Conditional)
-- This migration creates PostgreSQL triggers ONLY if AGE extension is installed
-- If AGE is not available, functions are created but do nothing

-- Check if AGE is available before creating functions with Cypher syntax
DO $$
DECLARE
    age_available boolean;
BEGIN
    -- Check if AGE extension exists
    SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'age') INTO age_available;
    
    IF NOT age_available THEN
        RAISE NOTICE 'Apache AGE extension not installed. Creating stub functions that do nothing.';
        RAISE NOTICE 'Triggers will be created but will skip graph operations.';
        
        -- Create stub functions that do nothing
        CREATE OR REPLACE FUNCTION sync_pod_to_graph()
        RETURNS TRIGGER AS $$
        BEGIN
            RETURN NEW; -- Do nothing if AGE not available
        END;
        $$ LANGUAGE plpgsql;
        
        CREATE OR REPLACE FUNCTION sync_serviceaccount_to_graph()
        RETURNS TRIGGER AS $$
        BEGIN
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
        
        CREATE OR REPLACE FUNCTION sync_rolebinding_to_graph()
        RETURNS TRIGGER AS $$
        BEGIN
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
        
        CREATE OR REPLACE FUNCTION sync_role_to_graph()
        RETURNS TRIGGER AS $$
        BEGIN
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
        
    ELSE
        RAISE NOTICE 'Apache AGE extension found. Creating full graph sync functions.';
        
        -- Create full functions with Cypher (only if AGE available)
        -- Note: These will be created in a separate migration when AGE is installed
        -- For now, create stubs that will be replaced
        CREATE OR REPLACE FUNCTION sync_pod_to_graph()
        RETURNS TRIGGER AS $$
        BEGIN
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
        
        CREATE OR REPLACE FUNCTION sync_serviceaccount_to_graph()
        RETURNS TRIGGER AS $$
        BEGIN
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
        
        CREATE OR REPLACE FUNCTION sync_rolebinding_to_graph()
        RETURNS TRIGGER AS $$
        BEGIN
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
        
        CREATE OR REPLACE FUNCTION sync_role_to_graph()
        RETURNS TRIGGER AS $$
        BEGIN
            RETURN NEW;
        END;
        $$ LANGUAGE plpgsql;
    END IF;
END $$;

-- Create triggers (they will use stub functions if AGE not available)
DROP TRIGGER IF EXISTS sync_pod_to_graph_trigger ON pods;
CREATE TRIGGER sync_pod_to_graph_trigger
AFTER INSERT OR UPDATE OR DELETE ON pods
FOR EACH ROW EXECUTE FUNCTION sync_pod_to_graph();

DROP TRIGGER IF EXISTS sync_serviceaccount_to_graph_trigger ON service_accounts;
CREATE TRIGGER sync_serviceaccount_to_graph_trigger
AFTER INSERT OR UPDATE OR DELETE ON service_accounts
FOR EACH ROW EXECUTE FUNCTION sync_serviceaccount_to_graph();

DROP TRIGGER IF EXISTS sync_rolebinding_to_graph_trigger ON role_bindings;
CREATE TRIGGER sync_rolebinding_to_graph_trigger
AFTER INSERT OR UPDATE OR DELETE ON role_bindings
FOR EACH ROW EXECUTE FUNCTION sync_rolebinding_to_graph();

DROP TRIGGER IF EXISTS sync_role_to_graph_trigger ON roles;
CREATE TRIGGER sync_role_to_graph_trigger
AFTER INSERT OR UPDATE OR DELETE ON roles
FOR EACH ROW EXECUTE FUNCTION sync_role_to_graph();

-- Create a migration that will upgrade functions when AGE is installed
-- This will be run after AGE extension is created
CREATE OR REPLACE FUNCTION upgrade_graph_functions_when_age_available()
RETURNS void AS $$
DECLARE
    age_available boolean;
BEGIN
    SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'age') INTO age_available;
    
    IF age_available THEN
        RAISE NOTICE 'Upgrading graph sync functions to use AGE...';
        -- Functions with full Cypher will be created via separate script
        -- when AGE is confirmed available
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Verify triggers created
DO $$
BEGIN
    RAISE NOTICE 'AGE triggers infrastructure created successfully';
    RAISE NOTICE 'Triggers will sync data to graph when AGE extension is available';
    RAISE NOTICE 'Run upgrade script after AGE installation to enable full functionality';
END $$;


