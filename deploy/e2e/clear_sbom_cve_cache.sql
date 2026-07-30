-- Fortuna E2E helper: clear SBOM/CVE derived caches for a clean end-to-end run.
-- WARNING: this deletes SBOM/CVE artifacts and vulnerability insights. Use on dev/test DB only.

-- Remove previous synthetic CVE test data (optional)
DELETE FROM package_vulnerabilities WHERE cve_id = 'CVE-2099-9999';
DELETE FROM cves WHERE cve_id = 'CVE-2099-9999';

-- Clear derived artifacts so the pipeline regenerates from scratch
-- Clear only vulnerability insights (keep policy/risk insights)
DELETE FROM insights WHERE type = 'vulnerability';

-- Now it is safe to clear CVE/SBOM derived artifacts (FKs from insights may reference cve_matches/sboms)
DELETE FROM cve_matches;
DELETE FROM sbom_components;
DELETE FROM sboms;
DELETE FROM pod_image_scans;


