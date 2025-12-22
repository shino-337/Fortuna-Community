-- Migration 006: Add SBOM-based scanning tables
-- MVP2 Phase 2: SBOM-based CVE Detection (replacing Trivy)
-- Date: 2025-12-12

-- ============================================
-- SBOM Tables
-- ============================================

-- Table 1: sboms - Persistent SBOM cache
CREATE TABLE IF NOT EXISTS sboms (
    id SERIAL PRIMARY KEY,
    
    -- Image identification
    image_name VARCHAR(255) NOT NULL,
    image_tag VARCHAR(255) NOT NULL,
    image_digest VARCHAR(255) NOT NULL UNIQUE,  -- SHA256, immutable!
    
    -- SBOM content
    sbom_format VARCHAR(50) NOT NULL DEFAULT 'cyclonedx-json',
    sbom_content JSONB NOT NULL,  -- CycloneDX JSON
    
    -- Component summary
    component_count INTEGER NOT NULL DEFAULT 0,
    os_packages INTEGER DEFAULT 0,
    language_packages INTEGER DEFAULT 0,
    
    -- Generation metadata
    generator VARCHAR(100) DEFAULT 'syft',
    generator_version VARCHAR(50),
    generated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Caching
    last_used_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    use_count INTEGER DEFAULT 1,
    
    -- Indexing
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for sboms
CREATE INDEX IF NOT EXISTS idx_sboms_image_digest ON sboms(image_digest);
CREATE INDEX IF NOT EXISTS idx_sboms_image_name_tag ON sboms(image_name, image_tag);
CREATE INDEX IF NOT EXISTS idx_sboms_last_used ON sboms(last_used_at);
CREATE INDEX IF NOT EXISTS idx_sboms_generated_at ON sboms(generated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sboms_deleted_at ON sboms(deleted_at) WHERE deleted_at IS NULL;

-- Comments
COMMENT ON TABLE sboms IS 'Software Bill of Materials cache - persistent, never expires (digest-based)';
COMMENT ON COLUMN sboms.image_digest IS 'SHA256 digest - immutable identifier for image';
COMMENT ON COLUMN sboms.sbom_content IS 'Full CycloneDX JSON content';
COMMENT ON COLUMN sboms.last_used_at IS 'Last time this SBOM was used for CVE matching';

-- Table 2: sbom_components - Extracted components for fast querying
CREATE TABLE IF NOT EXISTS sbom_components (
    id SERIAL PRIMARY KEY,
    sbom_id INTEGER NOT NULL,
    
    -- Component identification
    component_type VARCHAR(50) NOT NULL,  -- library, application, os
    component_name VARCHAR(255) NOT NULL,
    component_version VARCHAR(255) NOT NULL,
    purl VARCHAR(512),  -- Package URL (standard)
    
    -- Additional info
    licenses JSONB,
    supplier VARCHAR(255),
    
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    FOREIGN KEY (sbom_id) REFERENCES sboms(id) ON DELETE CASCADE
);

-- Indexes for sbom_components
CREATE INDEX IF NOT EXISTS idx_sbom_components_sbom_id ON sbom_components(sbom_id);
CREATE INDEX IF NOT EXISTS idx_sbom_components_purl ON sbom_components(purl);
CREATE INDEX IF NOT EXISTS idx_sbom_components_name_version 
    ON sbom_components(component_name, component_version);
CREATE INDEX IF NOT EXISTS idx_sbom_components_type ON sbom_components(component_type);

-- Comments
COMMENT ON TABLE sbom_components IS 'Extracted components from SBOM for fast CVE matching';
COMMENT ON COLUMN sbom_components.purl IS 'Package URL (pkg:type/namespace/name@version)';

-- Table 3: cve_matches - Cached CVE matches
CREATE TABLE IF NOT EXISTS cve_matches (
    id SERIAL PRIMARY KEY,
    sbom_id INTEGER NOT NULL,
    component_id INTEGER NOT NULL,
    
    -- CVE information
    cve_id VARCHAR(20) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    cvss_score DECIMAL(3,1),
    
    -- Fix information
    fixed_version VARCHAR(255),
    
    -- Metadata
    matched_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    matcher VARCHAR(50) DEFAULT 'grype',
    
    -- Freshness tracking
    db_version VARCHAR(50),  -- Grype DB version used
    needs_recheck BOOLEAN DEFAULT FALSE,
    
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    FOREIGN KEY (sbom_id) REFERENCES sboms(id) ON DELETE CASCADE,
    FOREIGN KEY (component_id) REFERENCES sbom_components(id) ON DELETE CASCADE
);

-- Indexes for cve_matches
CREATE INDEX IF NOT EXISTS idx_cve_matches_sbom_id ON cve_matches(sbom_id);
CREATE INDEX IF NOT EXISTS idx_cve_matches_component_id ON cve_matches(component_id);
CREATE INDEX IF NOT EXISTS idx_cve_matches_cve_id ON cve_matches(cve_id);
CREATE INDEX IF NOT EXISTS idx_cve_matches_severity ON cve_matches(severity);
CREATE INDEX IF NOT EXISTS idx_cve_matches_needs_recheck ON cve_matches(needs_recheck) WHERE needs_recheck = TRUE;
CREATE INDEX IF NOT EXISTS idx_cve_matches_matched_at ON cve_matches(matched_at DESC);

-- Comments
COMMENT ON TABLE cve_matches IS 'CVE matches for SBOM components - cached for fast lookup';
COMMENT ON COLUMN cve_matches.needs_recheck IS 'Set to true when CVE DB updates, triggers re-matching';

-- ============================================
-- Update existing tables
-- ============================================

-- Add SBOM references to insights table
ALTER TABLE insights 
ADD COLUMN IF NOT EXISTS sbom_id INTEGER REFERENCES sboms(id),
ADD COLUMN IF NOT EXISTS cve_match_id INTEGER REFERENCES cve_matches(id);

CREATE INDEX IF NOT EXISTS idx_insights_sbom_id ON insights(sbom_id) WHERE sbom_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_insights_cve_match_id ON insights(cve_match_id) WHERE cve_match_id IS NOT NULL;

-- Update pod_image_scans to reference SBOM instead of scan_result
ALTER TABLE pod_image_scans
ADD COLUMN IF NOT EXISTS sbom_id INTEGER REFERENCES sboms(id);

CREATE INDEX IF NOT EXISTS idx_pod_scans_sbom_id ON pod_image_scans(sbom_id) WHERE sbom_id IS NOT NULL;

-- Comments
COMMENT ON COLUMN insights.sbom_id IS 'Reference to SBOM used to detect this vulnerability';
COMMENT ON COLUMN insights.cve_match_id IS 'Reference to CVE match that created this insight';
COMMENT ON COLUMN pod_image_scans.sbom_id IS 'Reference to SBOM for this pod image';


