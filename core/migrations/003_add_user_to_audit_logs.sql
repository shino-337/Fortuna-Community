-- Migration 003: Add user_id column to audit_logs
-- Date: 2025-12-27
-- Description: Links audit logs to users table
-- Rollback: ALTER TABLE audit_logs DROP COLUMN IF EXISTS user_id CASCADE;

-- Add user_id column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = CURRENT_SCHEMA()
        AND table_name = 'audit_logs' 
        AND column_name = 'user_id'
    ) THEN
        ALTER TABLE audit_logs ADD COLUMN user_id INTEGER;

        -- Add foreign key constraint
        ALTER TABLE audit_logs
        ADD CONSTRAINT fk_audit_logs_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

        -- Add index
        CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
    END IF;
END $$;

