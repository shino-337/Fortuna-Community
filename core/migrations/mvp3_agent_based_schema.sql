-- Migration for Agent-Based Architecture Schema Updates
-- Date: 2025-12-23
-- Phase: MVP3 - Agent-Based SBOM & CVE Pipeline

-- ========================================
-- PART 1: Drop old tables (testing only)
-- ========================================
-- WARNING: This will delete ALL existing data!
-- For production, use ALTER TABLE instead (see PART 2)

DROP TABLE IF EXISTS cve_matches CASCADE;
DROP TABLE IF EXISTS sbom_components CASCADE;
DROP TABLE IF EXISTS sboms CASCADE;
DROP TABLE IF EXISTS insights CASCADE;
DROP TABLE IF EXISTS pod_image_scans CASCADE;

-- ========================================
-- PART 2: Create new tables with Agent-Based schema
-- ========================================

-- SBOM table with Agent context
CREATE TABLE IF NOT EXISTS sboms (
    id SERIAL PRIMARY KEY,
    image_name VARCHAR(255) NOT NULL,
    image_tag VARCHAR(255) NOT NULL,
    image_digest VARCHAR(255) NOT NULL UNIQUE,
    
    -- Pod context (from Agent)
    pod_uid VARCHAR(255),
    pod_name VARCHAR(255),
    namespace VARCHAR(255),
    container_name VARCHAR(255),
    
    -- OS info
    os_name VARCHAR(100),
    os_version VARCHAR(100),
    os_architecture VARCHAR(50),
    
    -- SBOM metadata
    package_count INTEGER DEFAULT 0,
    sbom_format VARCHAR(50) DEFAULT 'fortuna-agent',
    sbom_content JSONB,
    
    -- Agent info
    generated_at TIMESTAMP,
    agent_id VARCHAR(255),
    node_id VARCHAR(255),
    
    -- Labels & Annotations
    labels JSONB,
    annotations JSONB,
    
    -- Usage tracking
    last_used_at TIMESTAMP,
    use_count INTEGER DEFAULT 1,
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sboms_image_name ON sboms(image_name);
CREATE INDEX IF NOT EXISTS idx_sboms_image_tag ON sboms(image_tag);
CREATE INDEX IF NOT EXISTS idx_sboms_image_digest ON sboms(image_digest);
CREATE INDEX IF NOT EXISTS idx_sboms_namespace ON sboms(namespace);
CREATE INDEX IF NOT EXISTS idx_sboms_agent_id ON sboms(agent_id);
CREATE INDEX IF NOT EXISTS idx_sboms_node_id ON sboms(node_id);
CREATE INDEX IF NOT EXISTS idx_sboms_last_used_at ON sboms(last_used_at);
CREATE INDEX IF NOT EXISTS idx_sboms_deleted_at ON sboms(deleted_at);

-- SBOM Components (packages)
CREATE TABLE IF NOT EXISTS sbom_components (
    id SERIAL PRIMARY KEY,
    sbom_id INTEGER NOT NULL REFERENCES sboms(id) ON DELETE CASCADE,
    
    component_type VARCHAR(50) NOT NULL,
    component_name VARCHAR(255) NOT NULL,
    component_version VARCHAR(255) NOT NULL,
    purl VARCHAR(512),
    
    -- Additional metadata
    licenses TEXT,
    source VARCHAR(500),
    description TEXT,
    homepage VARCHAR(500),
    maintainer VARCHAR(255),
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sbom_components_sbom_id ON sbom_components(sbom_id);
CREATE INDEX IF NOT EXISTS idx_sbom_components_name ON sbom_components(component_name);
CREATE INDEX IF NOT EXISTS idx_sbom_components_purl ON sbom_components(purl);
CREATE INDEX IF NOT EXISTS idx_sbom_components_deleted_at ON sbom_components(deleted_at);

-- CVE Matches (Agent-Based: no component_id foreign key)
CREATE TABLE IF NOT EXISTS cve_matches (
    id SERIAL PRIMARY KEY,
    sbom_id INTEGER NOT NULL REFERENCES sboms(id) ON DELETE CASCADE,
    
    -- Pod context
    pod_uid VARCHAR(255),
    container_name VARCHAR(255),
    
    -- CVE info
    cve_id VARCHAR(20) NOT NULL,
    
    -- Package info (direct, no FK to sbom_components)
    package_name VARCHAR(255) NOT NULL,
    package_version VARCHAR(100) NOT NULL,
    purl VARCHAR(500),
    
    -- Severity & Fix
    severity VARCHAR(20) NOT NULL,
    cvss DECIMAL(4,1),
    fixed_version VARCHAR(255),
    matched_by VARCHAR(255),
    
    -- Timestamps
    matched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cve_matches_sbom_id ON cve_matches(sbom_id);
CREATE INDEX IF NOT EXISTS idx_cve_matches_cve_id ON cve_matches(cve_id);
CREATE INDEX IF NOT EXISTS idx_cve_matches_package_name ON cve_matches(package_name);
CREATE INDEX IF NOT EXISTS idx_cve_matches_severity ON cve_matches(severity);
CREATE INDEX IF NOT EXISTS idx_cve_matches_pod_uid ON cve_matches(pod_uid);
CREATE INDEX IF NOT EXISTS idx_cve_matches_deleted_at ON cve_matches(deleted_at);

-- Unique constraint: one CVE per package per SBOM
CREATE UNIQUE INDEX IF NOT EXISTS idx_cve_matches_unique 
ON cve_matches(sbom_id, cve_id, package_name) 
WHERE deleted_at IS NULL;

-- Insights (Agent-Based: direct resource fields, no JSONB)
CREATE TABLE IF NOT EXISTS insights (
    id SERIAL PRIMARY KEY,
    
    -- Resource context (direct fields, not JSONB)
    resource_type VARCHAR(50) NOT NULL,
    resource_namespace VARCHAR(255),
    resource_name VARCHAR(255) NOT NULL,
    resource_uid VARCHAR(255) NOT NULL,
    
    -- Insight info
    insight_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    title VARCHAR(500) NOT NULL,
    description TEXT NOT NULL,
    recommendation TEXT,
    
    -- CVE-specific fields (nullable)
    cve_id VARCHAR(20),
    affected_component VARCHAR(255),
    affected_version VARCHAR(100),
    fixed_version VARCHAR(100),
    cvss DECIMAL(4,1),
    
    -- Status & Timestamps
    status VARCHAR(20) DEFAULT 'active',
    detected_at TIMESTAMP NOT NULL,
    resolved_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_insights_resource_type ON insights(resource_type);
CREATE INDEX IF NOT EXISTS idx_insights_resource_namespace ON insights(resource_namespace);
CREATE INDEX IF NOT EXISTS idx_insights_resource_name ON insights(resource_name);
CREATE INDEX IF NOT EXISTS idx_insights_resource_uid ON insights(resource_uid);
CREATE INDEX IF NOT EXISTS idx_insights_insight_type ON insights(insight_type);
CREATE INDEX IF NOT EXISTS idx_insights_severity ON insights(severity);
CREATE INDEX IF NOT EXISTS idx_insights_cve_id ON insights(cve_id);
CREATE INDEX IF NOT EXISTS idx_insights_affected_component ON insights(affected_component);
CREATE INDEX IF NOT EXISTS idx_insights_status ON insights(status);
CREATE INDEX IF NOT EXISTS idx_insights_detected_at ON insights(detected_at);
CREATE INDEX IF NOT EXISTS idx_insights_deleted_at ON insights(deleted_at);

-- Unique constraint: one insight per resource + CVE + component
CREATE UNIQUE INDEX IF NOT EXISTS idx_insights_unique 
ON insights(resource_uid, cve_id, affected_component) 
WHERE deleted_at IS NULL;

-- Pod Image Scans (no changes needed, already compatible)
CREATE TABLE IF NOT EXISTS pod_image_scans (
    id SERIAL PRIMARY KEY,
    pod_uid VARCHAR(255) NOT NULL,
    pod_name VARCHAR(255) NOT NULL,
    pod_namespace VARCHAR(255) NOT NULL,
    cluster_id VARCHAR(255) NOT NULL,
    container_name VARCHAR(255) NOT NULL,
    container_image TEXT NOT NULL,
    image_name VARCHAR(255),
    image_tag VARCHAR(50),
    image_registry VARCHAR(255),
    sbom_id INTEGER REFERENCES sboms(id),
    scan_result_id INTEGER,
    pod_phase VARCHAR(20),
    pod_created_at TIMESTAMP,
    pod_deleted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pod_image_scans_pod_uid ON pod_image_scans(pod_uid);
CREATE INDEX IF NOT EXISTS idx_pod_image_scans_namespace ON pod_image_scans(pod_namespace);
CREATE INDEX IF NOT EXISTS idx_pod_image_scans_cluster ON pod_image_scans(cluster_id);
CREATE INDEX IF NOT EXISTS idx_pod_image_scans_sbom_id ON pod_image_scans(sbom_id);
CREATE INDEX IF NOT EXISTS idx_pod_image_scans_deleted_at ON pod_image_scans(deleted_at);

-- Unique constraint: one scan per pod + container
CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_image_scans_unique 
ON pod_image_scans(pod_uid, container_name) 
WHERE deleted_at IS NULL;

-- ========================================
-- PART 3: Comments & Documentation
-- ========================================

COMMENT ON TABLE sboms IS 'SBOM (Software Bill of Materials) from Agent with Pod context';
COMMENT ON COLUMN sboms.image_digest IS 'Immutable SHA256 digest, used for SBOM deduplication';
COMMENT ON COLUMN sboms.package_count IS 'Total number of packages in this SBOM';
COMMENT ON COLUMN sboms.agent_id IS 'ID of the Agent that generated this SBOM';

COMMENT ON TABLE cve_matches IS 'CVE matches from Agent with direct package info (no component_id FK)';
COMMENT ON COLUMN cve_matches.package_name IS 'Package name, used for unique constraint (no component_id)';
COMMENT ON COLUMN cve_matches.matched_by IS 'Version range that matched (e.g., ">=1.0.0 <1.2.0")';

COMMENT ON TABLE insights IS 'Security insights with direct resource fields (no JSONB)';
COMMENT ON COLUMN insights.resource_uid IS 'Unique identifier for the resource (e.g., Pod UID)';
COMMENT ON COLUMN insights.detected_at IS 'When the insight was first detected';
COMMENT ON COLUMN insights.resolved_at IS 'When the insight was marked as resolved';

