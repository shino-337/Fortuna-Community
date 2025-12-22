-- Migration 005: Add CVE scanning tables
-- MVP2 Phase 2: CVE Detection Integration
-- Date: 2025-12-11

-- ============================================
-- CVE Tables
-- ============================================

-- Table 1: cves - Store CVE data from NVD
CREATE TABLE IF NOT EXISTS cves (
    id SERIAL PRIMARY KEY,
    cve_id VARCHAR(20) UNIQUE NOT NULL,
    cvss_score DECIMAL(3,1),
    cvss_vector TEXT,
    cvss_version VARCHAR(10),
    severity VARCHAR(20) NOT NULL,
    title TEXT,
    description TEXT,
    published_date TIMESTAMP WITH TIME ZONE,
    last_modified_date TIMESTAMP WITH TIME ZONE,
    exploit_available BOOLEAN DEFAULT FALSE,
    exploit_maturity VARCHAR(20),
    exploit_sources TEXT[],
    cve_references JSONB,
    cwe_ids TEXT[],
    source VARCHAR(50) NOT NULL DEFAULT 'nvd',
    source_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for cves
CREATE INDEX IF NOT EXISTS idx_cves_cve_id ON cves(cve_id);
CREATE INDEX IF NOT EXISTS idx_cves_severity ON cves(severity);
CREATE INDEX IF NOT EXISTS idx_cves_cvss_score ON cves(cvss_score DESC);
CREATE INDEX IF NOT EXISTS idx_cves_published_date ON cves(published_date DESC);
CREATE INDEX IF NOT EXISTS idx_cves_exploit_available ON cves(exploit_available) WHERE exploit_available = TRUE;
CREATE INDEX IF NOT EXISTS idx_cves_deleted_at ON cves(deleted_at) WHERE deleted_at IS NULL;

-- Comments
COMMENT ON TABLE cves IS 'Common Vulnerability and Exposure records from NVD and other sources';
COMMENT ON COLUMN cves.cve_id IS 'CVE identifier (e.g., CVE-2021-23017)';
COMMENT ON COLUMN cves.cvss_score IS 'CVSS score (0.0-10.0)';
COMMENT ON COLUMN cves.severity IS 'Severity level: CRITICAL, HIGH, MEDIUM, LOW';
COMMENT ON COLUMN cves.exploit_available IS 'Whether public exploit is available';

-- Table 2: package_vulnerabilities - Link CVEs to packages
CREATE TABLE IF NOT EXISTS package_vulnerabilities (
    id SERIAL PRIMARY KEY,
    cve_id VARCHAR(20) NOT NULL,
    package_name VARCHAR(255) NOT NULL,
    package_type VARCHAR(50),
    ecosystem VARCHAR(50),
    affected_range TEXT,
    version_start_including VARCHAR(50),
    version_start_excluding VARCHAR(50),
    version_end_including VARCHAR(50),
    version_end_excluding VARCHAR(50),
    fixed_version VARCHAR(50),
    fixed_in_versions TEXT[],
    vendor VARCHAR(100),
    product VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    FOREIGN KEY (cve_id) REFERENCES cves(cve_id) ON DELETE CASCADE
);

-- Indexes for package_vulnerabilities
CREATE INDEX IF NOT EXISTS idx_package_vulns_cve_id ON package_vulnerabilities(cve_id);
CREATE INDEX IF NOT EXISTS idx_package_vulns_package_name ON package_vulnerabilities(package_name);
CREATE INDEX IF NOT EXISTS idx_package_vulns_ecosystem ON package_vulnerabilities(ecosystem);
CREATE INDEX IF NOT EXISTS idx_package_vulns_package_ecosystem ON package_vulnerabilities(package_name, ecosystem);
CREATE UNIQUE INDEX IF NOT EXISTS idx_package_vulns_unique ON package_vulnerabilities(cve_id, package_name, ecosystem, COALESCE(version_end_excluding, ''));

-- Comments
COMMENT ON TABLE package_vulnerabilities IS 'Links CVEs to affected packages and version ranges';
COMMENT ON COLUMN package_vulnerabilities.fixed_version IS 'Version that fixes the vulnerability';

-- Table 3: image_scan_results - Store Trivy scan results
CREATE TABLE IF NOT EXISTS image_scan_results (
    id SERIAL PRIMARY KEY,
    image_name VARCHAR(255) NOT NULL,
    image_tag VARCHAR(50) NOT NULL,
    image_digest VARCHAR(71),
    registry VARCHAR(255),
    full_image_ref TEXT,
    scanned_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    scanner_name VARCHAR(50) DEFAULT 'trivy',
    scanner_version VARCHAR(50),
    scan_duration_seconds DECIMAL(10,2),
    os_family VARCHAR(50),
    os_name VARCHAR(100),
    os_version VARCHAR(50),
    total_vulnerabilities INT DEFAULT 0,
    critical_count INT DEFAULT 0,
    high_count INT DEFAULT 0,
    medium_count INT DEFAULT 0,
    low_count INT DEFAULT 0,
    unknown_count INT DEFAULT 0,
    vulnerabilities JSONB,
    packages JSONB,
    status VARCHAR(20) DEFAULT 'in_progress',
    error_message TEXT,
    cache_key VARCHAR(100),
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for image_scan_results
CREATE INDEX IF NOT EXISTS idx_image_scans_image_tag ON image_scan_results(image_name, image_tag);
CREATE INDEX IF NOT EXISTS idx_image_scans_digest ON image_scan_results(image_digest);
CREATE INDEX IF NOT EXISTS idx_image_scans_scanned_at ON image_scan_results(scanned_at DESC);
CREATE INDEX IF NOT EXISTS idx_image_scans_status ON image_scan_results(status);
CREATE INDEX IF NOT EXISTS idx_image_scans_critical ON image_scan_results(critical_count) WHERE critical_count > 0;
CREATE INDEX IF NOT EXISTS idx_image_scans_vulnerabilities ON image_scan_results USING GIN(vulnerabilities);
CREATE UNIQUE INDEX IF NOT EXISTS idx_image_scans_unique ON image_scan_results(image_name, image_tag, COALESCE(image_digest, ''));

-- Comments
COMMENT ON TABLE image_scan_results IS 'Container image vulnerability scan results from Trivy';
COMMENT ON COLUMN image_scan_results.vulnerabilities IS 'Full JSON array of vulnerabilities found';
COMMENT ON COLUMN image_scan_results.packages IS 'All packages found in the image';

-- Table 4: pod_image_scans - Map pods to scan results
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
    scan_result_id INT,
    pod_phase VARCHAR(20),
    pod_created_at TIMESTAMP WITH TIME ZONE,
    pod_deleted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    FOREIGN KEY (scan_result_id) REFERENCES image_scan_results(id) ON DELETE SET NULL
);

-- Indexes for pod_image_scans
CREATE INDEX IF NOT EXISTS idx_pod_scans_pod_uid ON pod_image_scans(pod_uid);
CREATE INDEX IF NOT EXISTS idx_pod_scans_cluster_namespace ON pod_image_scans(cluster_id, pod_namespace);
CREATE INDEX IF NOT EXISTS idx_pod_scans_scan_result ON pod_image_scans(scan_result_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_scans_unique ON pod_image_scans(pod_uid, container_name);

-- Comments
COMMENT ON TABLE pod_image_scans IS 'Maps Kubernetes pods to their container image scan results';

-- ============================================
-- Update insights table for CVE support
-- ============================================

-- Add CVE-specific columns to insights table
ALTER TABLE insights 
ADD COLUMN IF NOT EXISTS cve_id VARCHAR(20),
ADD COLUMN IF NOT EXISTS cvss_score DECIMAL(3,1),
ADD COLUMN IF NOT EXISTS cvss_vector TEXT,
ADD COLUMN IF NOT EXISTS exploit_available BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS package_name VARCHAR(255),
ADD COLUMN IF NOT EXISTS installed_version VARCHAR(50),
ADD COLUMN IF NOT EXISTS fixed_version VARCHAR(50);

-- Indexes for CVE fields in insights
CREATE INDEX IF NOT EXISTS idx_insights_cve_id ON insights(cve_id) WHERE cve_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_insights_exploit ON insights(exploit_available) WHERE exploit_available = TRUE;
CREATE INDEX IF NOT EXISTS idx_insights_package ON insights(package_name) WHERE package_name IS NOT NULL;

-- Comments
COMMENT ON COLUMN insights.cve_id IS 'CVE identifier for vulnerability insights';
COMMENT ON COLUMN insights.cvss_score IS 'CVSS score for CVE insights';
COMMENT ON COLUMN insights.exploit_available IS 'Whether exploit is available for this CVE';

