import type { AttackPath, AttackPathNode, AttackStep } from '../types';
import { capabilityHumanSnippet } from './attackPathNarrative';
import { identityDetailPath, inventoryPath, podDetailPath } from './entityLinks';

/** Which technique categories an edge type can carry out; maps chain steps onto path edges. */
const EDGE_TO_TECHNIQUE_CATEGORIES: Record<string, string[]> = {
  ESC_HOSTPATH_NODE: ['ESCAPE_HOSTPATH'],
  ESC_HOSTPID: ['ESCAPE_HOSTPID'],
  ESC_PRIV_POD: ['ESCAPE_PRIVILEGED'],
  CONTAINER_ESCAPE: ['ESCAPE_HOSTPATH', 'ESCAPE_RUNTIME', 'ESCAPE_PROC_ROOT', 'ESCAPE_PRIVILEGED', 'ESCAPE_HOSTPID'],
  HOST_ACCESS: ['ESCAPE_HOSTPATH', 'ESCAPE_RUNTIME', 'ESCAPE_PROC_ROOT', 'ESCAPE_PRIVILEGED', 'ESCAPE_HOSTPID'],
  LATERAL_MOVE: ['LATERAL_NETWORK'],
  NETWORK_REACH: ['LATERAL_NETWORK'],
  NETWORK_REACH_SOFT: ['LATERAL_NETWORK'],
  SERVICE_ACCOUNT_ACCESS: ['KUBELET_TOKEN_HARVEST', 'RUNTIME_TOKEN_HARVEST', 'SA_TOKEN_REUSE'],
  USES_SERVICE_ACCOUNT: ['KUBELET_TOKEN_HARVEST', 'RUNTIME_TOKEN_HARVEST', 'SA_TOKEN_REUSE'],
  RBAC_BINDING: ['RBAC_PRIV_ESC', 'CLUSTER_ADMIN_ESC'],
  GRANTS_ROLE: ['RBAC_PRIV_ESC', 'CLUSTER_ADMIN_ESC'],
  CAN_STEAL_CREDENTIALS: ['KUBELET_API_PROBE', 'KUBELET_TOKEN_HARVEST', 'RUNTIME_TOKEN_HARVEST'],
};

/** 1-based index of the first chain step this edge type carries out, if any. */
export function techniqueStepIndex(edgeType: string, steps?: AttackStep[] | null): number | undefined {
  if (!steps || steps.length === 0) return undefined;
  const categories = EDGE_TO_TECHNIQUE_CATEGORIES[String(edgeType || '').toUpperCase()];
  if (!categories) return undefined;
  for (let i = 0; i < steps.length; i++) {
    if (categories.includes(steps[i].technique_id)) return i + 1;
  }
  return undefined;
}

/** Node type as the backend writes it (`service_account`), whatever casing it arrived in. */
export function nodeKind(node: Pick<AttackPathNode, 'type'> | null | undefined): string {
  return String(node?.type || '')
    .replace(/([a-z])([A-Z])/g, '$1_$2')
    .toLowerCase();
}

export const NODE_KIND_LABEL: Record<string, string> = {
  pod: 'Pod',
  service_account: 'Service account',
  role_binding: 'Role binding',
  cluster_role_binding: 'Cluster role binding',
  role: 'Role',
  cluster_role: 'Cluster role',
  node: 'Node',
  capability: 'Capability',
  attack_step: 'Attack step',
  secret: 'Secret',
  external: 'External',
};

const CLUSTER_SCOPED = new Set(['node', 'capability', 'cluster_role', 'cluster_role_binding', 'external']);

/** `namespace/name` for namespaced objects, `name` for cluster-wide ones. */
export function nodeDisplayName(node: AttackPathNode | null | undefined): string {
  if (!node) return '';
  const name = String(node.properties?.name || node.id);
  const ns = CLUSTER_SCOPED.has(nodeKind(node)) ? '' : String(node.properties?.namespace || '');
  return ns ? `${ns}/${name}` : name;
}

const IDENTITY_KIND: Record<string, string> = {
  role: 'Role',
  cluster_role: 'ClusterRole',
  role_binding: 'Binding',
  cluster_role_binding: 'Binding',
};

/** The page that shows this object, or null when there is none (or the viewer cannot open Inventory). */
export function nodeHref(node: AttackPathNode, clusterId: string | null, canInventory: boolean): string | null {
  if (!canInventory) return null;
  const kind = nodeKind(node);
  if (kind === 'pod') return podDetailPath(node.id, clusterId ?? undefined);
  if (kind === 'service_account') return identityDetailPath({ uid: node.id, clusterId });
  if (IDENTITY_KIND[kind]) {
    return inventoryPath('identities', {
      clusterId,
      kind: IDENTITY_KIND[kind],
      namespace: CLUSTER_SCOPED.has(kind) ? undefined : String(node.properties?.namespace || '') || undefined,
      search: String(node.properties?.name || ''),
    });
  }
  if (kind === 'node' && clusterId) {
    return `/clusters/${encodeURIComponent(clusterId)}/nodes/${encodeURIComponent(String(node.properties?.name || node.id))}`;
  }
  return null;
}

export interface PathHop {
  node: AttackPathNode;
  /** What the attacker does to reach this object; empty for the entry pod. */
  action: string;
  /** 1-based chain step carried out on the way in, when one maps. */
  step?: number;
  observed: boolean;
}

/**
 * The objects of one path in order, each with the move that reaches it. Step numbers follow `steps` (the
 * scenario's chain); "seen at runtime" follows `ownSteps`, the chain variant this path belongs to.
 */
export function pathHops(path: AttackPath, steps?: AttackStep[] | null, ownSteps?: AttackStep[] | null): PathHop[] {
  const own = Array.isArray(ownSteps) ? ownSteps : Array.isArray(steps) ? steps : [];
  const seenAtRuntime = (techniqueId?: string) => Boolean(techniqueId && own.some((s) => s.technique_id === techniqueId && s.runtime_observed));
  const nodes = Array.isArray(path.nodes) ? path.nodes : [];
  const edges = Array.isArray(path.edges) ? path.edges : [];
  const stepList = Array.isArray(steps) ? steps : [];
  const usedSteps = new Set<number>();
  const hops: PathHop[] = nodes.map((node, i) => {
    if (i === 0) return { node, action: '', observed: false };
    const edge = edges.find((e) => e.target === node.id && e.source === nodes[i - 1].id) ?? edges.find((e) => e.target === node.id);
    const step = edge ? techniqueStepIndex(edge.type, stepList) : undefined;
    if (step) usedSteps.add(step);
    const stepInfo = step ? stepList[step - 1] : undefined;
    return {
      node,
      action: stepInfo?.name || (edge ? capabilityHumanSnippet(edge.type) : ''),
      step,
      observed: seenAtRuntime(stepInfo?.technique_id),
    };
  });
  // A leading step no edge carries (initial access) belongs to the entry pod.
  if (hops[0] && stepList[0] && !usedSteps.has(1)) {
    hops[0] = { ...hops[0], action: stepList[0].name || capabilityHumanSnippet(stepList[0].technique_id), step: 1, observed: seenAtRuntime(stepList[0].technique_id) };
  }
  return hops;
}

export type TargetGroup = 'cluster-admin' | 'roles' | 'nodes' | 'secrets' | 'workloads' | 'other';

export const TARGET_GROUP_LABEL: Record<TargetGroup, string> = {
  'cluster-admin': 'cluster-admin',
  roles: 'Roles',
  nodes: 'Nodes',
  secrets: 'Secrets',
  workloads: 'Workloads',
  other: 'Other',
};

/** What kind of thing a path ends at, grouped the way a reader asks "what is at stake". */
export function targetGroupOf(node: AttackPathNode | null | undefined, fallbackName = ''): TargetGroup {
  const kind = nodeKind(node);
  const name = String(node?.properties?.name || fallbackName).toLowerCase();
  if (name === 'cluster-admin' || name.endsWith('/cluster-admin')) return 'cluster-admin';
  if (kind === 'role' || kind === 'cluster_role') return 'roles';
  if (kind === 'node') return 'nodes';
  if (kind === 'secret' || name.includes('secret')) return 'secrets';
  if (kind === 'pod') return 'workloads';
  return node ? 'other' : name.includes('cluster-admin') ? 'cluster-admin' : 'other';
}
