import type { ProvenanceKind } from '../design-system/components/ProvenanceBadge';
import type { Insight } from '../types';

/** Classify finding evidence lineage for trust UX (Layer 3 explainability). */
export function insightProvenance(insight: Insight): ProvenanceKind {
  const type = String(insight.insightType || '').toLowerCase();
  if ((insight as { degraded?: boolean }).degraded) return 'partial';
  if (type === 'vulnerability' || type === 'supply_chain_malware') {
    const mc = (insight as { matchConfidence?: string }).matchConfidence;
    return mc === 'high' ? 'observed' : 'inferred';
  }
  if (type === 'capability' || type === 'rbac_risk') return 'inferred';
  return 'estimated';
}

/** Build a provenance title for an insight. */
export function insightProvenanceTitle(insight: Insight): string {
  const k = insightProvenance(insight);
  const parts = [`Provenance: ${k}`];
  const conf = (insight as { finalRiskConfidence?: string }).finalRiskConfidence;
  if (conf) parts.push(`confidence ${conf}`);
  if ((insight as { degraded?: boolean }).degraded) parts.push('degraded scoring');
  return parts.join(' · ');
}

/** Classify an attack path edge's provenance based on its type name. */
export function edgeProvenance(edgeType: string | undefined): ProvenanceKind {
  const t = String(edgeType || '').toUpperCase().trim();
  if (!t) return 'estimated';
  if (t === 'NETWORK_REACH' || t === 'HAS_ATTACK_STEP') return 'observed';
  if (t === 'NETWORK_REACH_SOFT') return 'inferred';
  if (t.includes('SOFT') || t.includes('INFER')) return 'inferred';
  if (t === 'RBAC_BINDING' || t === 'GRANTS_ROLE' || t === 'SERVICE_ACCOUNT_ACCESS') return 'theoretical';
  if (t === 'CONTAINER_ESCAPE' || t === 'HOST_ACCESS' || t === 'LATERAL_MOVE') return 'inferred';
  return 'estimated';
}

/** Get a human-readable label for how severity counts were computed. */
export function severityCountTrustLabel(trust: 'exact' | 'sample' | 'page'): string {
  switch (trust) {
    case 'exact':
      return 'Exact counts (summary API)';
    case 'sample':
      return 'Sampled counts (≤250 rows)';
    case 'page':
      return 'Page-only counts (degraded)';
    default:
      return trust;
  }
}
