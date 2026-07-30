/**
 * Display name for clusters: always show the actual cluster name from K8s (via agent sync).
 * Query/filter/URL use cluster_id (hash); UI labels use this display name.
 * When name is missing (legacy), show a short label instead of raw hash.
 */
export interface ClusterLike {
  id: string;
  name?: string | null;
}

/** Returns the label to show in UI (from K8s name when available); never raw hash as primary. */
export function getClusterDisplayName(cluster: ClusterLike): string {
  const name = (cluster.name ?? '').trim();
  if (name && name !== cluster.id) return name;
  const id = cluster.id ?? '';
  if (id.startsWith('sha256-') && id.length > 15) return `Cluster ${id.slice(7, 15)}…`;
  return id || 'Cluster';
}
