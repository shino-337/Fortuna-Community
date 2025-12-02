-- Migration 009: Add ReplicaSets table for Workload Inventory
-- Date: 2025-11-23
-- Purpose: Track Kubernetes ReplicaSets across clusters

CREATE TABLE IF NOT EXISTS replicasets (
    id SERIAL PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL,
    uid VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    
    -- Replica status
    replicas INTEGER DEFAULT 0,
    ready_replicas INTEGER DEFAULT 0,
    available_replicas INTEGER DEFAULT 0,
    fully_labeled_replicas INTEGER DEFAULT 0,
    
    -- Owner reference (Deployment that owns this ReplicaSet)
    owner_kind VARCHAR(50),
    owner_name VARCHAR(255),
    owner_uid VARCHAR(255),
    
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
    
    -- Foreign key
    CONSTRAINT fk_replicasets_cluster 
        FOREIGN KEY (cluster_id) 
        REFERENCES clusters(id) 
        ON DELETE CASCADE
);

-- Indexes for fast querying
CREATE INDEX IF NOT EXISTS idx_replicasets_cluster_id ON replicasets(cluster_id);
CREATE INDEX IF NOT EXISTS idx_replicasets_namespace ON replicasets(namespace);
CREATE INDEX IF NOT EXISTS idx_replicasets_name ON replicasets(name);
CREATE INDEX IF NOT EXISTS idx_replicasets_uid ON replicasets(uid);
CREATE INDEX IF NOT EXISTS idx_replicasets_cluster_namespace ON replicasets(cluster_id, namespace);
CREATE INDEX IF NOT EXISTS idx_replicasets_owner ON replicasets(owner_kind, owner_name);
CREATE INDEX IF NOT EXISTS idx_replicasets_deleted_at ON replicasets(deleted_at);

-- Comments
COMMENT ON TABLE replicasets IS 'Kubernetes ReplicaSets tracked across all clusters';
COMMENT ON COLUMN replicasets.uid IS 'Kubernetes UID of the ReplicaSet';
COMMENT ON COLUMN replicasets.owner_kind IS 'Kind of owner (e.g., Deployment)';
COMMENT ON COLUMN replicasets.owner_name IS 'Name of owner resource';
COMMENT ON COLUMN replicasets.owner_uid IS 'UID of owner resource';
COMMENT ON COLUMN replicasets.template IS 'Pod template spec (containers, volumes, etc.)';
COMMENT ON COLUMN replicasets.conditions IS 'Array of ReplicaSet conditions';

