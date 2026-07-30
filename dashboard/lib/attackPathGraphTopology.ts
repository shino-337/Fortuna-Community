import type { AttackPathGraphData } from '../types';

/**
 * Path rows mark the first node as START and the last node as END. Once several
 * paths are merged into one graph, a node can be both a path-local START and an
 * intermediate hop from another path. For the rendered graph, badges should
 * describe the visible topology: START means no incoming edge, END means no
 * outgoing edge.
 */
export function reconcileAttackGraphEntryExit(graph: AttackPathGraphData): AttackPathGraphData {
  if (!graph.nodes.length || !graph.links.length) return graph;

  const nodeIds = new Set(graph.nodes.map((node) => node.id));
  const incoming = new Map<string, number>();
  const outgoing = new Map<string, number>();

  graph.links.forEach((link) => {
    if (nodeIds.has(link.target)) incoming.set(link.target, (incoming.get(link.target) ?? 0) + 1);
    if (nodeIds.has(link.source)) outgoing.set(link.source, (outgoing.get(link.source) ?? 0) + 1);
  });

  const reconciled = graph.nodes.map((node) => ({
    ...node,
    isStart: Boolean(node.isStart && !incoming.has(node.id)),
    isEnd: Boolean(node.isEnd && !outgoing.has(node.id)),
  }));

  const hasStart = reconciled.some((node) => node.isStart);
  const hasEnd = reconciled.some((node) => node.isEnd);

  return {
    nodes: reconciled.map((node) => ({
      ...node,
      isStart: hasStart ? node.isStart : graph.nodes.find((original) => original.id === node.id)?.isStart,
      isEnd: hasEnd ? node.isEnd : graph.nodes.find((original) => original.id === node.id)?.isEnd,
    })),
    links: graph.links,
  };
}
