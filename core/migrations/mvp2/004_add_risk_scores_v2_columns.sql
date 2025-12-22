-- Migration 004: Add V2 scoring columns to risk_scores table
-- MVP2 Phase 1.2: Risk Scoring V2 Enhancement
-- Date: 2025-12-11

-- Add V2 scoring columns
ALTER TABLE risk_scores 
ADD COLUMN IF NOT EXISTS exploitability_score DECIMAL(5,2) DEFAULT 0.0,
ADD COLUMN IF NOT EXISTS business_impact_score DECIMAL(5,2) DEFAULT 0.0,
ADD COLUMN IF NOT EXISTS scorer_version VARCHAR(10) DEFAULT 'v1';

-- Add comments
COMMENT ON COLUMN risk_scores.exploitability_score IS 'Exploitability score (0-30) for V2 scoring';
COMMENT ON COLUMN risk_scores.business_impact_score IS 'Business impact score (0-30) for V2 scoring';
COMMENT ON COLUMN risk_scores.scorer_version IS 'Scorer version: v1 (old) or v2 (new)';

-- Update priority_level to support P4
-- Note: This is a data migration, actual constraint update handled in code


