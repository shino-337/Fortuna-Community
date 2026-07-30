-- Migration 002: Add deleted_at column to risk_scores table
-- MVP2 Phase 1: Advanced Risk Analysis
-- Date: 2024-12-07

-- Add deleted_at column for soft delete support
ALTER TABLE risk_scores 
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

-- Add index for soft delete queries
CREATE INDEX IF NOT EXISTS idx_risk_scores_deleted_at ON risk_scores(deleted_at);

