/**
 * Build a canonical Pod Detail route without guessing ownership.
 *
 * Carry cluster ownership whenever the caller already knows it. If ownership is
 * unknown, keep the route UID-only so Core can fail closed for an ambiguous UID.
 */
export function podDetailPath(podUid: string, clusterId?: string | null): string {
  const uid = String(podUid ?? '').trim();
  const cluster = String(clusterId ?? '').trim();
  const base = `/resources/pods/uid/${encodeURIComponent(uid)}`;
  return cluster ? `${base}?clusterId=${encodeURIComponent(cluster)}` : base;
}
