-- Migration 004: Full AGE Functions (Run AFTER AGE extension is installed)
-- This migration upgrades stub functions to full Cypher-based functions
-- Only run this after AGE extension is successfully installed

-- Verify AGE is available
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'age') THEN
        RAISE EXCEPTION 'Apache AGE extension not installed. Please install AGE first.';
    END IF;
    
    RAISE NOTICE 'Upgrading graph sync functions to use Apache AGE...';
END $$;

-- Function to sync Pod to graph (FULL VERSION with Cypher)
CREATE OR REPLACE FUNCTION sync_pod_to_graph()
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
            PERFORM * FROM cypher('ksam_graph', $$
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
                RAISE WARNING 'Failed to sync Pod to graph: %', SQLERRM;
        END;
        
        IF NEW.service_account_id IS NOT NULL THEN
            BEGIN
                PERFORM * FROM cypher('ksam_graph', $$
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
        
        IF NEW.namespace IS NOT NULL THEN
            BEGIN
                PERFORM * FROM cypher('ksam_graph', $$
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
        BEGIN
            PERFORM * FROM cypher('ksam_graph', $$
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

-- Function to sync ServiceAccount to graph (FULL VERSION)
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
            PERFORM * FROM cypher('ksam_graph', $$
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
        
        IF NEW.namespace IS NOT NULL THEN
            BEGIN
                PERFORM * FROM cypher('ksam_graph', $$
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
            PERFORM * FROM cypher('ksam_graph', $$
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

-- Function to sync RoleBinding to graph (FULL VERSION)
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
        BEGIN
            PERFORM * FROM cypher('ksam_graph', $$
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
        
        IF NEW.service_account_id IS NOT NULL THEN
            BEGIN
                PERFORM * FROM cypher('ksam_graph', $$
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
                PERFORM * FROM cypher('ksam_graph', $$
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
            PERFORM * FROM cypher('ksam_graph', $$
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

-- Function to sync Role to graph (FULL VERSION)
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
            PERFORM * FROM cypher('ksam_graph', $$
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
            PERFORM * FROM cypher('ksam_graph', $$
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

DO $$
BEGIN
    RAISE NOTICE 'Graph sync functions upgraded to use Apache AGE';
    RAISE NOTICE 'Triggers will now sync data to graph on INSERT/UPDATE/DELETE';
END $$;


