/**
 * Graph-native trust semantics — classifies nodes/edges for truthful rendering.
 */
import type { GraphSemanticMode } from './persona';
import type { TelemetryContext } from './telemetryContext';
import type { SemanticVisibilityState } from './visibilityEngine';
import { edgeProvenance } from './provenance';

export type EdgeTrustKind =
  | 'observed_runtime'
  | 'inferred'
  | 'sampled'
  | 'stale'
  | 'hidden_by_policy'
  | 'hidden_by_scope'
  | 'exploit_chain'
  | 'network_policy'
  | 'identity_permission'
  | 'blast_radius_dependency';

export type NodeTrustKind =
  | 'runtime_confirmed'
  | 'latent_exposure'
  | 'stale_telemetry'
  | 'degraded_visibility'
  | 'partial_inventory'
  | 'crown_jewel'
  | 'sampled_cluster'
  | 'hidden_descendants';

export interface GraphTrustPosture {
  completeness: 'full' | 'sampled' | 'policy_limited' | 'scope_limited' | 'degraded';
  telemetryFresh: boolean;
  runtimeEvidenceAvailable: boolean;
  hiddenEdgeCount: number;
  sampled: boolean;
  policyLimited: boolean;
  stale: boolean;
  summary: string;
}

export interface GraphTrustContext {
  semanticMode: GraphSemanticMode;
  visibilityState?: SemanticVisibilityState;
  telemetry?: TelemetryContext;
  clustered?: boolean;
  crownJewelIds?: Set<string>;
  runtimeConfirmedNodeIds?: Set<string>;
}

export function classifyEdgeTrust(
  edgeType: string | undefined,
  ctx: GraphTrustContext,
  hasStepIndex?: boolean,
): EdgeTrustKind {
  const t = String(edgeType || '').toUpperCase().trim();
  const prov = edgeProvenance(edgeType);

  if (ctx.visibilityState === 'policy_hidden' || ctx.telemetry?.graphPolicyLimited) {
    return 'hidden_by_policy';
  }
  if (ctx.visibilityState === 'no_scope') {
    return 'hidden_by_scope';
  }
  if (ctx.clustered || ctx.telemetry?.graphSampled) {
    return 'sampled';
  }
  if (ctx.telemetry?.ingestionStale || ctx.telemetry?.pipelineDegraded) {
    return 'stale';
  }

  if (
    t === 'HAS_ATTACK_STEP' ||
    t === 'CONTAINER_ESCAPE' ||
    t === 'HOST_ACCESS' ||
    t === 'LATERAL_MOVE' ||
    (hasStepIndex && prov === 'observed')
  ) {
    return 'observed_runtime';
  }

  if (t === 'NETWORK_REACH' || t === 'NETWORK_REACH_SOFT') {
    return 'network_policy';
  }

  if (t === 'RBAC_BINDING' || t === 'GRANTS_ROLE' || t === 'SERVICE_ACCOUNT_ACCESS') {
    return 'identity_permission';
  }

  if (ctx.semanticMode === 'blast_radius' && (t === 'GRANTS_ROLE' || t === 'HOST_ACCESS' || t === 'NETWORK_REACH')) {
    return 'blast_radius_dependency';
  }

  if (hasStepIndex || t === 'HAS_ATTACK_STEP') {
    return 'exploit_chain';
  }

  if (prov === 'inferred' || prov === 'theoretical') {
    return 'inferred';
  }

  return prov === 'observed' ? 'observed_runtime' : 'inferred';
}

export function classifyNodeTrust(
  node: { id: string; type?: string; risk?: string; properties?: Record<string, unknown> },
  ctx: GraphTrustContext,
): NodeTrustKind {
  if (ctx.crownJewelIds?.has(node.id)) return 'crown_jewel';
  if (ctx.runtimeConfirmedNodeIds?.has(node.id)) return 'runtime_confirmed';
  if (ctx.clustered) return 'sampled_cluster';
  if (ctx.telemetry?.ingestionStale || ctx.telemetry?.pipelineDegraded) return 'stale_telemetry';
  if (ctx.visibilityState === 'policy_hidden' || ctx.telemetry?.graphPolicyLimited) {
    return 'degraded_visibility';
  }
  const risk = String(node.risk || '').toLowerCase();
  if (risk === 'critical' || risk === 'high') {
    return ctx.runtimeConfirmedNodeIds?.has(node.id) ? 'runtime_confirmed' : 'latent_exposure';
  }
  return 'partial_inventory';
}

export function buildGraphTrustPosture(ctx: GraphTrustContext, nodeCount: number, linkCount: number): GraphTrustPosture {
  const sampled = Boolean(ctx.clustered || ctx.telemetry?.graphSampled);
  const policyLimited = Boolean(ctx.visibilityState === 'policy_hidden' || ctx.telemetry?.graphPolicyLimited);
  const stale = Boolean(ctx.telemetry?.ingestionStale || ctx.telemetry?.pipelineDegraded);
  const scopeLimited = ctx.visibilityState === 'no_scope';

  let completeness: GraphTrustPosture['completeness'] = 'full';
  if (scopeLimited) completeness = 'scope_limited';
  else if (policyLimited) completeness = 'policy_limited';
  else if (sampled) completeness = 'sampled';
  else if (stale) completeness = 'degraded';

  const hiddenEdgeCount = policyLimited || scopeLimited ? Math.max(1, Math.floor(linkCount * 0.15)) : 0;

  const parts: string[] = [];
  if (completeness === 'full') parts.push('Graph reflects available relationships in your scope.');
  if (sampled) parts.push('Large graphs are clustered or sampled — not every node/edge is shown.');
  if (policyLimited) parts.push('Some relationships may be hidden by policy or query permissions.');
  if (stale) parts.push('Telemetry freshness is degraded; runtime paths may be incomplete.');
  if (scopeLimited) parts.push('Selected scope limits visible topology.');

  return {
    completeness,
    telemetryFresh: !stale,
    runtimeEvidenceAvailable: Boolean(ctx.runtimeConfirmedNodeIds?.size),
    hiddenEdgeCount,
    sampled,
    policyLimited,
    stale,
    summary: parts.join(' '),
  };
}

export function graphTrustQuestions(posture: GraphTrustPosture): { q: string; a: string }[] {
  return [
    { q: 'Is this complete?', a: posture.completeness === 'full' ? 'Yes, within scope.' : 'No — see completeness indicator.' },
    { q: 'Is this sampled?', a: posture.sampled ? 'Yes — clustered or LOD applied.' : 'No full-detail view.' },
    { q: 'Is telemetry stale?', a: posture.telemetryFresh ? 'Fresh enough for decisions.' : 'Degraded — treat exploit paths cautiously.' },
    { q: 'Are relationships hidden?', a: posture.policyLimited || posture.hiddenEdgeCount > 0 ? 'Some may be hidden.' : 'No known policy hides.' },
    { q: 'Runtime evidence?', a: posture.runtimeEvidenceAvailable ? 'Runtime-confirmed nodes present.' : 'No runtime confirmation in this view.' },
  ];
}
