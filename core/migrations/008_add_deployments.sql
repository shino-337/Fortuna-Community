-- Migration 008: Add Deployments table for Workload Inventory
-- Date: 2025-11-23
-- Purpose: Track Kubernetes Deployments across clusters

CREATE TABLE IF NOT EXISTS deployments (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL,
    uid VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    
    -- Replica status
    replicas INTEGER DEFAULT 0,
    ready_replicas INTEGER DEFAULT 0,
    available_replicas INTEGER DEFAULT 0,
    unavailable_replicas INTEGER DEFAULT 0,
    updated_replicas INTEGER DEFAULT 0,
    
    -- Deployment strategy
    strategy VARCHAR(50), -- RollingUpdate, Recreate
    
    -- Metadata (stored as JSONB for flexibility)
    labels JSONB DEFAULT '{}',
    annotations JSONB DEFAULT '{}',
    selector JSONB DEFAULT '{}',
    
    -- Pod template (for tracking image, resources, etc.)
    template JSONB DEFAULT '{}',
    
    -- Status conditions
    conditions JSONB DEFAULT '[]',
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP,
    
    -- Indexes for performance
    CONSTRAINT fk_deployments_cluster FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
);

-- Indexes for fast querying
CREATE INDEX IF NOT EXISTS idx_deployments_cluster_id ON deployments(cluster_id);
CREATE INDEX IF NOT EXISTS idx_deployments_namespace ON deployments(namespace);
CREATE INDEX IF NOT EXISTS idx_deployments_name ON deployments(name);
CREATE INDEX IF NOT EXISTS idx_deployments_uid ON deployments(uid);
CREATE INDEX IF NOT EXISTS idx_deployments_cluster_namespace ON deployments(cluster_id, namespace);
CREATE INDEX IF NOT EXISTS idx_deployments_deleted_at ON deployments(deleted_at);

-- Comments
COMMENT ON TABLE deployments IS 'Kubernetes Deployments tracked across all clusters';
COMMENT ON COLUMN deployments.uid IS 'Kubernetes UID of the Deployment';
COMMENT ON COLUMN deployments.template IS 'Pod template spec (containers, volumes, etc.)';
COMMENT ON COLUMN deployments.conditions IS 'Array of deployment conditions (Available, Progressing, etc.)';

