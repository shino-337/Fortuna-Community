-- Migration 015: Add Policy Instances Table
-- Date: 2025-12-27
-- Description: Creates policy_instances table for policy enforcement
-- Rollback: DROP TABLE IF EXISTS policy_instances CASCADE;

CREATE TABLE IF NOT EXISTS policy_instances (
    id SERIAL PRIMARY KEY,
    policy_template_id INTEGER NOT NULL,
    cluster_id INTEGER NOT NULL,
    namespace VARCHAR(255),
    resource_type VARCHAR(50) NOT NULL,
    resource_name VARCHAR(255) NOT NULL,
    resource_uid VARCHAR(255),
    enabled BOOLEAN NOT NULL DEFAULT true,
    parameters JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_policy_instances_template
        FOREIGN KEY (policy_template_id) REFERENCES policy_templates(id) ON DELETE CASCADE,
    CONSTRAINT fk_policy_instances_cluster
        FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_policy_instances_template_id ON policy_instances(policy_template_id);
CREATE INDEX IF NOT EXISTS idx_policy_instances_cluster_id ON policy_instances(cluster_id);
CREATE INDEX IF NOT EXISTS idx_policy_instances_namespace ON policy_instances(namespace) WHERE namespace IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_policy_instances_resource_type ON policy_instances(resource_type);
CREATE INDEX IF NOT EXISTS idx_policy_instances_enabled ON policy_instances(enabled);
CREATE INDEX IF NOT EXISTS idx_policy_instances_deleted_at ON policy_instances(deleted_at);

-- Add comment
COMMENT ON TABLE policy_instances IS 'Active policy instances applied to resources';

