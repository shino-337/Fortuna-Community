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
    'SBOM Source',
    'SBOM Confidence',
    'Go Version',
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
    sbom.sbomSource ?? '',
    sbom.confidence ?? '',
    sbom.goVersion ?? '',
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

function safeRef(s: string | undefined): string {
  return (s ?? '').replace(/[^A-Za-z0-9.\-]/g, '-');
}

function toIsoTime(input?: string): string {
  if (!input) return new Date().toISOString();
  const d = new Date(input);
  if (Number.isNaN(d.getTime())) return new Date().toISOString();
  return d.toISOString();
}

function randomUuid(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  const s4 = () => Math.floor((1 + Math.random()) * 0x10000).toString(16).substring(1);
  return `${s4()}${s4()}-${s4()}-4${s4().slice(1)}-${((8 + Math.random() * 4) | 0).toString(16)}${s4().slice(1)}-${s4()}${s4()}${s4()}`;
}

function maxSeverityRank(vulns: Array<{ severity?: string }>): string {
  const order = ['critical', 'high', 'medium', 'low'];
  for (const sev of order) {
    if (vulns.some((v) => (v.severity ?? '').toLowerCase() === sev)) return sev.toUpperCase();
  }
  return 'NONE';
}

function downloadJson(filename: string, data: unknown): void {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

function toCdxSeverity(input?: string): 'critical' | 'high' | 'medium' | 'low' | 'info' | 'none' | 'unknown' {
  const s = (input ?? '').trim().toLowerCase();
  if (s === 'critical' || s === 'high' || s === 'medium' || s === 'low' || s === 'info' || s === 'none') return s;
  return 'unknown';
}

function toSpdxExternalRefsForVulns(vulns: Array<{ id?: string }>): Array<{ referenceCategory: string; referenceType: string; referenceLocator: string }> {
  const refs: Array<{ referenceCategory: string; referenceType: string; referenceLocator: string }> = [];
  for (const v of vulns || []) {
    const id = (v?.id ?? '').trim();
    if (!id) continue;
    refs.push({
      referenceCategory: 'SECURITY',
      referenceType: 'advisory',
      referenceLocator: `https://nvd.nist.gov/vuln/detail/${id}`,
    });
  }
  return refs;
}

/**
 * Export SBOM as SPDX 2.3 JSON (package-level with purl external refs).
 */
export function exportSbomAsSpdxJson(sbom: PodSbom): void {
  const podRef = safeRef(sbom.podName || sbom.podId || 'pod');
  const created = toIsoTime(sbom.generatedAt);
  const namespace = `https://fortuna.local/spdx/${podRef}/${Date.now()}`;

  const packages = (sbom.components || []).map((c: SbomComponent, idx: number) => {
    const pkgId = `SPDXRef-Package-${idx + 1}-${safeRef(c.name || 'pkg')}`;
    const externalRefs = [];
    if (c.purl) {
      externalRefs.push({
        referenceCategory: 'PACKAGE-MANAGER',
        referenceType: 'purl',
        referenceLocator: c.purl,
      });
    }
    externalRefs.push(...toSpdxExternalRefsForVulns(c.vulnerabilities || []));
    const vulnList = (c.vulnerabilities || []).map((v) => v.id).filter(Boolean).join(', ');

    return {
      SPDXID: pkgId,
      name: c.name || 'unknown',
      versionInfo: c.version || 'unknown',
      downloadLocation: 'NOASSERTION',
      filesAnalyzed: false,
      licenseConcluded: 'NOASSERTION',
      licenseDeclared: 'NOASSERTION',
      copyrightText: 'NOASSERTION',
      externalRefs,
      summary: vulnList ? `Known vulnerabilities: ${vulnList}` : undefined,
    };
  });

  const spdx = {
    spdxVersion: 'SPDX-2.3',
    dataLicense: 'CC0-1.0',
    SPDXID: 'SPDXRef-DOCUMENT',
    name: `fortuna-sbom-${podRef}`,
    documentNamespace: namespace,
    creationInfo: {
      created,
      creators: ['Tool: Fortuna Dashboard Exporter'],
    },
    documentDescribes: packages.map((p: { SPDXID: string }) => p.SPDXID),
    packages,
    relationships: packages.map((p: { SPDXID: string }) => ({
      spdxElementId: 'SPDXRef-DOCUMENT',
      relationshipType: 'DESCRIBES',
      relatedSpdxElement: p.SPDXID,
    })),
  };

  downloadJson(`sbom-${podRef}-${new Date().toISOString().slice(0, 10)}.spdx.json`, spdx);
}

/**
 * Export SBOM as CycloneDX 1.5 JSON.
 */
export function exportSbomAsCycloneDxJson(sbom: PodSbom): void {
  const podRef = safeRef(sbom.podName || sbom.podId || 'pod');
  const created = toIsoTime(sbom.generatedAt);
  const vulnCount = (sbom.components || []).reduce((acc, c) => acc + (c.vulnerabilities || []).length, 0);

  const components = (sbom.components || []).map((c: SbomComponent) => ({
    type: c.type === 'os-package' ? 'operating-system' : 'library',
    'bom-ref': c.purl || `${c.name}@${c.version || 'unknown'}`,
    name: c.name,
    version: c.version || 'unknown',
    purl: c.purl,
    properties: [
      { name: 'fortuna:cveCount', value: String((c.vulnerabilities || []).length) },
      { name: 'fortuna:maxSeverity', value: maxSeverityRank(c.vulnerabilities || []) },
    ],
  }));

  const vulnerabilities = (sbom.components || []).flatMap((c: SbomComponent) =>
    (c.vulnerabilities || []).map((v) => ({
      id: v.id,
      source: { name: 'NVD' },
      ratings: [
        {
          severity: toCdxSeverity(v.severity),
          score: v.cvssScore ?? undefined,
          method: 'CVSSv31',
        },
      ],
      analysis: {
        state: v.status === 'fixed' ? 'resolved' : 'exploitable',
      },
      affects: [
        {
          ref: c.purl || `${c.name}@${c.version || 'unknown'}`,
        },
      ],
      recommendation: v.fixedVersion ? `Upgrade to ${v.fixedVersion} or newer` : undefined,
      description: v.description || undefined,
    }))
  );

  const cdx = {
    bomFormat: 'CycloneDX',
    specVersion: '1.5',
    serialNumber: `urn:uuid:${randomUuid()}`,
    version: 1,
    metadata: {
      timestamp: created,
      tools: [{ vendor: 'Fortuna', name: 'Dashboard Exporter' }],
      component: {
        type: 'container',
        name: sbom.podName || sbom.podId || 'pod',
        version: sbom.image || 'unknown',
        properties: [
          { name: 'fortuna:namespace', value: sbom.namespace || '' },
          { name: 'fortuna:container', value: sbom.container || '' },
          { name: 'fortuna:sbomSource', value: sbom.sbomSource || '' },
          { name: 'fortuna:confidence', value: sbom.confidence || '' },
          { name: 'fortuna:goVersion', value: sbom.goVersion || '' },
          { name: 'fortuna:vulnerabilityCount', value: String(vulnCount) },
        ],
      },
    },
    components,
    vulnerabilities,
  };

  downloadJson(`sbom-${podRef}-${new Date().toISOString().slice(0, 10)}.cdx.json`, cdx);
}
