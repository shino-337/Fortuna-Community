import type { PodWithRisk, ResourceRiskSignals } from '../types';

export type PodPathRole = 'entry' | 'pivot' | 'target';

export type PodQuickFilters = {
  attackPath: boolean;
  entryOnly: boolean;
  highRisk: boolean;
  new24h: boolean;
};

export function podPathRole(sig: ResourceRiskSignals | undefined): PodPathRole | null {
  if (!sig?.hasAttackPath) return null;
  if (sig.isEntryPoint) return 'entry';
  if (sig.isPivot) return 'pivot';
  return 'target';
}

export function roleLabel(r: PodPathRole | null): string {
  if (r === 'entry') return 'Entry point';
  if (r === 'pivot') return 'Pivot';
  if (r === 'target') return 'Target';
  return '—';
}

export function impactLines(sig: ResourceRiskSignals | undefined): string[] {
  if (!sig?.hasAttackPath) return [];
  const mi = (sig.maxImpact || '').toUpperCase();
  const out: string[] = [];
  if (mi === 'CRITICAL') {
    out.push('Node / cluster compromise');
    out.push('Privilege escalation');
  } else if (mi === 'HIGH') {
    out.push('Strong lateral movement');
    out.push('Elevated blast radius');
  } else if (mi === 'MEDIUM') {
    out.push('Scoped lateral movement');
  } else {
    out.push('Limited path impact');
  }
  return out;
}

export function exploitHint(sig: ResourceRiskSignals | undefined): { external: string; auth: string } {
  if (sig?.isEntryPoint) {
    return { external: 'Often YES (entry)', auth: 'Varies' };
  }
  if (sig?.hasAttackPath) {
    return { external: 'Indirect', auth: 'Often YES (chain)' };
  }
  return { external: 'Not indicated', auth: '—' };
}

export function formatPodAge(iso?: string | null): string | null {
  if (!iso) return null;
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t)) return null;
  const sec = Math.floor((Date.now() - t) / 1000);
  if (sec < 0) return null;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
  return `${Math.floor(sec / 86400)}d ago`;
}

export function filterPods(
  list: PodWithRisk[],
  quick: PodQuickFilters,
  minScore: number | null,
  role: PodPathRole | '',
): PodWithRisk[] {
  const now = Date.now();
  return list.filter((p) => {
    const sig = p.riskSignals;
    if (quick.attackPath && !sig?.hasAttackPath) return false;
    if (quick.entryOnly && !sig?.isEntryPoint) return false;
    if (quick.highRisk) {
      const lvl = (p.finalLevel || '').toLowerCase();
      const sc = Number(p.unifiedScore ?? p.totalScore ?? 0);
      if (lvl !== 'critical' && lvl !== 'high' && !(Number.isFinite(sc) && sc >= 70)) return false;
    }
    if (quick.new24h) {
      if (!p.createdAt) return false;
      const t = new Date(p.createdAt).getTime();
      if (!Number.isFinite(t) || now - t > 24 * 3600 * 1000) return false;
    }
    if (minScore != null && minScore > 0) {
      const sc = Number(p.unifiedScore ?? p.totalScore ?? 0);
      if (!Number.isFinite(sc) || sc < minScore) return false;
    }
    if (role) {
      const pr = podPathRole(sig);
      if (pr !== role) return false;
    }
    return true;
  });
}

export function topRiskNodesFromPods(pods: PodWithRisk[], limit = 3): { node: string; maxScore: number; count: number }[] {
  const byNode = new Map<string, { max: number; count: number }>();
  for (const p of pods) {
    const node = (p.nodeName || '').trim() || '(unscheduled)';
    const sc = Number(p.unifiedScore ?? p.totalScore ?? 0) || 0;
    const cur = byNode.get(node) || { max: 0, count: 0 };
    cur.count += 1;
    cur.max = Math.max(cur.max, sc);
    byNode.set(node, cur);
  }
  return [...byNode.entries()]
    .sort((a, b) => b[1].max - a[1].max)
    .slice(0, limit)
    .map(([node, v]) => ({ node, maxScore: v.max, count: v.count }));
}
