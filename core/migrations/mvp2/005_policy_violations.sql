-- Migration 005: Policy Violations Table
-- Part of MVP2 Phase 2: Policy Engine (Revised Architecture)

CREATE TABLE IF NOT EXISTS policy_violations (
    id SERIAL PRIMARY KEY,
    
    -- Policy reference
    instance_id INTEGER NOT NULL REFERENCES policy_instances(id) ON DELETE CASCADE,
    instance_name VARCHAR(255) NOT NULL,
    template_id VARCHAR(255) NOT NULL,
    template_name VARCHAR(255) NOT NULL,
    
    -- Resource info
    resource_type VARCHAR(50) NOT NULL,
    resource_uid VARCHAR(255) NOT NULL,
    resource_name VARCHAR(255),
    namespace VARCHAR(255),
    cluster_id VARCHAR(255) NOT NULL,
    
    -- Violation details
    severity VARCHAR(20) NOT NULL,
    action VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',  -- active, resolved, dismissed
    message TEXT,
    
    -- Enforcement
    enforced_at TIMESTAMP WITH TIME ZONE,
    enforcement_result TEXT,
    
    -- Metadata
    detected_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT check_severity CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT check_action CHECK (action IN ('alert', 'block', 'audit', 'remediate')),
    CONSTRAINT check_status CHECK (status IN ('active', 'resolved', 'dismissed'))
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_violations_instance 
    ON policy_violations(instance_id);
CREATE INDEX IF NOT EXISTS idx_violations_resource 
    ON policy_violations(resource_type, resource_uid);
CREATE INDEX IF NOT EXISTS idx_violations_status 
    ON policy_violations(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_violations_cluster 
    ON policy_violations(cluster_id);
CREATE INDEX IF NOT EXISTS idx_violations_detected_at 
    ON policy_violations(detected_at DESC) WHERE deleted_at IS NULL;

-- Comments
COMMENT ON TABLE policy_violations IS 'Policy violations detected during resource evaluation';
COMMENT ON COLUMN policy_violations.instance_id IS 'Reference to policy instance that detected violation';
COMMENT ON COLUMN policy_violations.status IS 'Violation status: active (current), resolved (fixed), dismissed (ignored)';
COMMENT ON COLUMN policy_violations.enforced_at IS 'Timestamp when enforcement action was executed';
COMMENT ON COLUMN policy_violations.enforcement_result IS 'Result of enforcement action (success, error, etc.)';

