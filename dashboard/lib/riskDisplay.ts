/**
 * Human-readable reference for Risk Operations list/detail (CVE vs supply-chain malware).
 */
export function formatRiskFindingReference(insight: {
  insightType?: string;
  cveId?: string;
  affectedComponent?: string;
  affectedVersion?: string;
}): string {
  const t = (insight.insightType || '').toLowerCase();
  if (t === 'supply_chain_malware') {
    const pkg = (insight.affectedComponent || '').trim();
    const ver = (insight.affectedVersion || '').trim();
    if (pkg && ver) return `${pkg}@${ver}`;
    if (pkg) return pkg;
  }
  const raw = (insight.cveId || '').trim();
  if (raw.toLowerCase().startsWith('supply-malware:')) {
    return raw.slice('supply-malware:'.length);
  }
  return raw;
}

export function riskListSecondaryLabel(insight: {
  insightType?: string;
  cveId?: string;
  affectedComponent?: string;
  affectedVersion?: string;
  id?: string;
}): string {
  const ref = formatRiskFindingReference(insight);
  if (ref) return ref;
  return insight.id ?? '';
}
