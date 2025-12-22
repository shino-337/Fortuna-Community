-- Migration 003: Apache AGE Triggers for Data Sync (Simple Version)
-- Creates stub functions and triggers that will work with or without AGE

-- Function to sync Pod to graph (stub - does nothing if AGE not available)
CREATE OR REPLACE FUNCTION sync_pod_to_graph()
RETURNS TRIGGER AS $$
BEGIN
    -- Stub function - will be upgraded when AGE is installed
    -- For now, just return NEW to allow normal database operations
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to sync ServiceAccount to graph (stub)
CREATE OR REPLACE FUNCTION sync_serviceaccount_to_graph()
RETURNS TRIGGER AS $$
BEGIN
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to sync RoleBinding to graph (stub)
CREATE OR REPLACE FUNCTION sync_rolebinding_to_graph()
RETURNS TRIGGER AS $$
BEGIN
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to sync Role to graph (stub)
CREATE OR REPLACE FUNCTION sync_role_to_graph()
RETURNS TRIGGER AS $$
BEGIN
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create triggers
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

-- Log success
DO $$
BEGIN
    RAISE NOTICE 'AGE triggers infrastructure created successfully';
    RAISE NOTICE 'Triggers are active but will skip graph operations until AGE is installed';
    RAISE NOTICE 'Run migration 004_age_functions_full.sql after AGE installation to enable graph sync';
END $$;


