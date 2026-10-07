import type { NetworkActivityConnectionRow, NetworkActivityDestinationRow } from '../../types';

/** One pod → destination group from `view=edges`. */
export type FlowRow = NetworkActivityConnectionRow & { lastObservedAt?: string };

/** What the source pod is, from GET /inventory/pods: name and how many open findings it has. */
export interface PodFacts {
  name: string;
  namespace: string;
  riskCount: number;
  finalLevel?: string;
  score?: number;
}

export type DestKind = 'service' | 'pod' | 'internal' | 'external';

type DestFields = Pick<NetworkActivityDestinationRow, 'destIp' | 'destServiceName' | 'destServiceNamespace' | 'destWorkloadName' | 'destWorkloadNamespace'>;

const PRIVATE_V4 = [
  [10, 0, 8],
  [172, 16, 12],
  [192, 168, 16],
  [127, 0, 8],
  [169, 254, 16],
  [100, 64, 10],
] as const;

/** RFC 1918, loopback, link-local and CGNAT addresses stay inside the cluster's network. */
export function isPrivateIp(ip: string): boolean {
  const v = ip.trim().toLowerCase();
  if (v.includes(':')) return v === '::1' || v.startsWith('fc') || v.startsWith('fd') || v.startsWith('fe80');
  const parts = v.split('.').map(Number);
  if (parts.length !== 4 || parts.some((n) => !Number.isInteger(n) || n < 0 || n > 255)) return false;
  const addr = ((parts[0] << 24) >>> 0) + (parts[1] << 16) + (parts[2] << 8) + parts[3];
  return PRIVATE_V4.some(([a, b, bits]) => {
    const base = ((a << 24) >>> 0) + (b << 16);
    const mask = (~0 << (32 - bits)) >>> 0;
    return (addr & mask) >>> 0 === (base & mask) >>> 0;
  });
}

export function destKind(d: DestFields): DestKind {
  if (d.destServiceName) return 'service';
  if (d.destWorkloadName) return 'pod';
  return isPrivateIp(d.destIp || '') ? 'internal' : 'external';
}

/** A destination as people know it: the service or pod name when known, never a bare ClusterIP. */
export function destLabel(d: DestFields): { name: string; detail: string } {
  if (d.destServiceName) {
    return { name: d.destServiceName, detail: [d.destServiceNamespace, d.destIp].filter(Boolean).join(' · ') };
  }
  if (d.destWorkloadName) {
    return { name: d.destWorkloadName, detail: [d.destWorkloadNamespace, d.destIp].filter(Boolean).join(' · ') };
  }
  return { name: d.destIp || '—', detail: '' };
}

/** Ports any workload is expected to use towards the internet. */
const COMMON_EXTERNAL_PORTS = new Set([53, 80, 123, 443]);

export interface FlowFlag {
  reasons: string[];
  /** Higher sorts first. */
  weight: number;
}

/**
 * Why a flow deserves a look. Only uses data the page has: the destination kind and port, and
 * whether the source pod already has open findings. Denied connections are not reported by Core.
 */
export function flagFlow(row: FlowRow, pod: PodFacts | undefined): FlowFlag | null {
  const reasons: string[] = [];
  let weight = 0;
  const kind = destKind({ destIp: row.destIp ?? '', destServiceName: row.destServiceName, destWorkloadName: row.destWorkloadName });
  const port = Number(row.destPort ?? 0);
  if (kind === 'external' && port > 0 && !COMMON_EXTERNAL_PORTS.has(port)) {
    reasons.push(`external on port ${port}`);
    weight += 2;
  }
  if (kind === 'external' && pod && pod.riskCount > 0) {
    reasons.push(`source has ${pod.riskCount} open ${pod.riskCount === 1 ? 'finding' : 'findings'}`);
    weight += 3;
  }
  if (reasons.length === 0) return null;
  return { reasons, weight };
}

export function flowKey(row: Pick<FlowRow, 'podUid' | 'destIp' | 'destPort' | 'protocol'>): string {
  return `${row.podUid ?? ''}|${row.destIp ?? ''}|${row.destPort ?? ''}|${(row.protocol ?? '').toLowerCase()}`;
}

export { relativeTime } from '../../lib/time';

export const SINCE_OPTIONS = [
  { value: 15, label: 'Last 15 min' },
  { value: 60, label: 'Last hour' },
  { value: 360, label: 'Last 6 hours' },
  { value: 1440, label: 'Last 24 hours' },
] as const;
