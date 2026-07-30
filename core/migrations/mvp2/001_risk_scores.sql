-- Migration 001: Risk Scores Table
-- MVP2 Phase 1: Advanced Risk Analysis
-- Date: 2024-12-06

-- Create risk_scores table
CREATE TABLE IF NOT EXISTS risk_scores (
    id SERIAL PRIMARY KEY,
    
    -- Resource identification
    resource_type VARCHAR(50) NOT NULL,
    resource_uid VARCHAR(255) NOT NULL,
    resource_name VARCHAR(255),
    namespace VARCHAR(255),
    cluster_id VARCHAR(255) NOT NULL,
    
    -- Scores (0-100)
    total_score DECIMAL(5,2) NOT NULL DEFAULT 0.0,
    base_score DECIMAL(5,2) NOT NULL DEFAULT 0.0,
    severity_weight DECIMAL(3,2) NOT NULL DEFAULT 1.0,
    impact_multiplier DECIMAL(3,2) NOT NULL DEFAULT 1.0,
    time_decay DECIMAL(3,2) NOT NULL DEFAULT 1.0,
    
    -- Context and factors
    factors JSONB DEFAULT '{}',
    insights_count INTEGER DEFAULT 0,
    highest_severity VARCHAR(20),
    priority_level VARCHAR(10), -- P0, P1, P2, P3
    
	-- Metadata
	calculated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
	created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
	deleted_at TIMESTAMP WITH TIME ZONE, -- Soft delete support

	-- Constraints
	UNIQUE(resource_type, resource_uid, cluster_id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_risk_scores_total_score ON risk_scores(total_score DESC);
CREATE INDEX IF NOT EXISTS idx_risk_scores_resource_uid ON risk_scores(resource_uid);
CREATE INDEX IF NOT EXISTS idx_risk_scores_cluster_id ON risk_scores(cluster_id);
CREATE INDEX IF NOT EXISTS idx_risk_scores_namespace ON risk_scores(namespace);
CREATE INDEX IF NOT EXISTS idx_risk_scores_priority ON risk_scores(priority_level);
CREATE INDEX IF NOT EXISTS idx_risk_scores_calculated_at ON risk_scores(calculated_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_scores_deleted_at ON risk_scores(deleted_at); -- Soft delete index

-- Comments
COMMENT ON TABLE risk_scores IS 'Risk scores for resources calculated using MVP2 scoring algorithm';
COMMENT ON COLUMN risk_scores.total_score IS 'Final risk score (0-100), capped at 100';
COMMENT ON COLUMN risk_scores.base_score IS 'Base score from insight severity (0-25)';
COMMENT ON COLUMN risk_scores.severity_weight IS 'Weight based on vulnerability type (1.0-2.0)';
COMMENT ON COLUMN risk_scores.impact_multiplier IS 'Business impact multiplier (1.0-4.0)';
COMMENT ON COLUMN risk_scores.time_decay IS 'Time decay factor based on age (0.5-1.0)';
COMMENT ON COLUMN risk_scores.factors IS 'JSON object with detailed scoring breakdown';
COMMENT ON COLUMN risk_scores.priority_level IS 'Priority level: P0 (90-100), P1 (70-89), P2 (40-69), P3 (0-39)';

