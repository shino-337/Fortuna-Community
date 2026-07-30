-- Remove E2E test data from database (run on dev/test only).
DELETE FROM package_vulnerabilities WHERE affected_range LIKE 'e2e-%';
DELETE FROM cves WHERE cve_id = 'CVE-2099-9999';
DELETE FROM package_vulnerabilities WHERE cve_id = 'CVE-2099-9999';
DELETE FROM agents WHERE agent_id = 'e2e-injector';
