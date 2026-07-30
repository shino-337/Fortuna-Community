import type { GraphSemanticMode } from './persona';
import { edgeProvenance } from './provenance';
import { GRAPH_EDGE_COLORS, GRAPH_THEME } from './graphTheme';

export interface GraphEdgeVisual {
  stroke: string;
  strokeWidth: number;
  strokeOpacity: number;
  strokeDasharray: string | null;
}

function baseStroke(edgeType: string | undefined): string {
  const t = String(edgeType || '').toUpperCase().trim();
  return GRAPH_EDGE_COLORS[t] || GRAPH_THEME.muted2;
}

/** Persona-specific edge rendering (Phase 4). */
export function graphEdgeVisual(
  mode: GraphSemanticMode,
  edgeType: string | undefined,
  hasStepIndex?: boolean,
): GraphEdgeVisual {
  const prov = edgeProvenance(edgeType);
  const stroke = baseStroke(edgeType);

  if (mode === 'integrity') {
    return {
      stroke: prov === 'observed' ? stroke : prov === 'inferred' ? GRAPH_THEME.muted : GRAPH_THEME.muted2,
      strokeWidth: prov === 'observed' ? 2.4 : 1.2,
      strokeOpacity: prov === 'observed' ? 0.95 : 0.45,
      strokeDasharray: prov === 'observed' ? null : '6 4',
    };
  }

  if (mode === 'exploitability') {
    const t = String(edgeType || '').toUpperCase();
    const hot =
      t === 'GRANTS_ROLE' ||
      t === 'RBAC_BINDING' ||
      t === 'CONTAINER_ESCAPE' ||
      t === 'HOST_ACCESS' ||
      t === 'HAS_ATTACK_STEP';
    return {
      stroke: hot ? stroke : GRAPH_THEME.muted2,
      strokeWidth: hot || hasStepIndex ? 2.5 : 1.1,
      strokeOpacity: hot ? 0.9 : 0.4,
      strokeDasharray: prov === 'theoretical' ? '4 3' : null,
    };
  }

  // blast_radius — emphasize high-impact paths, de-emphasize soft reach
  const t = String(edgeType || '').toUpperCase();
  const narrative =
    t === 'GRANTS_ROLE' || t === 'HOST_ACCESS' || t === 'CONTAINER_ESCAPE' || t === 'NETWORK_REACH';
  return {
    stroke,
    strokeWidth: narrative ? 2.2 : 1,
    strokeOpacity: t === 'NETWORK_REACH_SOFT' ? 0.35 : narrative ? 0.85 : 0.55,
    strokeDasharray: t === 'NETWORK_REACH_SOFT' ? '5 4' : null,
  };
}

/** Get the maximum node label size for a given mode and container width. */
export function graphNodeLabelMax(mode: GraphSemanticMode, width: number): number {
  if (mode === 'blast_radius') return width < 480 ? 14 : 20;
  if (mode === 'integrity') return width < 480 ? 10 : 16;
  return width < 480 ? 12 : 18;
}
