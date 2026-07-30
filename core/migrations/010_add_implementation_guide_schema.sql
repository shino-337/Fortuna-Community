-- Migration 010: Add tables according to IMPLEMENTATION_GUIDE.md
-- This migration adds: nodes, policies, insights, events_index

-- nodes table
CREATE TABLE IF NOT EXISTS nodes (
  id SERIAL PRIMARY KEY,
  cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  node_name TEXT NOT NULL,
  ip TEXT,
  kubelet_version TEXT,
  last_seen TIMESTAMP,
  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP DEFAULT now(),
  UNIQUE(cluster_id, node_name)
);

CREATE INDEX IF NOT EXISTS idx_nodes_cluster_id ON nodes(cluster_id);
CREATE INDEX IF NOT EXISTS idx_nodes_node_name ON nodes(node_name);

-- policies table (for generated policies)
CREATE TABLE IF NOT EXISTS policies (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL, -- e.g., "KubeArmor", "NetworkPolicy"
  policy_yaml TEXT NOT NULL,
  generated_by TEXT, -- e.g., "risk_engine", "policy_engine"
  status TEXT DEFAULT 'pending', -- pending, approved, applied, rejected
  created_by TEXT,
  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_policies_status ON policies(status);
CREATE INDEX IF NOT EXISTS idx_policies_type ON policies(type);

-- insights table (for risk insights)
CREATE TABLE IF NOT EXISTS insights (
  id SERIAL PRIMARY KEY,
  type TEXT NOT NULL, -- e.g., "rbac_risk", "network_risk", "runtime_anomaly"
  description TEXT NOT NULL,
  affected_resources JSONB, -- Array of resource references
  severity TEXT NOT NULL, -- low, medium, high, critical
  recommended_action TEXT,
  created_at TIMESTAMP DEFAULT now(),
  updated_at TIMESTAMP DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_insights_type ON insights(type);
CREATE INDEX IF NOT EXISTS idx_insights_severity ON insights(severity);
CREATE INDEX IF NOT EXISTS idx_insights_created_at ON insights(created_at);

-- events_index table (pointer to ClickHouse/Timescale for raw events)
CREATE TABLE IF NOT EXISTS events_index (
  event_id TEXT PRIMARY KEY,
  cluster_id TEXT REFERENCES clusters(id) ON DELETE CASCADE,
  node_id INTEGER REFERENCES nodes(id) ON DELETE SET NULL,
  pod_uid TEXT,
  ts TIMESTAMP NOT NULL,
  summary JSONB, -- Summary of event (type, severity, etc.)
  created_at TIMESTAMP DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_index_cluster_id ON events_index(cluster_id);
CREATE INDEX IF NOT EXISTS idx_events_index_node_id ON events_index(node_id);
CREATE INDEX IF NOT EXISTS idx_events_index_pod_uid ON events_index(pod_uid);
CREATE INDEX IF NOT EXISTS idx_events_index_ts ON events_index(ts);

-- Update clusters table to add region field if not exists
DO $$ 
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                 WHERE table_name='clusters' AND column_name='region') THEN
    ALTER TABLE clusters ADD COLUMN region TEXT;
  END IF;
END $$;

-- Update namespaces table to add labels and annotations as JSONB if not exists
DO $$ 
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                 WHERE table_name='namespaces' AND column_name='labels') THEN
    ALTER TABLE namespaces ADD COLUMN labels JSONB;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                 WHERE table_name='namespaces' AND column_name='annotations') THEN
    ALTER TABLE namespaces ADD COLUMN annotations JSONB;
  END IF;
END $$;

-- Update pods table structure to match IMPLEMENTATION_GUIDE
-- Note: pods table already exists, we'll add missing fields
DO $$ 
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                 WHERE table_name='pods' AND column_name='containers') THEN
    ALTER TABLE pods ADD COLUMN containers JSONB;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                 WHERE table_name='pods' AND column_name='image_digests') THEN
    ALTER TABLE pods ADD COLUMN image_digests JSONB;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                 WHERE table_name='pods' AND column_name='node_id') THEN
    ALTER TABLE pods ADD COLUMN node_id INTEGER REFERENCES nodes(id) ON DELETE SET NULL;
    CREATE INDEX IF NOT EXISTS idx_pods_node_id ON pods(node_id);
  END IF;
END $$;

-- Update service_accounts table to add linked_pods and last_used
DO $$ 
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                 WHERE table_name='service_accounts' AND column_name='linked_pods') THEN
    ALTER TABLE service_accounts ADD COLUMN linked_pods JSONB;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                 WHERE table_name='service_accounts' AND column_name='last_used') THEN
    ALTER TABLE service_accounts ADD COLUMN last_used TIMESTAMP;
  END IF;
END $$;

-- Update roles table to add cluster_scoped field
DO $$ 
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns 
                 WHERE table_name='roles' AND column_name='cluster_scoped') THEN
    ALTER TABLE roles ADD COLUMN cluster_scoped BOOLEAN DEFAULT FALSE;
  END IF;
END $$;


