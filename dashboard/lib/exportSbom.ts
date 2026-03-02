import type { PodSbom, SbomComponent } from '../types';

function escapeCsv(s: string): string {
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

/**
 * Export SBOM components as CSV (one row per component; vulnerabilities as comma-separated).
 */
export function exportSbomAsCsv(sbom: PodSbom): void {
  const headers = [
    'Pod Name',
    'Namespace',
    'Image',
    'Generated At',
    'Package Name',
    'Version',
    'Type',
    'PURL',
    'Vulnerability Count',
    'CVE IDs',
    'Severities',
  ];
  const rows = (sbom.components || []).map((c: SbomComponent) => [
    sbom.podName,
    sbom.namespace,
    sbom.image,
    sbom.generatedAt ?? '',
    c.name,
    c.version ?? '',
    (c.type as string) ?? '',
    c.purl ?? '',
    String((c.vulnerabilities || []).length),
    (c.vulnerabilities || []).map((v) => v.id).join('; '),
    (c.vulnerabilities || []).map((v) => v.severity).join('; '),
  ]);
  const lines = [headers.map(escapeCsv).join(',')].concat(rows.map((r) => r.map(escapeCsv).join(',')));
  const blob = new Blob([lines.join('\n')], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `sbom-${sbom.podName || sbom.podId}-${new Date().toISOString().slice(0, 10)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

/**
 * Export full SBOM as JSON (pod metadata + all components with vulnerabilities).
 */
export function exportSbomAsJson(sbom: PodSbom): void {
  const blob = new Blob([JSON.stringify(sbom, null, 2)], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `sbom-${sbom.podName || sbom.podId}-${new Date().toISOString().slice(0, 10)}.json`;
  a.click();
  URL.revokeObjectURL(url);
}
