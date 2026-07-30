-- Create pod_capabilities table if missing (same as migration 041).
-- Run if API returns "relation pod_capabilities does not exist".
-- Usage: kubectl cp scripts/e2e/fixtures/deploy-e2e/ensure_pod_capabilities_table.sql fortuna/<postgres-pod>:/tmp/ && kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -f /tmp/ensure_pod_capabilities_table.sql

CREATE TABLE IF NOT EXISTS pod_capabilities (
    id SERIAL PRIMARY KEY,
    pod_uid VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    capability_id VARCHAR(100) NOT NULL,
    capability_group VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    evidence JSONB,
    mitre TEXT[],
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_capabilities_unique ON pod_capabilities(pod_uid, capability_id);
CREATE INDEX IF NOT EXISTS idx_pod_capabilities_pod_uid ON pod_capabilities(pod_uid);
CREATE INDEX IF NOT EXISTS idx_pod_capabilities_capability_id ON pod_capabilities(capability_id);
