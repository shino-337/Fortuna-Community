-- Ensure cves and package_vulnerabilities exist (same DDL as migration 019).
-- Run after DB reset if Core migrations did not create them (e.g. migration order/skip).
-- Usage: kubectl cp scripts/e2e/fixtures/deploy-e2e/ensure_cve_tables.sql fortuna/<postgres-pod>:/tmp/ && kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -f /tmp/ensure_cve_tables.sql

CREATE TABLE IF NOT EXISTS cves (
    id SERIAL PRIMARY KEY,
    cve_id VARCHAR(100) UNIQUE NOT NULL,
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
    "cve_references" JSONB,
    cwe_ids TEXT[],
    source VARCHAR(50) NOT NULL DEFAULT 'nvd',
    source_url TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS package_vulnerabilities (
    id BIGSERIAL PRIMARY KEY,
    cve_id VARCHAR(100) NOT NULL,
    package_name VARCHAR(255) NOT NULL,
    package_type VARCHAR(255),
    ecosystem VARCHAR(255),
    affected_range TEXT,
    version_start_including VARCHAR(255),
    version_start_excluding VARCHAR(255),
    version_end_including VARCHAR(255),
    version_end_excluding VARCHAR(255),
    fixed_version VARCHAR(255),
    fixed_in_versions TEXT[],
    vendor VARCHAR(100),
    product VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT fk_cves_package_vulnerabilities FOREIGN KEY (cve_id) REFERENCES cves(cve_id)
);
