-- KSAM Initial Database Schema
-- PostgreSQL with TimescaleDB and Apache AGE extensions

-- Enable required extensions
-- Note: TimescaleDB and Apache AGE need to be installed in PostgreSQL
-- For now, we'll create tables without extensions and add them later
-- CREATE EXTENSION IF NOT EXISTS timescaledb;
-- CREATE EXTENSION IF NOT EXISTS age;

-- ============================================
-- Core Tables
-- ============================================

-- Clusters
CREATE TABLE IF NOT EXISTS clusters (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    region VARCHAR(255),
    endpoint VARCHAR(500),
    status VARCHAR(50) DEFAULT 'active',
    last_sync TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Nodes
CREATE TABLE IF NOT EXISTS nodes (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    node_name VARCHAR(255) NOT NULL,
    ip VARCHAR(50),
    kubelet_version VARCHAR(100),
    last_seen TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cluster_id, node_name)
);

CREATE INDEX IF NOT EXISTS idx_nodes_cluster ON nodes(cluster_id);
CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(node_name);

-- Namespaces
CREATE TABLE IF NOT EXISTS namespaces (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    labels JSONB,
    annotations JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cluster_id, name)
);

CREATE INDEX IF NOT EXISTS idx_namespaces_cluster ON namespaces(cluster_id);
CREATE INDEX IF NOT EXISTS idx_namespaces_name ON namespaces(name);

-- Service Accounts
CREATE TABLE IF NOT EXISTS service_accounts (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    uid VARCHAR(255) NOT NULL,
    labels JSONB,
    secrets JSONB,
    linked_pods JSONB,
    last_used TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cluster_id, namespace, name)
);

CREATE INDEX IF NOT EXISTS idx_service_accounts_cluster ON service_accounts(cluster_id);
CREATE INDEX IF NOT EXISTS idx_service_accounts_namespace ON service_accounts(namespace);
CREATE INDEX IF NOT EXISTS idx_service_accounts_uid ON service_accounts(uid);

-- Pods
CREATE TABLE IF NOT EXISTS pods (
    id SERIAL PRIMARY KEY,
    uid VARCHAR(255) NOT NULL UNIQUE,
    cluster_id VARCHAR(255) NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    namespace_id INTEGER REFERENCES namespaces(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    service_account VARCHAR(255),
    containers JSONB,
    image_digests JSONB,
    node_id INTEGER REFERENCES nodes(id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP
);

-- Fix existing pods table if it was created with uid as primary key
DO $$ 
BEGIN
    -- Check if pods table exists but doesn't have id column
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'pods')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'pods' AND column_name = 'id') THEN
        -- Drop existing primary key constraint if uid is the primary key
        IF EXISTS (
            SELECT 1 FROM information_schema.table_constraints 
            WHERE table_name = 'pods' 
            AND constraint_type = 'PRIMARY KEY'
            AND constraint_name IN (
                SELECT constraint_name FROM information_schema.key_column_usage 
                WHERE table_name = 'pods' AND column_name = 'uid'
            )
        ) THEN
            ALTER TABLE pods DROP CONSTRAINT pods_pkey;
        END IF;
        
        -- Add id column as primary key
        ALTER TABLE pods ADD COLUMN id SERIAL PRIMARY KEY;
        
        -- Make uid unique (not primary key)
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.table_constraints 
            WHERE table_name = 'pods' 
            AND constraint_type = 'UNIQUE'
            AND constraint_name = 'pods_uid_unique'
        ) THEN
            ALTER TABLE pods ADD CONSTRAINT pods_uid_unique UNIQUE (uid);
        END IF;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_pods_cluster ON pods(cluster_id);
CREATE INDEX IF NOT EXISTS idx_pods_namespace ON pods(namespace);
CREATE INDEX IF NOT EXISTS idx_pods_service_account ON pods(service_account);
CREATE INDEX IF NOT EXISTS idx_pods_node ON pods(node_id);
CREATE INDEX IF NOT EXISTS idx_pods_uid ON pods(uid);

-- Roles
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    uid VARCHAR(255) NOT NULL,
    rules JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cluster_id, namespace, name)
);

CREATE INDEX IF NOT EXISTS idx_roles_cluster ON roles(cluster_id);
CREATE INDEX IF NOT EXISTS idx_roles_namespace ON roles(namespace);

-- Cluster Roles
CREATE TABLE IF NOT EXISTS cluster_roles (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    uid VARCHAR(255) NOT NULL,
    rules JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cluster_id, name)
);

CREATE INDEX IF NOT EXISTS idx_cluster_roles_cluster ON cluster_roles(cluster_id);

-- Role Bindings
CREATE TABLE IF NOT EXISTS role_bindings (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    uid VARCHAR(255) NOT NULL,
    role_ref JSONB,
    subjects JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cluster_id, namespace, name)
);

CREATE INDEX IF NOT EXISTS idx_role_bindings_cluster ON role_bindings(cluster_id);
CREATE INDEX IF NOT EXISTS idx_role_bindings_namespace ON role_bindings(namespace);

-- Cluster Role Bindings
CREATE TABLE IF NOT EXISTS cluster_role_bindings (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    uid VARCHAR(255) NOT NULL,
    role_ref JSONB,
    subjects JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cluster_id, name)
);

CREATE INDEX IF NOT EXISTS idx_cluster_role_bindings_cluster ON cluster_role_bindings(cluster_id);

-- ============================================
-- Insights & Policies
-- ============================================

-- Insights
CREATE TABLE IF NOT EXISTS insights (
    id SERIAL PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    affected_resources JSONB,
    severity VARCHAR(50) NOT NULL,
    recommended_action TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_insights_type ON insights(type);
CREATE INDEX IF NOT EXISTS idx_insights_severity ON insights(severity);
CREATE INDEX IF NOT EXISTS idx_insights_created ON insights(created_at);

-- Policies
CREATE TABLE IF NOT EXISTS policies (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    policy_yaml TEXT NOT NULL,
    generated_by VARCHAR(100),
    status VARCHAR(50) DEFAULT 'pending',
    created_by VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_policies_type ON policies(type);
CREATE INDEX IF NOT EXISTS idx_policies_status ON policies(status);

-- ============================================
-- Events (TimescaleDB)
-- ============================================

-- Events Index (Hypertable for time-series)
CREATE TABLE IF NOT EXISTS events_index (
    event_id VARCHAR(255) PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL,
    node_id INTEGER REFERENCES nodes(id) ON DELETE SET NULL,
    pod_uid VARCHAR(255),
    ts TIMESTAMP NOT NULL,
    summary JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Convert to hypertable for time-series optimization
-- Note: Requires TimescaleDB extension
-- SELECT create_hypertable('events_index', 'ts', if_not_exists => TRUE);
-- For now, use regular table with index

CREATE INDEX IF NOT EXISTS idx_events_cluster ON events_index(cluster_id);
CREATE INDEX IF NOT EXISTS idx_events_node ON events_index(node_id);
CREATE INDEX IF NOT EXISTS idx_events_pod ON events_index(pod_uid);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events_index(ts DESC);

-- ============================================
-- Users & Audit
-- ============================================

-- Users
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    active BOOLEAN DEFAULT TRUE,
    last_login TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Audit Logs
CREATE TABLE IF NOT EXISTS audit_logs (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255),
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255),
    details JSONB,
    username VARCHAR(255),
    ip VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_cluster ON audit_logs(cluster_id);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at DESC);

-- ============================================
-- Apache AGE Graph Schema
-- ============================================

-- Note: Apache AGE graph creation requires AGE extension
-- Will be created later when extension is installed
-- SELECT create_graph('fortuna_graph');

