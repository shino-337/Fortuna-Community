-- Migration 003: Policy Templates Table
-- Part of MVP2 Phase 2: Policy Engine (Revised Architecture)

CREATE TABLE IF NOT EXISTS policy_templates (
    id SERIAL PRIMARY KEY,
    
    -- Template identity
    template_id VARCHAR(255) NOT NULL UNIQUE,
    version VARCHAR(20) NOT NULL,
    
    -- Metadata
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL,
    default_severity VARCHAR(20) NOT NULL,
    
    -- Immutable logic (user CANNOT change)
    cel_expression TEXT NOT NULL,
    cel_program_cache BYTEA,  -- Compiled CEL program (binary)
    
    -- Default configuration
    default_scope JSONB,  -- Default scope settings
    default_action VARCHAR(20) DEFAULT 'alert',
    
    -- Remediation
    supports_remediation BOOLEAN DEFAULT false,
    remediation_template JSONB,  -- Patch template
    
    -- Documentation
    rationale TEXT,  -- Why this policy exists
    references TEXT[],  -- Links to docs
    examples JSONB,  -- Example violations
    
    -- Metadata
    created_by VARCHAR(255) DEFAULT 'system',
    is_system BOOLEAN DEFAULT true,  -- Cannot be deleted
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT check_category CHECK (category IN ('security', 'compliance', 'operational', 'governance')),
    CONSTRAINT check_action CHECK (default_action IN ('alert', 'block', 'audit'))
);

-- Indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_policy_templates_id_version 
    ON policy_templates(template_id, version);
CREATE INDEX IF NOT EXISTS idx_policy_templates_category 
    ON policy_templates(category);
CREATE INDEX IF NOT EXISTS idx_policy_templates_system 
    ON policy_templates(is_system);

-- Comments
COMMENT ON TABLE policy_templates IS 'Immutable policy templates with CEL expressions (built-in by Fortuna)';
COMMENT ON COLUMN policy_templates.template_id IS 'Unique identifier for template (e.g., no-root-containers)';
COMMENT ON COLUMN policy_templates.version IS 'Semantic versioning (e.g., 1.0.0)';
COMMENT ON COLUMN policy_templates.cel_expression IS 'CEL expression that defines the policy rule (immutable)';
COMMENT ON COLUMN policy_templates.cel_program_cache IS 'Pre-compiled CEL program for fast evaluation';
COMMENT ON COLUMN policy_templates.is_system IS 'System templates cannot be deleted by users';

