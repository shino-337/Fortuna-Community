-- Migration: Add soft delete and status to insights table
-- Date: 2025-12-05
-- Description: Add status column and deleted_at column for soft delete support

-- Add status column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'insights' AND column_name = 'status'
    ) THEN
        ALTER TABLE insights ADD COLUMN status VARCHAR(50) DEFAULT 'active';
        CREATE INDEX idx_insights_status ON insights(status);
        COMMENT ON COLUMN insights.status IS 'Status: active, resolved, dismissed';
    END IF;
END $$;

-- Add deleted_at column if it doesn't exist (for soft delete)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'insights' AND column_name = 'deleted_at'
    ) THEN
        ALTER TABLE insights ADD COLUMN deleted_at TIMESTAMP;
        CREATE INDEX idx_insights_deleted_at ON insights(deleted_at);
        COMMENT ON COLUMN insights.deleted_at IS 'Soft delete timestamp - NULL means not deleted';
    END IF;
END $$;

-- Update existing insights to have status = 'active' if NULL
UPDATE insights SET status = 'active' WHERE status IS NULL;

-- Add comment to table
COMMENT ON TABLE insights IS 'Risk insights with soft delete and status support';

