/**
 * Native graph rendering styles from trust semantics.
 */
import type { EdgeTrustKind, NodeTrustKind } from './graphTrustSemantics';
import { graphEdgeVisual } from './graphSemantics';
import type { GraphSemanticMode } from './persona';
import { GRAPH_THEME } from './graphTheme';

export interface EdgeTrustVisual {
  stroke: string;
  strokeWidth: number;
  strokeOpacity: number;
  strokeDasharray: string | null;
  markerOpacity: number;
  trustKind: EdgeTrustKind;
}

export interface NodeTrustVisual {
  fill: string;
  stroke: string;
  strokeWidth: number;
  fillOpacity: number;
  strokeDasharray: string | null;
  glowColor: string | null;
  trustKind: NodeTrustKind;
}

const EDGE_TRUST_STYLE: Record<EdgeTrustKind, Omit<EdgeTrustVisual, 'trustKind'>> = {
  observed_runtime: { stroke: GRAPH_THEME.success, strokeWidth: 2.8, strokeOpacity: 0.95, strokeDasharray: null, markerOpacity: 1 },
  exploit_chain: { stroke: GRAPH_THEME.high, strokeWidth: 2.6, strokeOpacity: 0.92, strokeDasharray: null, markerOpacity: 1 },
  network_policy: { stroke: GRAPH_THEME.success, strokeWidth: 2, strokeOpacity: 0.75, strokeDasharray: '2 2', markerOpacity: 0.9 },
  identity_permission: { stroke: GRAPH_THEME.warning, strokeWidth: 2.2, strokeOpacity: 0.88, strokeDasharray: null, markerOpacity: 1 },
  blast_radius_dependency: { stroke: GRAPH_THEME.critical, strokeWidth: 2.4, strokeOpacity: 0.9, strokeDasharray: null, markerOpacity: 1 },
  inferred: { stroke: GRAPH_THEME.muted, strokeWidth: 1.4, strokeOpacity: 0.55, strokeDasharray: '6 4', markerOpacity: 0.7 },
  sampled: { stroke: GRAPH_THEME.muted2, strokeWidth: 1.2, strokeOpacity: 0.4, strokeDasharray: '3 5', markerOpacity: 0.5 },
  stale: { stroke: GRAPH_THEME.warning, strokeWidth: 1.6, strokeOpacity: 0.5, strokeDasharray: '8 4', markerOpacity: 0.6 },
  hidden_by_policy: { stroke: GRAPH_THEME.border, strokeWidth: 1, strokeOpacity: 0.25, strokeDasharray: '2 6', markerOpacity: 0.3 },
  hidden_by_scope: { stroke: GRAPH_THEME.border, strokeWidth: 1, strokeOpacity: 0.2, strokeDasharray: '1 8', markerOpacity: 0.25 },
};

const NODE_TRUST_STYLE: Record<NodeTrustKind, Omit<NodeTrustVisual, 'trustKind'>> = {
  runtime_confirmed: {
    fill: GRAPH_THEME.brand,
    stroke: GRAPH_THEME.success,
    strokeWidth: 3,
    fillOpacity: 1,
    strokeDasharray: null,
    glowColor: GRAPH_THEME.success,
  },
  crown_jewel: {
    fill: GRAPH_THEME.violet,
    stroke: GRAPH_THEME.warning,
    strokeWidth: 3,
    fillOpacity: 1,
    strokeDasharray: null,
    glowColor: GRAPH_THEME.warning,
  },
  latent_exposure: {
    fill: GRAPH_THEME.high,
    stroke: GRAPH_THEME.high,
    strokeWidth: 2,
    fillOpacity: 0.85,
    strokeDasharray: '4 3',
    glowColor: GRAPH_THEME.high,
  },
  stale_telemetry: {
    fill: GRAPH_THEME.muted2,
    stroke: GRAPH_THEME.warning,
    strokeWidth: 2,
    fillOpacity: 0.7,
    strokeDasharray: '5 3',
    glowColor: null,
  },
  degraded_visibility: {
    fill: GRAPH_THEME.border,
    stroke: GRAPH_THEME.muted,
    strokeWidth: 1.5,
    fillOpacity: 0.65,
    strokeDasharray: '3 3',
    glowColor: null,
  },
  partial_inventory: {
    fill: GRAPH_THEME.muted2,
    stroke: GRAPH_THEME.border,
    strokeWidth: 1.5,
    fillOpacity: 0.8,
    strokeDasharray: null,
    glowColor: null,
  },
  sampled_cluster: {
    fill: GRAPH_THEME.info,
    stroke: GRAPH_THEME.cyan,
    strokeWidth: 2,
    fillOpacity: 0.75,
    strokeDasharray: '2 4',
    glowColor: null,
  },
  hidden_descendants: {
    fill: GRAPH_THEME.border,
    stroke: GRAPH_THEME.surfaceDeep,
    strokeWidth: 1,
    fillOpacity: 0.5,
    strokeDasharray: '1 4',
    glowColor: null,
  },
};

export function edgeTrustVisual(
  trustKind: EdgeTrustKind,
  semanticMode: GraphSemanticMode,
  edgeType: string | undefined,
  hasStepIndex?: boolean,
): EdgeTrustVisual {
  const base = EDGE_TRUST_STYLE[trustKind];
  const persona = graphEdgeVisual(semanticMode, edgeType, hasStepIndex);
  const hot = trustKind === 'observed_runtime' || trustKind === 'exploit_chain' || trustKind === 'blast_radius_dependency';
  return {
    ...base,
    stroke: hot ? base.stroke : persona.stroke,
    strokeWidth: Math.max(base.strokeWidth, persona.strokeWidth),
    strokeOpacity: Math.min(base.strokeOpacity, persona.strokeOpacity + (hot ? 0.1 : 0)),
    strokeDasharray: base.strokeDasharray ?? persona.strokeDasharray,
    trustKind,
  };
}

export function nodeTrustVisual(trustKind: NodeTrustKind, fallbackFill: string, fallbackStroke: string): NodeTrustVisual {
  const base = NODE_TRUST_STYLE[trustKind];
  return {
    ...base,
    fill: fallbackFill,
    stroke: base.stroke,
    trustKind,
  };
}
