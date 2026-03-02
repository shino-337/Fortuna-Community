-- One-off: expand package_vulnerabilities string columns (same as migration 067).
-- Run if CVE loader fails with "value too long for type character varying(50)".
-- Usage: kubectl cp deploy/e2e/067_expand_package_vuln_version_columns.sql fortuna/<postgres-pod>:/tmp/ && kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -f /tmp/067_expand_package_vuln_version_columns.sql

ALTER TABLE package_vulnerabilities ALTER COLUMN package_type TYPE VARCHAR(255);
ALTER TABLE package_vulnerabilities ALTER COLUMN ecosystem TYPE VARCHAR(255);
ALTER TABLE package_vulnerabilities ALTER COLUMN version_start_including TYPE VARCHAR(255);
ALTER TABLE package_vulnerabilities ALTER COLUMN version_start_excluding TYPE VARCHAR(255);
ALTER TABLE package_vulnerabilities ALTER COLUMN version_end_including TYPE VARCHAR(255);
ALTER TABLE package_vulnerabilities ALTER COLUMN version_end_excluding TYPE VARCHAR(255);
ALTER TABLE package_vulnerabilities ALTER COLUMN fixed_version TYPE VARCHAR(255);
