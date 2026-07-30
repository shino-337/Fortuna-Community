-- Run after reset-db if Core logs "relation does not exist" for agents or k8s_events.
-- Prefer: restart Core with image built from current repo (migrations 093+094 apply automatically).
--
--   kubectl exec -i -n fortuna deploy/postgres -- psql -U postgres -d fortuna -f - < scripts/e2e/fixtures/deploy-e2e/repair_missing_tables_after_reset.sql

CREATE TABLE IF NOT EXISTS agents (
  id SERIAL PRIMARY KEY,
  agent_id VARCHAR(255) UNIQUE NOT NULL,
  node_name VARCHAR(255),
  version VARCHAR(100),
  status VARCHAR(50),
  capabilities JSONB,
  last_seen_at TIMESTAMP WITH TIME ZONE,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_agents_deleted_at ON agents(deleted_at);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);

CREATE TABLE IF NOT EXISTS k8s_events (
  id BIGSERIAL PRIMARY KEY,
  cluster_id VARCHAR(255) NOT NULL,
  event_uid VARCHAR(255) NOT NULL,
  namespace VARCHAR(255) NOT NULL,
  event_name VARCHAR(255) NOT NULL,
  involved_kind VARCHAR(64) NOT NULL,
  involved_uid VARCHAR(255) NOT NULL,
  involved_name VARCHAR(255) NOT NULL,
  reason VARCHAR(128) NOT NULL,
  message TEXT DEFAULT '',
  event_type VARCHAR(32) DEFAULT 'Normal',
  count INTEGER DEFAULT 1,
  first_timestamp TIMESTAMP WITH TIME ZONE,
  last_timestamp TIMESTAMP WITH TIME ZONE,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_k8s_events_uid ON k8s_events(cluster_id, event_uid);
CREATE INDEX IF NOT EXISTS idx_k8s_events_involved_uid ON k8s_events(involved_uid);
CREATE INDEX IF NOT EXISTS idx_k8s_events_last_timestamp ON k8s_events(last_timestamp);
