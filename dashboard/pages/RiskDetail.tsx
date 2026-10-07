import React, { useCallback, useEffect, useState } from 'react';
import { Link, useLocation, useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { CapabilityMetadata, Insight, RuntimeSignal } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { FindingActions } from '../components/FindingActions';
import { Button } from '../components/ui/Button';
import { ArrowLeft, ShieldAlert, Calendar, FileText, Box, AlertTriangle, Link2 } from 'lucide-react';
import { getSeverityBadgeClass, deriveUnifiedRiskLevelFromScore } from '../lib/severity';
import { parseThreatIntelEvidence } from '../lib/threatIntel';
import { formatRiskFindingReference, insightTypeUiLabel } from '../lib/riskDisplay';
import { runtimeSignalVisual } from '../lib/runtimeSignalVisual';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH_COMPACT, UI_TR, UI_TD_COMPACT_TIGHT } from '../lib/tableChrome';
import { buildEvidenceLogEntries } from '../lib/evidenceLog';
import { attackPathsForPodPath, findingsForResourcePath, identityDetailPath, networkForPodPath, podDetailPath } from '../lib/entityLinks';
import { When } from '../components/When';
import { useToast } from '../design-system/components/Toast';

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
  const { id: routeId } = useParams<{ id: string }>();
  const location = useLocation();
  const id = routeId ?? decodeURIComponent(location.pathname.match(/^\/risks\/([^/]+)/)?.[1] ?? '');
  const navigate = useNavigate();
  const [insight, setInsight] = useState<Insight | null>(null);
  const [loading, setLoading] = useState(true);
  const toast = useToast();
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
      const refs = insight.evidence_refs;
      const ruleIds = Array.from(new Set([...parseRuleIDsFromViolatedRules(insight.violatedRules), ...(refs?.ruleIds ?? [])])).slice(0, 6);
      const capabilityIds = Array.from(new Set([...collectCapabilityIDsFromEvidence(insight.evidence), ...(refs?.capabilityIds ?? [])])).slice(0, 6);
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
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Findings
        </Button>
      </PageLayout>
    );
  }

  const hintSev = (insight.severityHint ?? insight.severity ?? '').toLowerCase();
  const derivedLevel: string | undefined =
    insight.finalLevel != null && String(insight.finalLevel).trim() !== ''
      ? String(insight.finalLevel).toLowerCase()
      : deriveUnifiedRiskLevelFromScore(insight.score);
  const levelBadgeClass = getSeverityBadgeClass(derivedLevel ?? hintSev);
  const levelLabel = derivedLevel ?? 'N/A';
  const threatIntel = parseThreatIntelEvidence(insight.evidence);
  const statusLabelMap: Record<string, string> = {
    new: 'Active',
    active: 'Active',
    acknowledged: 'In review',
    resolved: 'Resolved',
    dismissed: 'Dismissed',
  };

  const evidenceLogEntries = buildEvidenceLogEntries(insight);

  const resources = insight.affectedResources ?? [];
  const firstPod = resources.find((r) => r.kind === 'Pod' && r.id);
  const statusLabel = statusLabelMap[insight.status ?? ''] ?? (insight.status ?? 'Active');
  const reference = formatRiskFindingReference(insight) || insight.cveId;

  return (
    <PageLayout
      title={insight.title}
      description={`Finding #${insight.id}${reference ? ` · ${reference}` : ''}`}
      actions={
        <Button variant="secondary" onClick={() => navigate('/risks')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Findings
        </Button>
      }
    >
      {/* What it is and what to do: level, status, owner and the actions in one strip. */}
      <Card className="p-5 mb-6">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div className="flex flex-wrap items-center gap-x-5 gap-y-3">
            <div
              className="flex items-center gap-2"
              title={
                insight.score != null
                  ? `Risk level is the band of the resource's risk score (${insight.score}/100). The score is a priority signal, not an exploit probability.`
                  : 'No risk score for this resource yet'
              }
            >
              <span className={`px-3 py-1 rounded-md border text-body font-semibold capitalize ${levelBadgeClass}`}>{levelLabel}</span>
              {insight.score != null ? (
                <span className="text-body text-text tabular-nums">
                  {insight.score}
                  <span className="text-muted">/100</span>
                </span>
              ) : null}
            </div>
            <span className="rounded border border-border bg-surface-2 px-2 py-0.5 text-caption uppercase text-text">{statusLabel}</span>
            <span className="text-caption text-muted" title="Severity the rule or CVE assigns. The risk level above decides priority.">
              Rule severity <span className="text-text capitalize">{hintSev || '—'}</span>
            </span>
            {insight.insightType ? (
              <span className="text-caption text-muted">
                Type <span className="text-text">{insightTypeUiLabel(insight.insightType)}</span>
              </span>
            ) : null}
            {insight.timestamp ? (
              <span className="text-caption text-muted">
                Detected <When iso={insight.timestamp} className="text-text" />
              </span>
            ) : null}
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
          {/* Same actions as the panel on Findings. */}
          <FindingActions
            insight={insight}
            onDone={(action) => {
              const done = { acknowledge: 'acknowledged', resolve: 'resolved', dismiss: 'dismissed', reopen: 'reopened' }[action];
              toast({ title: `Finding ${done}`, variant: 'success' });
              void fetchInsight();
            }}
            onAssigned={(a) => {
              toast({ title: a ? `Assigned to ${a.username}` : 'Finding unassigned', variant: 'success' });
              void fetchInsight();
            }}
          />
        </div>
      </Card>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="min-w-0 space-y-6 lg:col-span-2">
          {(insight.description || insight.impact) && (
            <Card className="p-6">
              {insight.description && (
                <>
                  <h3 className="text-section-title text-text mb-2">Why it matters</h3>
                  <p className="text-text text-body">{insight.description}</p>
                </>
              )}
              {insight.impact && (
                <div className={insight.description ? 'mt-5 pt-5 border-t border-border' : ''}>
                  <h3 className="text-section-title text-text mb-2">How to fix</h3>
                  <p className="text-text text-body">{insight.impact}</p>
                </div>
              )}
            </Card>
          )}

          {insight.breakdown && insight.breakdown.length > 0 && (
            <Card className="p-6">
              <h3 className="text-section-title text-text mb-1 flex items-center gap-2">
                <ShieldAlert className="w-5 h-5 text-brand" /> Why this score
              </h3>
              <p className="text-caption text-muted mb-4">Points each risk factor adds to the resource score.</p>
              <div className="overflow-x-auto rounded-lg border border-border">
                <table className={UI_TABLE}>
                  <thead className={UI_THEAD_STICKY}>
                    <tr>
                      <th className={UI_TH_COMPACT}>Factor</th>
                      <th className={UI_TH_COMPACT}>Category</th>
                      <th className={UI_TH_COMPACT}>Scope</th>
                      <th className={UI_TH_COMPACT}>Source</th>
                      <th className={`${UI_TH_COMPACT} text-right`}>Points</th>
                    </tr>
                  </thead>
                  <tbody>
                    {insight.breakdown.map((row, idx) => (
                      <tr key={`${row.factor_id || idx}-${idx}`} className={UI_TR}>
                        <td className={`${UI_TD_COMPACT_TIGHT} font-mono`}>{row.factor_id || '—'}</td>
                        <td className={UI_TD_COMPACT_TIGHT}>{row.category || '—'}</td>
                        <td className={UI_TD_COMPACT_TIGHT}>{row.scope || '—'}</td>
                        <td className={UI_TD_COMPACT_TIGHT}>{row.source || '—'}</td>
                        <td className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums`}>+{Number(row.contribution).toFixed(1)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Card>
          )}

          {/* Evidence log: normalized from evidence, violated rules, and backend explanation refs. */}
          {(evidenceLogEntries.length > 0 || insight.evidence != null || insight.violatedRules != null) && (
            <Card className="p-6">
              <h3 className="text-section-title text-text mb-4 flex items-center gap-2">
                <FileText className="w-5 h-5 text-brand" /> Evidence
              </h3>
              {evidenceLogEntries.length === 0 ? (
                <p className="text-muted text-body">No structured evidence was returned for this finding.</p>
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
              {insight.evidence_chain_refs != null && insight.evidence_chain_refs.length > 0 && (
                <details className="mt-4 text-caption">
                  <summary className="cursor-pointer select-none text-muted hover:text-text">
                    Pipeline refs ({insight.evidence_chain_refs.length})
                  </summary>
                  <ul className="mt-2 space-y-1 font-mono text-text">
                    {insight.evidence_chain_refs.map((row, idx) => (
                      <li key={`${row.layer}-${row.ref}-${idx}`}>
                        <span className="text-muted">{row.layer || '—'}</span>
                        <span className="text-muted-2"> · </span>
                        {row.ref}
                      </li>
                    ))}
                  </ul>
                </details>
              )}
            </Card>
          )}

          {/* Runtime signals of the impacted pods, in the selected time window. */}
          {firstPod && (
            <Card className="p-6">
              <h3 className="text-section-title text-text mb-4 flex items-center gap-2">
                <AlertTriangle className="w-5 h-5 text-amber-500" /> Runtime activity
              </h3>
              {podRuntimeSignals.length === 0 ? (
                <p className="text-muted text-body">No runtime signals on the impacted pod in the selected time window.</p>
              ) : (
                <div className="overflow-x-auto rounded-lg border border-border">
                  <table className={UI_TABLE}>
                    <thead className={UI_THEAD_STICKY}>
                      <tr>
                        <th className={UI_TH_COMPACT}>Signal</th>
                        <th className={UI_TH_COMPACT}>Category</th>
                        <th className={UI_TH_COMPACT}>When</th>
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
                          <td className={`${UI_TD_COMPACT_TIGHT} text-muted`}>
                            <When iso={s.createdAt} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </Card>
          )}
        </div>

        <div className="min-w-0 space-y-6">
          <Card className="p-5">
            <h3 className="text-section-title text-text mb-3 flex items-center gap-2">
              <Box className="w-5 h-5 text-brand" /> Resource
            </h3>
            {resources.length ? (
              <ul className="space-y-3">
                {resources.map((r, i) => {
                  const ref = { uid: r.id, clusterId: insight.clusterId };
                  return (
                    <li key={r.id || i}>
                      <div className="text-body text-text font-medium break-all">{r.name ?? r.id}</div>
                      <div className="text-caption text-muted">
                        {r.kind ?? 'Resource'}
                        {r.namespace ? ` · ${r.namespace}` : ''}
                      </div>
                      {r.id && r.kind === 'Pod' ? (
                        <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-caption">
                          <Link className="text-brand hover:underline" to={podDetailPath(r.id, insight.clusterId)}>Pod detail</Link>
                          <Link className="text-brand hover:underline" to={attackPathsForPodPath(ref, { insightId: insight.id })}>Attack paths</Link>
                          <Link className="text-brand hover:underline" to={networkForPodPath(ref, r.namespace)}>Network</Link>
                          <Link className="text-brand hover:underline" to={findingsForResourcePath(ref)}>All findings</Link>
                        </div>
                      ) : r.id && r.kind === 'ServiceAccount' ? (
                        <div className="mt-2 text-caption">
                          <Link className="text-brand hover:underline" to={identityDetailPath(ref)}>Identity detail</Link>
                        </div>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
            ) : (
              <p className="text-muted text-body">No resource linked.</p>
            )}
          </Card>

          <Card className="p-5">
            <h3 className="text-section-title text-text mb-3 flex items-center gap-2">
              <Calendar className="w-5 h-5 text-brand" /> Timeline
            </h3>
            <dl className="grid grid-cols-[auto,1fr] gap-x-4 gap-y-2 text-body">
              <dt className="text-muted">Detected</dt>
              <dd className="text-text"><When iso={insight.timestamp} /></dd>
              {insight.assignee ? (
                <>
                  <dt className="text-muted">Owner</dt>
                  <dd className="text-text">
                    {insight.assignee}
                    {insight.assignedAt ? <span className="text-muted"> · <When iso={insight.assignedAt} /></span> : null}
                  </dd>
                </>
              ) : null}
              <dt className="text-muted">Updated</dt>
              <dd className="text-text"><When iso={insight.updatedAt} /></dd>
              {insight.resolvedAt ? (
                <>
                  <dt className="text-muted">Resolved</dt>
                  <dd className="text-emerald-400"><When iso={insight.resolvedAt} /></dd>
                </>
              ) : null}
            </dl>
          </Card>

          <Card className="p-5">
            <h3 className="text-section-title text-text mb-3 flex items-center gap-2">
              <Link2 className="w-5 h-5 text-brand" /> Why it was raised
            </h3>
            <div className="text-caption uppercase tracking-wide text-muted mb-2">Rules</div>
            {linkedRules.length === 0 ? (
              <p className="text-body text-muted">No rule reference on this finding.</p>
            ) : (
              <ul className="space-y-2">
                {linkedRules.map((r) => (
                  <li key={r.id}>
                    <Link className="text-body text-brand hover:underline" to={`/rules/uid/${encodeURIComponent(r.id)}`}>
                      {r.name}
                    </Link>
                    <div
                      className="text-caption text-muted font-mono"
                      title="Primary rule = main rule in a shared signature group. Overlapping rule = same signature group, kept for compatibility or tuning."
                    >
                      {r.id}
                      {r.source ? ` · ${r.source}` : ''}
                      {r.isCanonical === false ? ` · overlaps ${r.canonicalRuleId}` : ''}
                    </div>
                  </li>
                ))}
              </ul>
            )}
            <div className="text-caption uppercase tracking-wide text-muted mt-4 mb-2">Capabilities</div>
            {linkedCapabilities.length === 0 ? (
              <p className="text-body text-muted">No capability linked to this finding.</p>
            ) : (
              <ul className="space-y-2">
                {linkedCapabilities.map((c) => (
                  <li key={c.capabilityId}>
                    <Link className="text-body text-brand hover:underline" to={`/capabilities/${encodeURIComponent(c.capabilityId)}`}>
                      {c.name || c.capabilityId}
                    </Link>
                    <div className="text-caption text-muted font-mono">{c.capabilityId}</div>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      </div>
    </PageLayout>
  );
};
