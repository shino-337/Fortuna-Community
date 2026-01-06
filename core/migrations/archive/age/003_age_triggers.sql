-- Migration 003: Apache AGE Triggers for Data Sync
-- This migration creates PostgreSQL triggers to sync relational data to graph

-- Note: This migration will only work if AGE extension is installed
-- If AGE is not available, triggers will be created but will fail gracefully

-- Function to sync Pod to graph
CREATE OR REPLACE FUNCTION sync_pod_to_graph()
RETURNS TRIGGER AS $$
DECLARE
    age_available boolean;
BEGIN
    -- Check if AGE is available
    SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'age') INTO age_available;
    
    IF NOT age_available THEN
        RETURN NEW; -- Skip graph sync if AGE not available
    END IF;
    
    -- Set AGE environment
    PERFORM set_config('search_path', 'ag_catalog, public', true);
    
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        -- Upsert Pod vertex using Cypher (only if AGE available)
        BEGIN
            PERFORM * FROM cypher('fortuna_graph', $$
                MERGE (p:Pod {uid: $uid})
                SET p.name = $name,
                    p.namespace = $namespace,
                    p.cluster_name = $cluster_name,
                    p.created_at = $created_at,
                    p.service_account_name = $service_account_name
            $$, jsonb_build_object(
                'uid', NEW.uid::text,
                'name', NEW.name,
                'namespace', NEW.namespace,
                'cluster_name', NEW.cluster_name,
                'created_at', NEW.created_at::text,
                'service_account_name', COALESCE(NEW.service_account_name, '')
            )) AS (result agtype);
        EXCEPTION
            WHEN OTHERS THEN
                -- Log error but don't fail transaction
                RAISE WARNING 'Failed to sync Pod to graph: %', SQLERRM;
        END;
        
        -- Create edge to ServiceAccount if exists
        IF NEW.service_account_id IS NOT NULL THEN
            BEGIN
                PERFORM * FROM cypher('fortuna_graph', $$
                    MATCH (p:Pod {uid: $pod_uid})
                    MATCH (sa:ServiceAccount {id: $sa_id})
                    MERGE (p)-[r:USES_SERVICE_ACCOUNT]->(sa)
                $$, jsonb_build_object(
                    'pod_uid', NEW.uid::text,
                    'sa_id', NEW.service_account_id::text
                )) AS (result agtype);
            EXCEPTION
                WHEN OTHERS THEN
                    RAISE WARNING 'Failed to create Pod->ServiceAccount edge: %', SQLERRM;
            END;
        END IF;
        
        -- Create edge to Namespace
        IF NEW.namespace IS NOT NULL THEN
            BEGIN
                PERFORM * FROM cypher('fortuna_graph', $$
                    MATCH (p:Pod {uid: $pod_uid})
                    MERGE (ns:Namespace {name: $namespace, cluster_name: $cluster_name})
                    MERGE (p)-[r:IN_NAMESPACE]->(ns)
                $$, jsonb_build_object(
                    'pod_uid', NEW.uid::text,
                    'namespace', NEW.namespace,
                    'cluster_name', NEW.cluster_name
                )) AS (result agtype);
            EXCEPTION
                WHEN OTHERS THEN
                    RAISE WARNING 'Failed to create Pod->Namespace edge: %', SQLERRM;
            END;
        END IF;
        
    ELSIF TG_OP = 'DELETE' THEN
        -- Delete Pod vertex and relationships
        BEGIN
            PERFORM * FROM cypher('fortuna_graph', $$
                MATCH (p:Pod {uid: $uid})
                DETACH DELETE p
            $$, jsonb_build_object('uid', OLD.uid::text)) AS (result agtype);
        EXCEPTION
            WHEN OTHERS THEN
                RAISE WARNING 'Failed to delete Pod from graph: %', SQLERRM;
        END;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger on pods table
DROP TRIGGER IF EXISTS sync_pod_to_graph_trigger ON pods;
CREATE TRIGGER sync_pod_to_graph_trigger
AFTER INSERT OR UPDATE OR DELETE ON pods
FOR EACH ROW EXECUTE FUNCTION sync_pod_to_graph();

-- Function to sync ServiceAccount to graph
CREATE OR REPLACE FUNCTION sync_serviceaccount_to_graph()
RETURNS TRIGGER AS $$
DECLARE
    age_available boolean;
BEGIN
    SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'age') INTO age_available;
    IF NOT age_available THEN
        RETURN NEW;
    END IF;
    
    PERFORM set_config('search_path', 'ag_catalog, public', true);
    
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        BEGIN
            PERFORM * FROM cypher('fortuna_graph', $$
                MERGE (sa:ServiceAccount {id: $id})
                SET sa.name = $name,
                    sa.namespace = $namespace,
                    sa.cluster_name = $cluster_name,
                    sa.created_at = $created_at
            $$, jsonb_build_object(
                'id', NEW.id::text,
                'name', NEW.name,
                'namespace', NEW.namespace,
                'cluster_name', NEW.cluster_name,
                'created_at', NEW.created_at::text
            )) AS (result agtype);
        EXCEPTION
            WHEN OTHERS THEN
                RAISE WARNING 'Failed to sync ServiceAccount to graph: %', SQLERRM;
        END;
        
        -- Create edge to Namespace
        IF NEW.namespace IS NOT NULL THEN
            BEGIN
                PERFORM * FROM cypher('fortuna_graph', $$
                    MATCH (sa:ServiceAccount {id: $sa_id})
                    MERGE (ns:Namespace {name: $namespace, cluster_name: $cluster_name})
                    MERGE (sa)-[r:IN_NAMESPACE]->(ns)
                $$, jsonb_build_object(
                    'sa_id', NEW.id::text,
                    'namespace', NEW.namespace,
                    'cluster_name', NEW.cluster_name
                )) AS (result agtype);
            EXCEPTION
                WHEN OTHERS THEN
                    RAISE WARNING 'Failed to create ServiceAccount->Namespace edge: %', SQLERRM;
            END;
        END IF;
        
    ELSIF TG_OP = 'DELETE' THEN
        BEGIN
            PERFORM * FROM cypher('fortuna_graph', $$
                MATCH (sa:ServiceAccount {id: $id})
                DETACH DELETE sa
            $$, jsonb_build_object('id', OLD.id::text)) AS (result agtype);
        EXCEPTION
            WHEN OTHERS THEN
                RAISE WARNING 'Failed to delete ServiceAccount from graph: %', SQLERRM;
        END;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger on service_accounts table
DROP TRIGGER IF EXISTS sync_serviceaccount_to_graph_trigger ON service_accounts;
CREATE TRIGGER sync_serviceaccount_to_graph_trigger
AFTER INSERT OR UPDATE OR DELETE ON service_accounts
FOR EACH ROW EXECUTE FUNCTION sync_serviceaccount_to_graph();

-- Function to sync RoleBinding to graph
CREATE OR REPLACE FUNCTION sync_rolebinding_to_graph()
RETURNS TRIGGER AS $$
DECLARE
    age_available boolean;
BEGIN
    SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'age') INTO age_available;
    IF NOT age_available THEN
        RETURN NEW;
    END IF;
    
    PERFORM set_config('search_path', 'ag_catalog, public', true);
    
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        -- Create RoleBinding vertex
        BEGIN
            PERFORM * FROM cypher('fortuna_graph', $$
                MERGE (rb:RoleBinding {uid: $uid})
                SET rb.name = $name,
                    rb.namespace = $namespace,
                    rb.cluster_name = $cluster_name
            $$, jsonb_build_object(
                'uid', NEW.uid::text,
                'name', NEW.name,
                'namespace', NEW.namespace,
                'cluster_name', NEW.cluster_name
            )) AS (result agtype);
        EXCEPTION
            WHEN OTHERS THEN
                RAISE WARNING 'Failed to sync RoleBinding to graph: %', SQLERRM;
        END;
        
        -- Create edges: ServiceAccount -> RoleBinding -> Role
        IF NEW.service_account_id IS NOT NULL THEN
            BEGIN
                PERFORM * FROM cypher('fortuna_graph', $$
                    MATCH (sa:ServiceAccount {id: $sa_id})
                    MATCH (rb:RoleBinding {uid: $rb_uid})
                    MERGE (sa)-[:BINDS_TO]->(rb)
                $$, jsonb_build_object(
                    'sa_id', NEW.service_account_id::text,
                    'rb_uid', NEW.uid::text
                )) AS (result agtype);
            EXCEPTION
                WHEN OTHERS THEN
                    RAISE WARNING 'Failed to create ServiceAccount->RoleBinding edge: %', SQLERRM;
            END;
        END IF;
        
        IF NEW.role_id IS NOT NULL THEN
            BEGIN
                PERFORM * FROM cypher('fortuna_graph', $$
                    MATCH (rb:RoleBinding {uid: $rb_uid})
                    MATCH (r:Role {id: $role_id})
                    MERGE (rb)-[:GRANTS_ROLE]->(r)
                $$, jsonb_build_object(
                    'rb_uid', NEW.uid::text,
                    'role_id', NEW.role_id::text
                )) AS (result agtype);
            EXCEPTION
                WHEN OTHERS THEN
                    RAISE WARNING 'Failed to create RoleBinding->Role edge: %', SQLERRM;
            END;
        END IF;
        
    ELSIF TG_OP = 'DELETE' THEN
        BEGIN
            PERFORM * FROM cypher('fortuna_graph', $$
                MATCH (rb:RoleBinding {uid: $uid})
                DETACH DELETE rb
            $$, jsonb_build_object('uid', OLD.uid::text)) AS (result agtype);
        EXCEPTION
            WHEN OTHERS THEN
                RAISE WARNING 'Failed to delete RoleBinding from graph: %', SQLERRM;
        END;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger on role_bindings table
DROP TRIGGER IF EXISTS sync_rolebinding_to_graph_trigger ON role_bindings;
CREATE TRIGGER sync_rolebinding_to_graph_trigger
AFTER INSERT OR UPDATE OR DELETE ON role_bindings
FOR EACH ROW EXECUTE FUNCTION sync_rolebinding_to_graph();

-- Function to sync Role to graph
CREATE OR REPLACE FUNCTION sync_role_to_graph()
RETURNS TRIGGER AS $$
DECLARE
    age_available boolean;
BEGIN
    SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'age') INTO age_available;
    IF NOT age_available THEN
        RETURN NEW;
    END IF;
    
    PERFORM set_config('search_path', 'ag_catalog, public', true);
    
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        BEGIN
            PERFORM * FROM cypher('fortuna_graph', $$
                MERGE (r:Role {id: $id})
                SET r.name = $name,
                    r.namespace = $namespace,
                    r.cluster_name = $cluster_name,
                    r.rules = $rules
            $$, jsonb_build_object(
                'id', NEW.id::text,
                'name', NEW.name,
                'namespace', NEW.namespace,
                'cluster_name', NEW.cluster_name,
                'rules', NEW.rules::text
            )) AS (result agtype);
        EXCEPTION
            WHEN OTHERS THEN
                RAISE WARNING 'Failed to sync Role to graph: %', SQLERRM;
        END;
        
    ELSIF TG_OP = 'DELETE' THEN
        BEGIN
            PERFORM * FROM cypher('fortuna_graph', $$
                MATCH (r:Role {id: $id})
                DETACH DELETE r
            $$, jsonb_build_object('id', OLD.id::text)) AS (result agtype);
        EXCEPTION
            WHEN OTHERS THEN
                RAISE WARNING 'Failed to delete Role from graph: %', SQLERRM;
        END;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger on roles table
DROP TRIGGER IF EXISTS sync_role_to_graph_trigger ON roles;
CREATE TRIGGER sync_role_to_graph_trigger
AFTER INSERT OR UPDATE OR DELETE ON roles
FOR EACH ROW EXECUTE FUNCTION sync_role_to_graph();

-- Verify triggers created
DO $$
BEGIN
    RAISE NOTICE 'AGE triggers created successfully';
    RAISE NOTICE 'Triggers will sync data to graph when AGE extension is available';
END $$;

