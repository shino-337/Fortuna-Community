import type { AttackStepSummary, Insight, InsightsSummary, RiskHistogramResponse, ThreatVelocityPoint } from '../types';
import { insightTypeUiLabel } from './riskDisplay';

export function insightTouchesAttackPath(r: Insight): boolean {
  const layers = (r.explanation_chain || []).map((s) => (s.layer || '').toLowerCase());
  if (layers.some((l) => l.includes('attack') || l.includes('path'))) return true;
  const refs = r.evidence_chain_refs || [];
  if (
    refs.some((x) => {
      const l = (x.layer || '').toLowerCase();
      const ref = (x.ref || '').toLowerCase();
      return l.includes('attack') || l.includes('path') || ref.includes('attack');
    })
  ) {
    return true;
  }
  const t = `${r.title || ''} ${r.description || ''}`.toLowerCase();
  if (t.includes('attack path') || t.includes('lateral movement') || t.includes('kill chain')) return true;
  return false;
}

export type TopRiskClusterRow = {
  key: string;
  label: string;
  count: number;
  impactLabel: string;
};

export function topRiskClusters(
  summary: InsightsSummary | null,
  risks: Insight[],
  limit = 5,
): TopRiskClusterRow[] {
  let entries: [string, number][] = [];
  if (summary?.byType && Object.keys(summary.byType).length > 0) {
    entries = Object.entries(summary.byType);
  } else {
    const m = new Map<string, number>();
    for (const r of risks) {
      const k = (r.insightType || 'unknown').trim() || 'unknown';
      m.set(k, (m.get(k) || 0) + 1);
    }
    entries = [...m.entries()];
  }
  const maxCount = Math.max(1, ...entries.map((e) => e[1]));
  return entries
    .sort((a, b) => b[1] - a[1])
    .slice(0, limit)
    .map(([key, count]) => {
      let impactLabel = 'Watch';
      if (count >= Math.max(8, maxCount * 0.35)) impactLabel = 'Critical mass';
      else if (count >= 5) impactLabel = 'High impact';
      else if (count >= 3) impactLabel = 'Elevated';
      return { key, label: insightTypeUiLabel(key), count, impactLabel };
    });
}

export function criticalAttackPathSplit(risks: Insight[]): { inPath: number; inSample: number } {
  const crit = risks.filter((r) => (r.finalLevel || '').toLowerCase() === 'critical');
  const inPath = crit.filter(insightTouchesAttackPath).length;
  return { inPath, inSample: crit.length };
}

export function velocityFromTrend(threatVelocity: ThreatVelocityPoint[]): {
  lastDelta: number;
  baselinePerDay: number;
  pctVsBaseline: number | null;
  rangeDelta: number;
} | null {
  if (threatVelocity.length < 2) return null;
  const sorted = [...threatVelocity].sort((a, b) => a.date.localeCompare(b.date));
  const totals = sorted.map((p) => (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0));
  const rangeDelta = totals[totals.length - 1] - totals[0];
  const deltas: number[] = [];
  for (let i = 1; i < totals.length; i++) deltas.push(totals[i] - totals[i - 1]);
  const lastDelta = deltas[deltas.length - 1] ?? 0;
  const baselineDeltas = deltas.length >= 2 ? deltas.slice(0, -1) : deltas;
  const baselinePerDay =
    baselineDeltas.length > 0 ? baselineDeltas.reduce((a, b) => a + b, 0) / baselineDeltas.length : 0;
  const denom =
    Math.abs(baselinePerDay) >= 0.5 ? Math.abs(baselinePerDay) : Math.max(1, Math.abs(lastDelta) || 1);
  const pctVsBaseline = Math.round((100 * (lastDelta - baselinePerDay)) / denom);
  return { lastDelta, baselinePerDay, pctVsBaseline, rangeDelta };
}

export function trendSpikeAnnotation(
  threatVelocity: ThreatVelocityPoint[],
): { idx: number; date: string; delta: number } | null {
  if (threatVelocity.length < 2) return null;
  const sorted = [...threatVelocity].sort((a, b) => a.date.localeCompare(b.date));
  const totals = sorted.map((p) => (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0));
  let maxD = 0;
  let maxIdx = 1;
  for (let i = 1; i < totals.length; i++) {
    const d = totals[i] - totals[i - 1];
    if (d > maxD) {
      maxD = d;
      maxIdx = i;
    }
  }
  if (maxD <= 0) return null;
  return { idx: maxIdx, date: sorted[maxIdx].date, delta: maxD };
}

export function trendAnomalyDetected(
  threatVelocity: ThreatVelocityPoint[],
): { spike: boolean; message?: string } {
  const spike = trendSpikeAnnotation(threatVelocity);
  if (!spike || threatVelocity.length < 3) return { spike: false };
  const sorted = [...threatVelocity].sort((a, b) => a.date.localeCompare(b.date));
  const totals = sorted.map((p) => (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0));
  const deltas: number[] = [];
  for (let i = 1; i < totals.length; i++) deltas.push(totals[i] - totals[i - 1]);
  const mean = deltas.reduce((a, b) => a + b, 0) / deltas.length;
  const variance = deltas.reduce((s, d) => s + (d - mean) ** 2, 0) / Math.max(deltas.length, 1);
  const std = Math.sqrt(variance);
  const last = deltas[deltas.length - 1];
  if (last > mean + 2 * std && last >= 3) {
    return { spike: true, message: `Unusual day-over-day surge (+${last} vs ~${mean.toFixed(1)} avg step)` };
  }
  if (spike.delta >= 5 && spike.delta >= mean * 2) {
    return { spike: true, message: `Largest spike in range (+${spike.delta} on ${spike.date})` };
  }
  return { spike: false };
}

export function histogramScoreTailInsight(data: RiskHistogramResponse | null): string | null {
  if (!data?.bins?.length) return null;
  let mass = 0;
  let tailMass = 0;
  let tailCount = 0;
  for (const b of data.bins) {
    const mid = b.bin + 5;
    const w = b.count * mid;
    mass += w;
    if (b.bin >= 80) {
      tailMass += w;
      tailCount += b.count;
    }
  }
  if (mass <= 0) return null;
  const massPct = Math.round((100 * tailMass) / mass);
  const countPct = data.totalFindings > 0 ? Math.round((100 * tailCount) / data.totalFindings) : 0;
  return `Top score bands (≥80): ${countPct}% of findings, ~${massPct}% of weighted score mass.`;
}

export function newVsExistingFindings(risks: Insight[], hours = 24): { nu: number; existing: number } {
  const cutoff = Date.now() - hours * 3600 * 1000;
  let nu = 0;
  for (const r of risks) {
    const t = r.timestamp || r.updatedAt;
    if (!t) continue;
    const ms = new Date(t).getTime();
    if (Number.isFinite(ms) && ms >= cutoff) nu++;
  }
  return { nu, existing: Math.max(0, risks.length - nu) };
}

export function exploitabilityBuckets(risks: Insight[]): { external: number; auth: number; local: number } {
  let external = 0;
  let auth = 0;
  let local = 0;
  for (const r of risks) {
    const ex = r.exploitabilityScore;
    if (ex != null && Number.isFinite(ex)) {
      if (ex >= 70) external++;
      else if (ex >= 35) auth++;
      else local++;
      continue;
    }
    const t = `${r.title} ${r.description || ''}`.toLowerCase();
    if (/\b(ingress|exposed|public|loadbalancer|0\.0\.0\.0|nodeport)\b/.test(t)) external++;
    else if (/\b(token|secret|serviceaccount|rbac|auth|kubeconfig)\b/.test(t)) auth++;
    else local++;
  }
  return { external, auth, local };
}

export function blastRadiusBuckets(risks: Insight[]): { cluster: number; ns: number; pod: number } {
  let cluster = 0;
  let ns = 0;
  let pod = 0;
  for (const r of risks) {
    const res = (r.affectedResources || []).map((x) => `${x.kind} ${x.namespace || ''}`).join(' ');
    const t = `${r.title} ${r.impact || ''} ${res}`.toLowerCase();
    if (/\bclusterrole|clusterrolebinding|cluster-wide|nodes?\b/.test(t)) cluster++;
    else if (/\bnamespace|rolebinding|role\b/.test(t) || (r.affectedResources || []).some((x) => x.namespace && x.kind && x.kind !== 'Pod'))
      ns++;
    else pod++;
  }
  return { cluster, ns, pod };
}

export function privilegedDriftHint(risks: Insight[]): string | null {
  const sixh = Date.now() - 6 * 3600 * 1000;
  let n = 0;
  for (const r of risks) {
    if (!/privileged|hostpath|host\s*network|hostpid|hostipc/i.test(`${r.title} ${r.description || ''}`)) continue;
    const t = r.timestamp || r.updatedAt;
    if (!t) continue;
    const ms = new Date(t).getTime();
    if (Number.isFinite(ms) && ms >= sixh) n++;
  }
  if (n === 0) return null;
  return `${n} privileged / host-style finding(s) touched in the last 6h (sample).`;
}

export function suggestedRiskActions(
  clusters: TopRiskClusterRow[],
  risks: Insight[],
  attackSummary: AttackStepSummary[],
): string[] {
  const lines: string[] = [];
  const top = clusters[0];
  if (top) lines.push(`Prioritize ${top.label} (${top.count} findings)`);
  const critPath = risks.find((r) => (r.finalLevel || '').toLowerCase() === 'critical' && insightTouchesAttackPath(r));
  const resName = critPath?.affectedResources?.find((x) => x.name)?.name;
  if (resName) lines.push(`Investigate ${resName} (critical + path signal)`);
  const ns = risks.find((r) => (r.affectedResources?.[0]?.namespace || '').includes('kube-system'));
  if (ns?.affectedResources?.[0]?.namespace) {
    lines.push(`Review ${ns.affectedResources[0].namespace} for host mounts / privileged workloads`);
  }
  const topStep = attackSummary[0];
  if (topStep) lines.push(`Attack-step hotspot: ${topStep.stepId} (${topStep.count} pods)`);
  return [...new Set(lines)].slice(0, 5);
}

export function attackNarrativeHint(attackSummary: AttackStepSummary[]): string | null {
  if (!attackSummary.length) return null;
  const top = attackSummary.slice(0, 4);
  return `Potential chain (step coverage): ${top.map((s) => s.stepId).join(' → ')}`;
}
