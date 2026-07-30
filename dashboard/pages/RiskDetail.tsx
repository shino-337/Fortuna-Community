import React, { useCallback, useEffect, useState } from 'react';
import { useLocation, useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { CapabilityMetadata, Insight, RuntimeSignal } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, ShieldAlert, Calendar, FileText, Box, AlertTriangle, Link2, Info } from 'lucide-react';
import { getSeverityBadgeClass, deriveUnifiedRiskLevelFromScore } from '../lib/severity';
import { parseThreatIntelEvidence } from '../lib/threatIntel';
import { formatRiskFindingReference, insightTypeUiLabel } from '../lib/riskDisplay';
import { runtimeSignalVisual } from '../lib/runtimeSignalVisual';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH_COMPACT, UI_TR, UI_TD_COMPACT_TIGHT } from '../lib/tableChrome';
import { buildEvidenceLogEntries } from '../lib/evidenceLog';

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
  const { id: routeId } = useParams<{ id: string }>();
  const location = useLocation();
  const id = routeId ?? decodeURIComponent(location.pathname.match(/^\/risks\/([^/]+)/)?.[1] ?? '');
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
      <div className="flex flex-col justify-center items-center h-[40dvh]">
        <div className="w-12 h-12 border-4 border-brand border-t-transparent rounded-full animate-spin" />
        <span className="text-muted mt-4">Loading risk...</span>
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

  const hintSev = (insight.severityHint ?? insight.severity ?? '').toLowerCase();
  const derivedLevel: string | undefined =
    insight.finalLevel != null && String(insight.finalLevel).trim() !== ''
      ? String(insight.finalLevel).toLowerCase()
      : deriveUnifiedRiskLevelFromScore(insight.score);
  const hintBadgeClass = getSeverityBadgeClass(hintSev);
  const levelBadgeClass = getSeverityBadgeClass(derivedLevel ?? hintSev);
  const levelLabel = derivedLevel ?? 'N/A';
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
  const evidenceLogEntries = buildEvidenceLogEntries(insight);

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
          <div className="flex flex-col gap-1 min-w-0">
            <span className="text-caption uppercase tracking-wide text-muted">Finding severity (hint)</span>
            <span className={`px-3 py-1 rounded-full text-body font-medium w-fit ${hintBadgeClass}`}>
              {hintSev || '—'}
            </span>
          </div>
          <div className="flex flex-col gap-1 min-w-0">
            <span className="text-caption uppercase tracking-wide text-muted">Risk level (from score)</span>
            <span
              className={`px-3 py-1 rounded-full text-body font-medium w-fit ${levelBadgeClass}`}
              title={insight.score != null ? `Derived from authoritative score ${insight.score}/100` : 'No resource score row; level not derived'}
            >
              {levelLabel}
            </span>
          </div>
          <span className="text-muted text-body uppercase">
            {statusLabelMap[insight.status ?? ''] ?? (insight.status ?? 'Active')}
          </span>
          {insight.score != null && (
            <span className="text-muted text-body">
              Risk score: <span className="text-text font-semibold">{insight.score != null ? `${insight.score}/100` : 'N/A'}</span>
            </span>
          )}
          {threatIntel.cisaKev && (
            <span
              className="text-caption font-semibold uppercase px-2 py-0.5 rounded-full border border-rose-600/80 bg-rose-950/50 text-rose-200"
              title="CVE listed in CISA Known Exploited Vulnerabilities catalog"
            >
              CISA KEV
            </span>
          )}
          {threatIntel.epss != null && (
            <span
              className="text-caption font-medium px-2 py-0.5 rounded-full border border-amber-700/60 bg-amber-950/40 text-amber-100"
              title={threatIntel.epssSource ? `EPSS source: ${threatIntel.epssSource}` : 'FIRST.org EPSS (exploit probability)'}
            >
              EPSS {(threatIntel.epss * 100).toFixed(1)}%
              {threatIntel.epssPercentile != null && (
                <span className="text-amber-200/80"> · p{(threatIntel.epssPercentile * 100).toFixed(0)}</span>
              )}
            </span>
          )}
        </div>
        <div className="mt-3 flex flex-wrap gap-3 text-caption text-muted">
          {insight.insightType && (
            <span>
              <span className="text-muted uppercase tracking-wider mr-1">Type</span>
              <span className="text-text">
                {insightTypeUiLabel(insight.insightType)}
              </span>
            </span>
          )}
          {(formatRiskFindingReference(insight) || insight.cveId) && (
            <span>
              <span className="text-muted uppercase tracking-wider mr-1">Reference</span>
              <span className="text-text font-mono">{formatRiskFindingReference(insight) || insight.cveId}</span>
            </span>
          )}
        </div>
        {insight.score != null && (
          <div className="mt-3 rounded-lg border border-border bg-surface/60 px-3 py-2 text-caption text-muted">
            Score interpretation: <span className="text-text">prioritization signal</span>, not exploit probability. Validate runtime evidence, impacted assets, and factor sources before using this for containment or acceptance decisions.
          </div>
        )}
        {insight.description && (
          <div className="mt-4">
            <h4 className="text-caption font-semibold text-muted uppercase tracking-wider mb-1">
              Why this risk matters
            </h4>
            <p className="text-text text-body">
              {insight.description}
            </p>
          </div>
        )}
        {insight.impact && (
          <div className="mt-4 pt-4 border-t border-border">
            <h4 className="text-caption font-semibold text-muted uppercase tracking-wider mb-1">
              Remediation
            </h4>
            <p className="text-text text-body">{insight.impact}</p>
          </div>
        )}
      </Card>

      {insight.breakdown && insight.breakdown.length > 0 && (
        <Card className="p-6 mb-6">
          <h3 className="text-section-title text-text mb-1 flex items-center gap-2">
            <ShieldAlert className="w-5 h-5 text-brand" /> Why this score (breakdown)
          </h3>
          <p className="text-caption text-muted mb-4">
            Contributions from normalized risk factors on the authoritative resource score. These are factor weights, not independent probabilities.
          </p>
          <div className="overflow-x-auto rounded-lg border border-border">
            <table className={UI_TABLE}>
              <thead className={UI_THEAD_STICKY}>
                <tr>
                  <th className={UI_TH_COMPACT}>Factor</th>
                  <th className={UI_TH_COMPACT}>Category</th>
                  <th className={UI_TH_COMPACT}>Scope</th>
                  <th className={UI_TH_COMPACT}>Source</th>
                  <th className={`${UI_TH_COMPACT} text-right`}>Contribution</th>
                </tr>
              </thead>
              <tbody>
                {insight.breakdown.map((row, idx) => (
                  <tr key={`${row.factor_id || idx}-${idx}`} className={UI_TR}>
                    <td className={`${UI_TD_COMPACT_TIGHT} font-mono`}>{row.factor_id || '—'}</td>
                    <td className={UI_TD_COMPACT_TIGHT}>{row.category || '—'}</td>
                    <td className={UI_TD_COMPACT_TIGHT}>{row.scope || '—'}</td>
                    <td className={UI_TD_COMPACT_TIGHT}>{row.source || '—'}</td>
                    <td className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums`}>{Number(row.contribution).toFixed(2)} pts</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Affected Assets */}
        <Card className="p-6">
          <h3 className="text-section-title text-text mb-4 flex items-center gap-2">
            <Box className="w-5 h-5 text-brand" /> Impacted Resources
          </h3>
          {insight.affectedResources?.length ? (
            <ul className="space-y-2">
              {insight.affectedResources.map((r, i) => (
                <li key={r.id || i} className="flex items-center justify-between p-3 bg-surface/50 rounded-lg border border-border">
                  <div>
                    <span className="text-text font-medium">{r.name ?? r.id}</span>
                    {r.namespace && <span className="text-muted ml-2">ns/{r.namespace}</span>}
                    {r.kind && <span className="text-muted ml-2">({r.kind})</span>}
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
            <p className="text-muted text-body">No impacted resources linked.</p>
          )}
        </Card>

        {/* Timeline */}
        <Card className="p-6">
          <h3 className="text-section-title text-text mb-4 flex items-center gap-2">
            <Calendar className="w-5 h-5 text-brand" /> Investigation Timeline
          </h3>
          <dl className="space-y-3 text-body">
            {insight.timestamp && (
              <div>
                <dt className="text-muted">Detected</dt>
                <dd className="text-text">{new Date(insight.timestamp).toLocaleString()}</dd>
              </div>
            )}
            {insight.updatedAt && (
              <div>
                <dt className="text-muted">Updated</dt>
                <dd className="text-text">{new Date(insight.updatedAt).toLocaleString()}</dd>
              </div>
            )}
            {insight.resolvedAt && (
              <div>
                <dt className="text-muted">Resolved</dt>
                <dd className="text-emerald-400">{new Date(insight.resolvedAt).toLocaleString()}</dd>
              </div>
            )}
            {!insight.timestamp && !insight.updatedAt && !insight.resolvedAt && (
              <p className="text-muted">No timeline data available.</p>
            )}
          </dl>
        </Card>
      </div>

      {/* Runtime / Escape signals – for affected Pods so risk view shows escape info when runtime has it */}
      {(insight.affectedResources?.some((r) => r.kind === 'Pod') || podRuntimeSignals.length > 0) && (
        <Card className="p-6 mt-6">
          <h3 className="text-section-title text-text mb-2 flex items-center gap-2">
            <AlertTriangle className="w-5 h-5 text-amber-500" /> Runtime Evidence
          </h3>
          <p className="text-muted text-body mb-4">Runtime events linked to impacted pods (e.g. PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT).</p>
          {podRuntimeSignals.length === 0 ? (
            <p className="text-muted text-body">No runtime evidence for impacted pods in selected time window.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className={UI_TABLE}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH_COMPACT}>Event</th>
                    <th className={UI_TH_COMPACT}>Category</th>
                    <th className={UI_TH_COMPACT}>Pod UID</th>
                    <th className={UI_TH_COMPACT}>Date</th>
                  </tr>
                </thead>
                <tbody>
                  {podRuntimeSignals.slice(0, 10).map((s) => (
                    <tr key={s.id} className={UI_TR}>
                      <td className={UI_TD_COMPACT_TIGHT}>
                        <div className="flex items-center gap-2">
                          <span className={`px-2 py-0.5 rounded text-caption font-semibold border ${runtimeSignalVisual(s.signalType).signalClass}`}>
                            {s.signalType}
                          </span>
                          <span className={`px-2 py-0.5 rounded text-caption font-medium border ${runtimeSignalVisual(s.signalType).severityClass}`}>
                            {runtimeSignalVisual(s.signalType).severity}
                          </span>
                        </div>
                      </td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted`}>{s.category}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted font-mono truncate max-w-[120px]`} title={s.podUid}>
                        {s.podUid ? (
                          <button
                            type="button"
                            onClick={() => navigate(`/resources/pods/uid/${encodeURIComponent(s.podUid)}`)}
                            className="text-brand hover:text-brand/90 hover:underline"
                          >
                            {`${s.podUid.slice(0, 8)}…`}
                          </button>
                        ) : (
                          '—'
                        )}
                      </td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted`}>{s.createdAt ? new Date(s.createdAt).toLocaleString() : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      )}

      {/* Evidence log: normalized from evidence, violated rules, and backend explanation refs. */}
      {(evidenceLogEntries.length > 0 || insight.evidence != null || insight.violatedRules != null) && (
        <Card className="p-6 mt-6">
          <h3 className="text-section-title text-text mb-2 flex items-center gap-2">
            <FileText className="w-5 h-5 text-brand" /> Evidence Log
          </h3>
          <p className="text-muted text-body mb-4">
            Normalized evidence from the finding payload, violated rules, and backend explanation refs.
          </p>
          {evidenceLogEntries.length === 0 ? (
            <p className="text-muted text-body">No structured evidence entries were returned for this finding.</p>
          ) : (
            <div className="overflow-x-auto rounded-lg border border-border">
              <table className={UI_TABLE}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH_COMPACT}>Type</th>
                    <th className={UI_TH_COMPACT}>Evidence</th>
                    <th className={UI_TH_COMPACT}>Value</th>
                    <th className={UI_TH_COMPACT}>Source</th>
                  </tr>
                </thead>
                <tbody>
                  {evidenceLogEntries.map((entry) => (
                    <tr key={entry.id} className={UI_TR}>
                      <td className={`${UI_TD_COMPACT_TIGHT} capitalize`}>{entry.kind}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted`}>{entry.label}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} font-mono break-words max-w-[34rem]`}>{entry.value}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted font-mono`}>{entry.source}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      )}

      {(linkedRules.length > 0 || linkedCapabilities.length > 0) && (
        <Card className="p-6 mt-6">
          <h3 className="text-section-title text-text mb-3 flex items-center gap-2">
            <Link2 className="w-5 h-5 text-brand" /> Linked Detections
          </h3>
          <p className="text-muted text-body mb-4">
            Bridge from this finding to detection logic and capability semantics for faster root-cause triage.
          </p>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="rounded-lg border border-border bg-surface/40 p-4">
              <div className="text-caption uppercase tracking-wide text-muted mb-2">Rules</div>
              {linkedRules.length === 0 ? (
                <p className="text-body text-muted">No linked rules found in this finding.</p>
              ) : (
                <div className="space-y-2">
                  {linkedRules.map((r) => (
                    <div key={r.id} className="flex items-center justify-between gap-2 border border-border rounded px-3 py-2">
                      <div className="min-w-0">
                        <p className="text-body text-text truncate">{r.name}</p>
                        <p className="text-caption text-muted font-mono">{r.id} {r.source ? `· ${r.source}` : ''}</p>
                      </div>
                      <Button size="sm" variant="secondary" onClick={() => navigate(`/rules/uid/${encodeURIComponent(r.id)}`)}>
                        Open
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div className="rounded-lg border border-border bg-surface/40 p-4">
              <div className="text-caption uppercase tracking-wide text-muted mb-2">Capabilities</div>
              {linkedCapabilities.length === 0 ? (
                <p className="text-body text-muted">No linked capabilities found in this finding.</p>
              ) : (
                <div className="space-y-2">
                  {linkedCapabilities.map((c) => (
                    <div key={c.capabilityId} className="border border-border rounded px-3 py-2">
                      <p className="text-body text-text">{c.name || c.capabilityId}</p>
                      <p className="text-caption text-muted font-mono">{c.capabilityId}</p>
                    </div>
                  ))}
                  <Button size="sm" variant="secondary" onClick={() => navigate('/capabilities')}>
                    Open Capabilities
                  </Button>
                </div>
              )}
            </div>
          </div>
        </Card>
      )}

      <Card className="p-6 mt-6">
        <h3 className="text-section-title text-text mb-3 flex items-center gap-2">
          <Link2 className="w-5 h-5 text-brand" /> Why triggered
        </h3>
        <p className="text-muted text-body mb-4">
          Detection context that explains why this finding was raised.
        </p>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <div className="rounded-lg border border-border bg-surface/40 p-4">
            <div className="text-caption uppercase tracking-wide text-muted mb-2">
              <span className={tooltipLabelClass} title="Source, rule signature, and rule role (Primary/Overlapping) for rules linked to this finding">
                Rule context <Info className="w-3 h-3" />
              </span>
            </div>
            {linkedRules.length === 0 ? (
              <p className="text-body text-muted">No explicit rule reference found for this finding.</p>
            ) : (
              <div className="space-y-2">
                {linkedRules.map((r) => (
                  <div key={r.id} className="border border-border rounded px-3 py-2">
                    <p className="text-body text-text">{r.name}</p>
                    <p className="text-caption text-muted font-mono" title="Rule ID, source, and rule signature">
                      {r.id} · source={r.source ?? 'unknown'} · signature={r.signature ?? 'n/a'}
                    </p>
                    <p className="text-caption text-muted" title="Primary rule = main rule in a shared signature group. Overlapping rule = same signature group, kept for compatibility/tuning.">
                      {r.isCanonical === false ? `Overlapping rule of ${r.canonicalRuleId}` : 'Primary rule'}
                    </p>
                    <Button className="mt-2" size="sm" variant="secondary" onClick={() => navigate(`/rules/uid/${encodeURIComponent(r.id)}`)}>
                      Open rule
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>
          <div className="rounded-lg border border-border bg-surface/40 p-4">
            <div className="text-caption uppercase tracking-wide text-muted mb-2">
              <span className={tooltipLabelClass} title="Most informative keys in finding evidence payload for quick non-technical review">
                Top evidence fields <Info className="w-3 h-3" />
              </span>
            </div>
            {topEvidenceFields.length === 0 ? (
              <p className="text-body text-muted">No structured evidence fields found.</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {topEvidenceFields.map((k) => (
                  <span key={k} className="px-2 py-1 rounded text-caption bg-surface-2 text-text border border-border">
                    {k}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>
        {insight.evidence_chain_refs != null && insight.evidence_chain_refs.length > 0 && (
          <div className="mt-4 rounded-lg border border-border bg-surface/40 p-4">
            <div className="text-caption uppercase tracking-wide text-muted mb-2">
              <span
                className={tooltipLabelClass}
                title="Ordered layer + ref pairs from the backend explanation chain (one row per id)."
              >
                Evidence pipeline refs <Info className="w-3 h-3" />
              </span>
            </div>
            <ul className="space-y-1 text-caption font-mono text-text">
              {insight.evidence_chain_refs.map((row, idx) => (
                <li key={`${row.layer}-${row.ref}-${idx}`}>
                  <span className="text-muted">{row.layer || '—'}</span>
                  <span className="text-muted-2"> · </span>
                  {row.ref}
                </li>
              ))}
            </ul>
          </div>
        )}
      </Card>
    </PageLayout>
  );
};
