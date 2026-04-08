import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { CapabilityMetadata, Insight, RuntimeSignal } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, ShieldAlert, Calendar, FileText, Box, AlertTriangle, Link2, Info } from 'lucide-react';
import { getSeverityBadgeClass } from '../lib/severity';
import { parseThreatIntelEvidence } from '../lib/threatIntel';
import { formatRiskFindingReference, insightTypeUiLabel } from '../lib/riskDisplay';
import { runtimeSignalVisual } from '../lib/runtimeSignalVisual';
import { useTimeWindowStore } from '../store/timeWindowStore';

const parseRuleIDsFromViolatedRules = (violatedRules: Insight['violatedRules']): string[] => {
  if (!violatedRules) return [];
  let raw: unknown = violatedRules;
  if (typeof violatedRules === 'string') {
    try {
      raw = JSON.parse(violatedRules);
    } catch {
      return [];
    }
  }
  const values = Array.isArray(raw) ? raw : [raw];
  const out = new Set<string>();
  values.forEach((v) => {
    if (typeof v === 'string' && v.trim()) out.add(v.trim());
    if (v && typeof v === 'object') {
      const o = v as Record<string, unknown>;
      const id = o.ruleId ?? o.rule_id ?? o.id;
      if (typeof id === 'string' && id.trim()) out.add(id.trim());
    }
  });
  return Array.from(out);
};

const collectCapabilityIDsFromEvidence = (evidence: Insight['evidence']): string[] => {
  if (!evidence) return [];
  let raw: unknown = evidence;
  if (typeof evidence === 'string') {
    try {
      raw = JSON.parse(evidence);
    } catch {
      return [];
    }
  }
  const out = new Set<string>();
  const walk = (node: unknown) => {
    if (!node) return;
    if (Array.isArray(node)) {
      node.forEach(walk);
      return;
    }
    if (typeof node === 'object') {
      const obj = node as Record<string, unknown>;
      const cap = obj.capabilityId ?? obj.capability_id ?? obj.capability;
      if (typeof cap === 'string' && cap.trim()) out.add(cap.trim());
      Object.values(obj).forEach(walk);
    }
  };
  walk(raw);
  return Array.from(out);
};

export const RiskDetail: React.FC = () => {
  const tooltipLabelClass = 'inline-flex items-center gap-1 underline decoration-dotted underline-offset-2 cursor-help';
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [insight, setInsight] = useState<Insight | null>(null);
  const [loading, setLoading] = useState(true);
  const [resolving, setResolving] = useState(false);
  const [podRuntimeSignals, setPodRuntimeSignals] = useState<RuntimeSignal[]>([]);
  const [linkedRules, setLinkedRules] = useState<Array<{ id: string; name: string; source?: string; signature?: string; isCanonical?: boolean; canonicalRuleId?: string }>>([]);
  const [linkedCapabilities, setLinkedCapabilities] = useState<CapabilityMetadata[]>([]);
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);

  const fetchInsight = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    const data = await api.getInsight(id);
    setInsight(data);
    setLoading(false);
  }, [id]);

  useEffect(() => {
    fetchInsight();
  }, [fetchInsight]);

  useEffect(() => {
    if (!insight) {
      setLinkedRules([]);
      setLinkedCapabilities([]);
      return;
    }
    let cancelled = false;
    const loadLinkedDetections = async () => {
      const ruleIds = parseRuleIDsFromViolatedRules(insight.violatedRules).slice(0, 6);
      const capabilityIds = collectCapabilityIDsFromEvidence(insight.evidence).slice(0, 6);
      const [rulesRes, capsRes] = await Promise.all([
        Promise.allSettled(ruleIds.map((ruleId) => api.getRule(ruleId))),
        Promise.allSettled(capabilityIds.map((capabilityId) => api.getCapabilityMetadataById(capabilityId))),
      ]);
      if (cancelled) return;

      const rules = rulesRes
        .filter((r): r is PromiseFulfilledResult<Awaited<ReturnType<typeof api.getRule>>> => r.status === 'fulfilled' && r.value != null)
        .map((r) => ({
          id: r.value!.rule.id,
          name: r.value!.rule.name ?? r.value!.rule.id,
          source: r.value!.rule.source,
          signature: r.value!.rule.signature,
          isCanonical: r.value!.rule.isCanonical,
          canonicalRuleId: r.value!.rule.canonicalRuleId,
        }));
      const caps = capsRes
        .filter((r): r is PromiseFulfilledResult<CapabilityMetadata | null> => r.status === 'fulfilled' && r.value != null)
        .map((r) => r.value as CapabilityMetadata);
      setLinkedRules(rules);
      setLinkedCapabilities(caps);
    };
    loadLinkedDetections();
    return () => {
      cancelled = true;
    };
  }, [insight]);

  // When insight has Pod assets, fetch runtime/escape signals for those pods so risk view shows escape info
  useEffect(() => {
    if (!insight?.affectedResources?.length) {
      setPodRuntimeSignals([]);
      return;
    }
    const podUids = insight.affectedResources
      .filter((r) => r.kind === 'Pod' && r.id)
      .map((r) => r.id as string);
    if (podUids.length === 0) {
      setPodRuntimeSignals([]);
      return;
    }
    let cancelled = false;
    const load = async () => {
      const all: RuntimeSignal[] = [];
      for (const uid of podUids.slice(0, 3)) {
        try {
          const sinceMinutes = timeWindowMinutes > 0 ? timeWindowMinutes : undefined;
      const signals = await api.getRuntimeSignalsByPod(uid, { limit: 10, sinceMinutes });
          if (!cancelled) all.push(...signals);
        } catch {
          // ignore per-pod errors
        }
      }
      if (!cancelled) setPodRuntimeSignals(all);
    };
    load();
    return () => { cancelled = true; };
  }, [insight?.id, insight?.affectedResources, timeWindowMinutes]);

  const handleResolve = async () => {
    if (!id || insight?.status === 'resolved') return;
    setResolving(true);
    try {
      await api.resolveInsight(id);
      await fetchInsight();
    } finally {
      setResolving(false);
    }
  };

  if (loading || !id) {
    return (
      <div className="flex flex-col justify-center items-center h-[40vh]">
        <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-slate-500 mt-4">Loading risk...</span>
      </div>
    );
  }

  if (!insight) {
    return (
      <PageLayout title="Risk not found" description="The risk may have been resolved or removed.">
        <Button variant="secondary" onClick={() => navigate('/risks')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Risk Operations
        </Button>
      </PageLayout>
    );
  }

  const severityClass = getSeverityBadgeClass(insight.severity);
  const threatIntel = parseThreatIntelEvidence(insight.evidence);
  const statusLabelMap: Record<string, string> = {
    new: 'Active',
    acknowledged: 'In review',
    resolved: 'Resolved',
  };

  const topEvidenceFields = (() => {
    if (!insight.evidence) return [] as string[];
    let raw: unknown = insight.evidence;
    if (typeof raw === 'string') {
      try {
        raw = JSON.parse(raw);
      } catch {
        return [];
      }
    }
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return [];
    return Object.keys(raw as Record<string, unknown>).slice(0, 8);
  })();

  return (
      <PageLayout
      title={insight.title}
      description={`Finding #${insight.id}${formatRiskFindingReference(insight) ? ` · ${formatRiskFindingReference(insight)}` : ''}`}
      actions={
        <div className="flex items-center gap-2">
          {insight.status !== 'resolved' && (
            <Button variant="secondary" onClick={handleResolve} disabled={resolving}>
              {resolving ? 'Resolving...' : 'Mark as resolved'}
            </Button>
          )}
          <Button variant="secondary" onClick={() => navigate('/risks')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Risk Operations
          </Button>
        </div>
      }
    >
      {/* Summary banner */}
      <Card className="p-6 mb-6">
        <div className="flex flex-wrap items-center gap-4">
          <span className={`px-3 py-1 rounded-full text-sm font-medium ${severityClass}`}>
            {insight.severity}
          </span>
          <span className="text-slate-400 text-sm uppercase">
            {statusLabelMap[insight.status ?? ''] ?? (insight.status ?? 'Active')}
          </span>
          {insight.score != null && (
            <span className="text-slate-400 text-sm">
              Risk score: <span className="text-slate-100 font-semibold">{insight.score}</span>/100
            </span>
          )}
          {insight.priorityLevel && (
            <span className="text-[11px] font-semibold uppercase px-2 py-0.5 rounded-full border border-slate-700 text-slate-300">
              Priority {insight.priorityLevel}
            </span>
          )}
          {threatIntel.cisaKev && (
            <span
              className="text-[11px] font-semibold uppercase px-2 py-0.5 rounded-full border border-rose-600/80 bg-rose-950/50 text-rose-200"
              title="CVE listed in CISA Known Exploited Vulnerabilities catalog"
            >
              CISA KEV
            </span>
          )}
          {threatIntel.epss != null && (
            <span
              className="text-[11px] font-medium px-2 py-0.5 rounded-full border border-amber-700/60 bg-amber-950/40 text-amber-100"
              title={threatIntel.epssSource ? `EPSS source: ${threatIntel.epssSource}` : 'FIRST.org EPSS (exploit probability)'}
            >
              EPSS {(threatIntel.epss * 100).toFixed(1)}%
              {threatIntel.epssPercentile != null && (
                <span className="text-amber-200/80"> · p{(threatIntel.epssPercentile * 100).toFixed(0)}</span>
              )}
            </span>
          )}
        </div>
        <div className="mt-3 flex flex-wrap gap-3 text-xs text-slate-400">
          {insight.insightType && (
            <span>
              <span className="text-slate-500 uppercase tracking-wider mr-1">Type</span>
              <span className="text-slate-200">
                {insightTypeUiLabel(insight.insightType)}
              </span>
            </span>
          )}
          {(formatRiskFindingReference(insight) || insight.cveId) && (
            <span>
              <span className="text-slate-500 uppercase tracking-wider mr-1">Reference</span>
              <span className="text-slate-200 font-mono">{formatRiskFindingReference(insight) || insight.cveId}</span>
            </span>
          )}
        </div>
        {insight.description && (
          <div className="mt-4">
            <h4 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1">
              Why this risk matters
            </h4>
            <p className="text-slate-300 text-sm">
              {insight.description}
            </p>
          </div>
        )}
        {insight.impact && (
          <div className="mt-4 pt-4 border-t border-slate-800">
            <h4 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1">
              Remediation
            </h4>
            <p className="text-slate-300 text-sm">{insight.impact}</p>
          </div>
        )}
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Affected Assets */}
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <Box className="w-5 h-5 text-pink-500" /> Impacted Resources
          </h3>
          {insight.affectedResources?.length ? (
            <ul className="space-y-2">
              {insight.affectedResources.map((r, i) => (
                <li key={r.id || i} className="flex items-center justify-between p-3 bg-slate-900/50 rounded-lg border border-slate-800">
                  <div>
                    <span className="text-white font-medium">{r.name ?? r.id}</span>
                    {r.namespace && <span className="text-slate-500 ml-2">ns/{r.namespace}</span>}
                    {r.kind && <span className="text-slate-500 ml-2">({r.kind})</span>}
                  </div>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => {
                      if (r.kind === 'Pod' && r.id) navigate(`/resources/pods/uid/${encodeURIComponent(r.id)}`);
                      if (r.kind === 'ServiceAccount' && r.id) navigate(`/identities/uid/${encodeURIComponent(r.id)}`);
                    }}
                    title={r.kind === 'Pod' ? 'View pod in Resources' : r.kind === 'ServiceAccount' ? 'View identity' : 'View resource'}
                  >
                    {r.kind === 'Pod' ? 'View pod' : r.kind === 'ServiceAccount' ? 'View identity' : 'View'}
                  </Button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-slate-500 text-sm">No impacted resources linked.</p>
          )}
        </Card>

        {/* Timeline */}
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <Calendar className="w-5 h-5 text-pink-500" /> Investigation Timeline
          </h3>
          <dl className="space-y-3 text-sm">
            {insight.timestamp && (
              <div>
                <dt className="text-slate-500">Detected</dt>
                <dd className="text-slate-300">{new Date(insight.timestamp).toLocaleString()}</dd>
              </div>
            )}
            {insight.updatedAt && (
              <div>
                <dt className="text-slate-500">Updated</dt>
                <dd className="text-slate-300">{new Date(insight.updatedAt).toLocaleString()}</dd>
              </div>
            )}
            {insight.resolvedAt && (
              <div>
                <dt className="text-slate-500">Resolved</dt>
                <dd className="text-emerald-400">{new Date(insight.resolvedAt).toLocaleString()}</dd>
              </div>
            )}
            {!insight.timestamp && !insight.updatedAt && !insight.resolvedAt && (
              <p className="text-slate-500">No timeline data available.</p>
            )}
          </dl>
        </Card>
      </div>

      {/* Runtime / Escape signals – for affected Pods so risk view shows escape info when runtime has it */}
      {(insight.affectedResources?.some((r) => r.kind === 'Pod') || podRuntimeSignals.length > 0) && (
        <Card className="p-6 mt-6">
          <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2">
            <AlertTriangle className="w-5 h-5 text-amber-500" /> Runtime Evidence
          </h3>
          <p className="text-slate-500 text-sm mb-4">Runtime events linked to impacted pods (e.g. PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT).</p>
          {podRuntimeSignals.length === 0 ? (
            <p className="text-slate-500 text-sm">No runtime evidence for impacted pods in selected time window.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-slate-500 border-b border-slate-800">
                    <th className="text-left py-2">Event</th>
                    <th className="text-left py-2">Category</th>
                    <th className="text-left py-2">Pod UID</th>
                    <th className="text-left py-2">Date</th>
                  </tr>
                </thead>
                <tbody>
                  {podRuntimeSignals.slice(0, 10).map((s) => (
                    <tr key={s.id} className="border-b border-slate-800/50">
                      <td className="py-1.5">
                        <div className="flex items-center gap-2">
                          <span className={`px-2 py-0.5 rounded text-[11px] font-semibold border ${runtimeSignalVisual(s.signalType).signalClass}`}>
                            {s.signalType}
                          </span>
                          <span className={`px-2 py-0.5 rounded text-[10px] font-medium border ${runtimeSignalVisual(s.signalType).severityClass}`}>
                            {runtimeSignalVisual(s.signalType).severity}
                          </span>
                        </div>
                      </td>
                      <td className="py-1.5 text-slate-400">{s.category}</td>
                      <td className="py-1.5 text-slate-500 font-mono truncate max-w-[120px]" title={s.podUid}>
                        {s.podUid ? (
                          <button
                            type="button"
                            onClick={() => navigate(`/resources/pods/uid/${encodeURIComponent(s.podUid)}`)}
                            className="text-pink-400 hover:text-pink-300 hover:underline"
                          >
                            {`${s.podUid.slice(0, 8)}…`}
                          </button>
                        ) : (
                          '—'
                        )}
                      </td>
                      <td className="py-1.5 text-muted">{s.createdAt ? new Date(s.createdAt).toLocaleString() : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      )}

      {/* Evidence / Violated Rules – from backend insights.evidence and insights.violated_rules */}
      {(insight.evidence != null || insight.violatedRules != null) && (
        <Card className="p-6 mt-6">
          <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2">
            <FileText className="w-5 h-5 text-pink-500" /> Evidence & Violated Rules
          </h3>
          {insight.evidence != null && (
            <div className="mb-4">
              <h4 className="text-slate-400 text-xs uppercase tracking-wider mb-2">Evidence</h4>
              <pre className="p-3 bg-slate-900/50 rounded border border-slate-800 text-slate-300 text-xs font-mono whitespace-pre-wrap break-words max-w-full overflow-x-auto">
                {typeof insight.evidence === 'string'
                  ? insight.evidence
                  : JSON.stringify(insight.evidence, null, 2)}
              </pre>
            </div>
          )}
          {insight.violatedRules != null && (
            <div>
              <h4 className="text-slate-400 text-xs uppercase tracking-wider mb-2">Violated Rules</h4>
              <pre className="p-3 bg-slate-900/50 rounded border border-slate-800 text-slate-300 text-xs font-mono whitespace-pre-wrap break-words max-w-full overflow-x-auto">
                {typeof insight.violatedRules === 'string'
                  ? insight.violatedRules
                  : JSON.stringify(insight.violatedRules, null, 2)}
              </pre>
            </div>
          )}
        </Card>
      )}

      {(linkedRules.length > 0 || linkedCapabilities.length > 0) && (
        <Card className="p-6 mt-6">
          <h3 className="text-lg font-semibold text-white mb-3 flex items-center gap-2">
            <Link2 className="w-5 h-5 text-pink-500" /> Linked Detections
          </h3>
          <p className="text-slate-500 text-sm mb-4">
            Bridge from this finding to detection logic and capability semantics for faster root-cause triage.
          </p>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-4">
              <div className="text-xs uppercase tracking-wide text-slate-500 mb-2">Rules</div>
              {linkedRules.length === 0 ? (
                <p className="text-sm text-slate-500">No linked rules found in this finding.</p>
              ) : (
                <div className="space-y-2">
                  {linkedRules.map((r) => (
                    <div key={r.id} className="flex items-center justify-between gap-2 border border-slate-800 rounded px-3 py-2">
                      <div className="min-w-0">
                        <p className="text-sm text-slate-200 truncate">{r.name}</p>
                        <p className="text-xs text-slate-500 font-mono">{r.id} {r.source ? `· ${r.source}` : ''}</p>
                      </div>
                      <Button size="sm" variant="secondary" onClick={() => navigate(`/rules/${encodeURIComponent(r.id)}`)}>
                        Open
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-4">
              <div className="text-xs uppercase tracking-wide text-slate-500 mb-2">Capabilities</div>
              {linkedCapabilities.length === 0 ? (
                <p className="text-sm text-slate-500">No linked capabilities found in this finding.</p>
              ) : (
                <div className="space-y-2">
                  {linkedCapabilities.map((c) => (
                    <div key={c.capabilityId} className="border border-slate-800 rounded px-3 py-2">
                      <p className="text-sm text-slate-200">{c.name || c.capabilityId}</p>
                      <p className="text-xs text-slate-500 font-mono">{c.capabilityId}</p>
                    </div>
                  ))}
                  <Button size="sm" variant="secondary" onClick={() => navigate('/capabilities')}>
                    Open Capability Knowledge
                  </Button>
                </div>
              )}
            </div>
          </div>
        </Card>
      )}

      <Card className="p-6 mt-6">
        <h3 className="text-lg font-semibold text-white mb-3 flex items-center gap-2">
          <Link2 className="w-5 h-5 text-pink-500" /> Why triggered
        </h3>
        <p className="text-slate-500 text-sm mb-4">
          Detection context that explains why this finding was raised.
        </p>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-4">
            <div className="text-xs uppercase tracking-wide text-slate-500 mb-2">
              <span className={tooltipLabelClass} title="Source, rule signature, and rule role (Primary/Overlapping) for rules linked to this finding">
                Rule context <Info className="w-3 h-3" />
              </span>
            </div>
            {linkedRules.length === 0 ? (
              <p className="text-sm text-slate-500">No explicit rule reference found for this finding.</p>
            ) : (
              <div className="space-y-2">
                {linkedRules.map((r) => (
                  <div key={r.id} className="border border-slate-800 rounded px-3 py-2">
                    <p className="text-sm text-slate-200">{r.name}</p>
                    <p className="text-xs text-slate-500 font-mono" title="Rule ID, source, and rule signature">
                      {r.id} · source={r.source ?? 'unknown'} · signature={r.signature ?? 'n/a'}
                    </p>
                    <p className="text-xs text-slate-500" title="Primary rule = main rule in a shared signature group. Overlapping rule = same signature group, kept for compatibility/tuning.">
                      {r.isCanonical === false ? `Overlapping rule of ${r.canonicalRuleId}` : 'Primary rule'}
                    </p>
                    <Button className="mt-2" size="sm" variant="secondary" onClick={() => navigate(`/rules/${encodeURIComponent(r.id)}`)}>
                      Open rule
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>
          <div className="rounded-lg border border-slate-800 bg-slate-900/40 p-4">
            <div className="text-xs uppercase tracking-wide text-slate-500 mb-2">
              <span className={tooltipLabelClass} title="Most informative keys in finding evidence payload for quick non-technical review">
                Top evidence fields <Info className="w-3 h-3" />
              </span>
            </div>
            {topEvidenceFields.length === 0 ? (
              <p className="text-sm text-slate-500">No structured evidence fields found.</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {topEvidenceFields.map((k) => (
                  <span key={k} className="px-2 py-1 rounded text-[11px] bg-slate-800 text-slate-300 border border-slate-700">
                    {k}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>
      </Card>
    </PageLayout>
  );
};
