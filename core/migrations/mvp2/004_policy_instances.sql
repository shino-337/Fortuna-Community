-- Migration 004: Policy Instances Table
-- Part of MVP2 Phase 2: Policy Engine (Revised Architecture)

CREATE TABLE IF NOT EXISTS policy_instances (
    id SERIAL PRIMARY KEY,
    
    -- Template reference (immutable after creation)
    template_id VARCHAR(255) NOT NULL,
    template_version VARCHAR(20) NOT NULL,
    
    -- Instance identity
    instance_name VARCHAR(255) NOT NULL UNIQUE,  -- User-defined name
    description TEXT,
    
    -- Status
    enabled BOOLEAN DEFAULT true,
    
    -- Scope configuration (USER CONFIGURABLE)
    clusters TEXT[],  -- ["prod-*", "staging-*"]
    namespaces TEXT[],  -- ["default", "production"]
    resource_types TEXT[],  -- ["Pod", "Deployment"]
    label_selectors JSONB,  -- {"env": "production"}
    
    -- Action override (USER CONFIGURABLE)
    action VARCHAR(20),  -- Override default action
    severity VARCHAR(20),  -- Override default severity
    
    -- Message override (USER CONFIGURABLE)
    custom_message TEXT,  -- Override default message
    
    -- Remediation settings (USER CONFIGURABLE)
    auto_remediate BOOLEAN DEFAULT false,  -- Enable auto-remediation
    remediation_dry_run BOOLEAN DEFAULT true,  -- Dry-run mode
    
    -- Exemptions
    exemptions JSONB,  -- Array of exemption rules
    
    -- Metadata
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Foreign key
    FOREIGN KEY (template_id, template_version) 
        REFERENCES policy_templates(template_id, version) ON DELETE RESTRICT,
    
    CONSTRAINT check_action CHECK (action IN ('alert', 'block', 'audit', 'remediate')),
    CONSTRAINT check_severity CHECK (severity IN ('low', 'medium', 'high', 'critical'))
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_policy_instances_template 
    ON policy_instances(template_id, template_version);
CREATE INDEX IF NOT EXISTS idx_policy_instances_enabled 
    ON policy_instances(enabled) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_policy_instances_deleted_at 
    ON policy_instances(deleted_at);

-- Comments
COMMENT ON TABLE policy_instances IS 'User-configurable policy instances (scope, action, severity only)';
COMMENT ON COLUMN policy_instances.instance_name IS 'User-defined unique name for this instance';
COMMENT ON COLUMN policy_instances.template_id IS 'Reference to immutable policy template';
COMMENT ON COLUMN policy_instances.clusters IS 'Array of cluster name patterns (e.g., ["prod-*"])';
COMMENT ON COLUMN policy_instances.namespaces IS 'Array of namespace names to apply policy';
COMMENT ON COLUMN policy_instances.resource_types IS 'Array of resource types (e.g., ["Pod", "Deployment"])';
COMMENT ON COLUMN policy_instances.label_selectors IS 'JSONB object for label-based filtering';

