-- Migration 016: Add Policy Violations Table
-- Date: 2025-12-27
-- Description: Creates policy_violations table for tracking policy violations
-- Rollback: DROP TABLE IF EXISTS policy_violations CASCADE;

CREATE TABLE IF NOT EXISTS policy_violations (
    id SERIAL PRIMARY KEY,
    policy_instance_id INTEGER NOT NULL,
    cluster_id INTEGER NOT NULL,
    namespace VARCHAR(255),
    resource_type VARCHAR(50) NOT NULL,
    resource_name VARCHAR(255) NOT NULL,
    resource_uid VARCHAR(255),
    violation_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'medium',
    message TEXT NOT NULL,
    details JSONB,
    resolved BOOLEAN NOT NULL DEFAULT false,
    resolved_at TIMESTAMP WITH TIME ZONE,
    resolved_by INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_policy_violations_instance
        FOREIGN KEY (policy_instance_id) REFERENCES policy_instances(id) ON DELETE CASCADE,
    CONSTRAINT fk_policy_violations_cluster
        FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
    CONSTRAINT fk_policy_violations_resolved_by
        FOREIGN KEY (resolved_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_policy_violations_instance_id ON policy_violations(policy_instance_id);
CREATE INDEX IF NOT EXISTS idx_policy_violations_cluster_id ON policy_violations(cluster_id);
CREATE INDEX IF NOT EXISTS idx_policy_violations_namespace ON policy_violations(namespace) WHERE namespace IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_policy_violations_resource_type ON policy_violations(resource_type);
CREATE INDEX IF NOT EXISTS idx_policy_violations_severity ON policy_violations(severity);
CREATE INDEX IF NOT EXISTS idx_policy_violations_resolved ON policy_violations(resolved);
CREATE INDEX IF NOT EXISTS idx_policy_violations_created_at ON policy_violations(created_at);
CREATE INDEX IF NOT EXISTS idx_policy_violations_deleted_at ON policy_violations(deleted_at);

-- Add comment
COMMENT ON TABLE policy_violations IS 'Policy violations detected in the cluster';

