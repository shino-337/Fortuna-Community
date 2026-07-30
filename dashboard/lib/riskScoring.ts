/**
 * Shared risk scoring utilities — single source of truth for
 * risk bands, dimension analysis, and pod score resolution.
 *
 * Previously duplicated between Dashboard.tsx and PodDetail.tsx.
 */
import { deriveUnifiedRiskLevelFromScore, type SeverityLevel } from './severity';
import type { PodWithRisk, UnifiedRiskScore } from '../types';

/* ─── types ──────────────────────────────────────────────── */

export type RiskBand = 'critical' | 'high' | 'medium' | 'low';

/** Capability knowledge base for explainable scoring */
export const CAPABILITY_KNOWLEDGE: Record<string, { explanation: string; riskPoints: number }> = {
  SA_TOKEN: { explanation: 'Can access the Kubernetes API with service account credentials.', riskPoints: 1.2 },
  SERVICE_ACCOUNT_ACCESS: { explanation: 'Can use Kubernetes service account credentials.', riskPoints: 1.2 },
  CLUSTER_ADMIN: { explanation: 'Full cluster takeover through cluster-wide control.', riskPoints: 2 },
  NODE_SHELL_ACCESS: { explanation: 'Can interact with host or node filesystem/process space.', riskPoints: 1.5 },
  HOST_ACCESS: { explanation: 'Can touch host-level surfaces from the workload.', riskPoints: 1.5 },
  NETWORK_ACCESS: { explanation: 'Can reach another workload or control plane endpoint.', riskPoints: 0.65 },
};

/** Edge-type to MITRE technique category mapping */
export const EDGE_TO_TECHNIQUE_CATEGORIES: Record<string, string[]> = {
  ESC_HOSTPATH_NODE: ['ESCAPE_HOSTPATH'],
  ESC_HOSTPID: ['ESCAPE_HOSTPID'],
  ESC_PRIV_POD: ['ESCAPE_PRIVILEGED'],
  CONTAINER_ESCAPE: ['ESCAPE_HOSTPATH', 'ESCAPE_RUNTIME', 'ESCAPE_PROC_ROOT', 'ESCAPE_PRIVILEGED', 'ESCAPE_HOSTPID'],
  HOST_ACCESS: ['ESCAPE_HOSTPATH', 'ESCAPE_RUNTIME', 'ESCAPE_PROC_ROOT', 'ESCAPE_PRIVILEGED', 'ESCAPE_HOSTPID'],
  LATERAL_MOVE: ['LATERAL_NETWORK'],
  NETWORK_REACH: ['LATERAL_NETWORK'],
  NETWORK_REACH_SOFT: ['LATERAL_NETWORK'],
  SERVICE_ACCOUNT_ACCESS: ['KUBELET_TOKEN_HARVEST', 'RUNTIME_TOKEN_HARVEST', 'SA_TOKEN_REUSE'],
  RBAC_BINDING: ['RBAC_PRIV_ESC', 'CLUSTER_ADMIN_ESC'],
  GRANTS_ROLE: ['RBAC_PRIV_ESC', 'CLUSTER_ADMIN_ESC'],
  CAN_STEAL_CREDENTIALS: ['KUBELET_API_PROBE', 'KUBELET_TOKEN_HARVEST', 'RUNTIME_TOKEN_HARVEST'],
};

export const IMPACT_RANK: Record<string, number> = {
  CRITICAL: 4, HIGH: 3, MEDIUM: 2, LOW: 1,
};

export const RISK_RANK: Record<string, number> = {
  CRITICAL: 4, HIGH: 3, MEDIUM: 2, LOW: 1,
};

/* ─── scoring functions ──────────────────────────────────── */

/**
 * Identify the top contributing dimension from a unified risk score.
 * Returns the human-readable label and raw value.
 */
export function topContributingDimension(
  dim: UnifiedRiskScore['dimensions'],
): { short: string; value: number } {
  const pairs: [string, number][] = [
    ['Vuln', dim.vulnerability ?? 0],
    ['Capability', dim.capabilityExposure ?? 0],
    ['Attack path', dim.attackPath ?? 0],
    ['RBAC', dim.rbacPolicy ?? 0],
    ['Runtime', dim.runtimeThreat ?? 0],
    ['Exposure', dim.exposure ?? 0],
    ['Blast', dim.blastRadius ?? 0],
  ];
  let best = pairs[0];
  for (const p of pairs) {
    if (p[1] > best[1]) best = p;
  }
  return { short: best[0], value: best[1] };
}

/**
 * Resolve the numeric risk score from a pod or unified risk score object.
 * Handles both `totalScore` and `unifiedScore` field shapes.
 */
export function scoreForPod(pod: PodWithRisk | UnifiedRiskScore): number | undefined {
  const raw = 'totalScore' in pod ? pod.totalScore : undefined;
  const value =
    typeof raw === 'number'
      ? raw
      : 'unifiedScore' in pod && typeof pod.unifiedScore === 'number'
        ? pod.unifiedScore
        : undefined;
  return value != null && Number.isFinite(value) ? value : undefined;
}

/** Derive the canonical user-facing risk band from the unified score. */
export function finalRiskBandFromScore(score?: number): RiskBand {
  return deriveUnifiedRiskLevelFromScore(score) ?? 'low';
}

/** Normalize an API finalLevel or numeric unified score into a RiskBand. Do not pass evidence severity here. */
export function riskBandFrom(value?: string, score?: number): RiskBand {
  const raw = String(value || '').toLowerCase();
  if (raw === 'critical' || raw === 'high' || raw === 'medium' || raw === 'low') return raw;
  return finalRiskBandFromScore(score);
}

/** Get an uppercase risk band label (e.g., "CRITICAL"). */
export function riskLabelUpper(value?: string, score?: number): string {
  return riskBandFrom(value, score).toUpperCase();
}

/** Get a title-cased risk band label (e.g., "Critical"). */
export function riskLabelTitle(value?: string, score?: number): string {
  const label = riskBandFrom(value, score);
  return label.charAt(0).toUpperCase() + label.slice(1);
}

/** Get a numeric sort key for an impact label. */
export function rankImpact(value?: string): number {
  return IMPACT_RANK[String(value || '').toUpperCase()] ?? 0;
}

/** Get a numeric sort key for a risk label. */
export function rankRisk(value?: string, score?: number): number {
  return RISK_RANK[riskLabelUpper(value, score)] ?? 0;
}
