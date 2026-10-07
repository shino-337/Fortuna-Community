/**
 * Links between the pages that show one workload: pod detail, its findings, the attack paths through it
 * and its network flows. Every builder carries the cluster when the caller knows it, so following a link
 * never widens the view to every cluster or matches a same-named pod elsewhere.
 */
export { podDetailPath } from './podRoute';

interface PodRef {
  uid: string;
  clusterId?: string | null;
}

function withCluster(params: URLSearchParams, clusterId?: string | null): URLSearchParams {
  const cluster = String(clusterId ?? '').trim();
  if (cluster) params.set('clusterId', cluster);
  return params;
}

/** Findings on one resource, in every workflow state. */
export function findingsForResourcePath({ uid, clusterId }: PodRef): string {
  const params = withCluster(new URLSearchParams({ resourceUid: uid.trim(), view: 'all' }), clusterId);
  return `/risks/findings?${params.toString()}`;
}

/** Attack paths that start at a pod; `pathId` opens one path, `insightId` the path for a finding. */
export function attackPathsForPodPath({ uid, clusterId }: PodRef, opts?: { pathId?: string; insightId?: string }): string {
  const params = withCluster(new URLSearchParams({ podUid: uid.trim() }), clusterId);
  if (opts?.pathId) params.set('path', opts.pathId);
  if (opts?.insightId) params.set('insightId', opts.insightId);
  return `/attack-paths?${params.toString()}`;
}

/** Network flows to and from a pod. */
export function networkForPodPath({ uid, clusterId }: PodRef, namespace?: string): string {
  const params = withCluster(new URLSearchParams({ tab: 'connections', podUid: uid.trim() }), clusterId);
  if (namespace) params.set('namespace', namespace);
  return `/network-activity?${params.toString()}`;
}

/** Service account detail. */
export function identityDetailPath({ uid, clusterId }: PodRef): string {
  const params = withCluster(new URLSearchParams(), clusterId);
  const qs = params.toString();
  return `/identities/uid/${encodeURIComponent(uid.trim())}${qs ? `?${qs}` : ''}`;
}

export type InventoryView = 'workloads' | 'identities' | 'clusters';

/** Inventory, optionally on one view and filtered (e.g. the workloads on one node or in one namespace). */
export function inventoryPath(
  view: InventoryView = 'workloads',
  filters?: { clusterId?: string | null; namespace?: string; node?: string; kind?: string },
): string {
  const params = new URLSearchParams();
  if (view !== 'workloads') params.set('view', view);
  if (filters?.clusterId) params.set('clusterId', filters.clusterId);
  if (filters?.namespace) params.set('namespace', filters.namespace);
  if (filters?.node) params.set('node', filters.node);
  if (filters?.kind) params.set('kind', filters.kind);
  const qs = params.toString();
  return `/resources${qs ? `?${qs}` : ''}`;
}
