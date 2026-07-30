/**
 * Single source of truth for insight type labels (Risk Center, Pod Related Risks, Risk detail).
 * Matches Risk Operations filter options (e.g. Supply-chain malware, not abbreviated "Malware").
 */
export function insightTypeUiLabel(insightType: string | undefined): string {
  const t = (insightType || '').trim().toLowerCase();
  switch (t) {
    case 'vulnerability':
      return 'Vulnerability';
    case 'supply_chain_malware':
      return 'Supply-chain malware';
    case 'rbac':
    case 'rbac_risk':
      return 'Behavior';
    default:
      if (!insightType?.trim()) return 'Finding';
      return insightType.replace(/_/g, ' ');
  }
}

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

/** Get a secondary label for risk list items (CVE or package reference). */
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
