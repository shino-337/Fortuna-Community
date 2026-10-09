/**
 * Links for vulnerability and advisory IDs (CVE, GHSA, DSA/DLA, RHSA, MAL, GO, PYSEC, ALSA, …).
 * Findings are keyed by CVE when one exists, otherwise by the advisory that reported them, so a
 * finding ID is not always a CVE and must not be sent to NVD blindly.
 */

const normalize = (id: string) => id.trim();

/** Public page for a vulnerability or advisory ID; OSV covers every ecosystem we do not map explicitly. */
export const advisoryUrl = (rawId: string): string => {
  const id = normalize(rawId);
  const upper = id.toUpperCase();
  // Keep ':' readable (RHSA-2024:1234) while escaping anything that could break out of the path.
  const enc = encodeURIComponent(id).replace(/%3A/gi, ':');
  if (upper.startsWith('CVE-')) return `https://nvd.nist.gov/vuln/detail/${enc}`;
  if (upper.startsWith('GHSA-')) return `https://github.com/advisories/${enc}`;
  if (upper.startsWith('DSA-') || upper.startsWith('DLA-')) return `https://security-tracker.debian.org/tracker/${enc}`;
  if (upper.startsWith('RHSA-') || upper.startsWith('RHBA-') || upper.startsWith('RHEA-')) return `https://access.redhat.com/errata/${enc}`;
  return `https://osv.dev/vulnerability/${enc}`;
};

/** Short name of the database that publishes an ID (CycloneDX rating/vulnerability source name). */
export const advisorySourceName = (rawId: string): string => {
  const upper = normalize(rawId).toUpperCase();
  if (upper.startsWith('CVE-')) return 'NVD';
  if (upper.startsWith('GHSA-')) return 'GitHub Advisories';
  if (upper.startsWith('DSA-') || upper.startsWith('DLA-')) return 'Debian Security Tracker';
  if (upper.startsWith('RHSA-') || upper.startsWith('RHBA-') || upper.startsWith('RHEA-')) return 'Red Hat';
  return 'OSV';
};

/** Opens an advisory page in a new tab without giving it access to this window. */
export const openAdvisory = (id: string) => {
  window.open(advisoryUrl(id), '_blank', 'noopener');
};
