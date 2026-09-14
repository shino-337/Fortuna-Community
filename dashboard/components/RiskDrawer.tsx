import { summarizeBulkFindingResult } from '../lib/bulkFindingResult';
import React, { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, User, ExternalLink, Loader2, X } from 'lucide-react';
import { Button } from './ui/Button';
import { api } from '../lib/api';
import { runtimeSignalVisual } from '../lib/runtimeSignalVisual';
import { deriveUnifiedRiskLevelFromScore, getSeverityBadgeClass, getSeverityIcon } from '../lib/severity';
import type { AuditLog, Insight, PodCapabilityDetail, RuntimeSignal } from '../types';
import {
  UI_TABLE, UI_THEAD_STICKY, UI_TH_COMPACT, UI_TR, UI_TD_COMPACT_TIGHT,
} from '../lib/tableChrome';
import { PinToInvestigationButton } from './PinToInvestigationButton';
import { findingInvestigationEntity } from '../lib/investigationEntities';

/* ─── props ──────────────────────────────────────────────── */

export interface RiskDrawerProps {
  insight: Insight;
  sinceMinutesForApi?: number;
  canAck: boolean;
  canResolve: boolean;
  canDismiss: boolean;
  onClose: () => void;
  /** Called after a successful workflow action so the parent can refresh its list */
  onActionComplete: (action: 'acknowledge' | 'resolve' | 'dismiss') => void;
  /** Callback to switch the parent to the PCE tab */
  onOpenPceTab?: () => void;
}

type DrawerTab = 'summary' | 'evidence' | 'pce';
type RiskWorkflowAction = 'acknowledge' | 'resolve' | 'dismiss';

const statusLabelMap: Record<string, string> = {
  new: 'Active',
  acknowledged: 'In review',
  resolved: 'Resolved',
};

function formatRiskFactor(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(Number(value))) return '—';
  const numeric = Number(value);
  const normalized = numeric <= 1 ? numeric * 10 : numeric;
  const band = normalized >= 7 ? 'High' : normalized >= 4 ? 'Medium' : 'Low';
  return `${band} (${numeric.toFixed(1)})`;
}

function normalizeResourceKind(kind?: string): string {
  return String(kind ?? '')
    .trim()
    .toLowerCase()
    .replace(/[\s_-]+/g, '');
}

function affectedPodUid(insight: Insight): string | undefined {
  const pod = insight.affectedResources?.find((r) => normalizeResourceKind(r.kind) === 'pod' && String(r.id ?? '').trim());
  const uid = String(pod?.id ?? '').trim();
  return uid || undefined;
}

/* ─── component ──────────────────────────────────────────── */

/** Drawer modal for a risk finding — summary, evidence/audit tabs, and workflow actions. */
export const RiskDrawer: React.FC<RiskDrawerProps> = ({
  insight,
  sinceMinutesForApi,
  canAck,
  canResolve,
  canDismiss,
  onClose,
  onActionComplete,
  onOpenPceTab,
}) => {
  const navigate = useNavigate();
  const drawerRef = useRef<HTMLDivElement>(null);
  const actionDialogRef = useRef<HTMLDivElement>(null);

  /* ── local state (owned by this component, not the parent) ── */
  const [tab, setTab] = useState<DrawerTab>('summary');
  const [loading, setLoading] = useState(false);
  const [actionBusy, setActionBusy] = useState<null | 'ack' | 'resolve' | 'dismiss'>(null);
  const [pendingAction, setPendingAction] = useState<RiskWorkflowAction | null>(null);
  const [actionReason, setActionReason] = useState('');
  const [notice, setNotice] = useState<string | null>(null);

  const [detail, setDetail] = useState<Insight | null>(null);
  const [signals, setSignals] = useState<RuntimeSignal[]>([]);
  const [capabilities, setCapabilities] = useState<PodCapabilityDetail[]>([]);
  const [references, setReferences] = useState<string[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [context, setContext] = useState<{
    pods: any[]; cluster?: { id?: string; name?: string }; rules: any[];
  } | null>(null);

  const scoreEvidenceLabel = loading
    ? 'Loading evidence context'
    : signals.length > 0
      ? `Runtime-backed in selected window (${signals.length} signal${signals.length === 1 ? '' : 's'})`
      : 'Latent/static only in selected window';
  const finalRiskLevel = insight.finalLevel ?? deriveUnifiedRiskLevelFromScore(insight.score);

  /* ── data loading ─────────────────────────────────────────── */
  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const podUid = affectedPodUid(insight);
        const [detailRes, signalsRes, capsRes, auditRes, contextRes] = await Promise.allSettled([
          api.getInsight(insight.id),
          podUid ? api.getRuntimeSignalsByPod(podUid, { limit: 10, sinceMinutes: sinceMinutesForApi }) : Promise.resolve([]),
          podUid ? api.getPodCapabilities(podUid) : Promise.resolve([]),
          api.getAuditLogs({ resource: 'insight', resourceId: insight.id, page: 1, pageSize: 20 }),
          api.getInsightContext(insight.id),
        ]);

        if (cancelled) return;

        setDetail(detailRes.status === 'fulfilled' ? detailRes.value : null);
        setSignals(signalsRes.status === 'fulfilled' ? signalsRes.value : []);
        setCapabilities(capsRes.status === 'fulfilled' ? capsRes.value : []);
        setAuditLogs(auditRes.status === 'fulfilled' ? auditRes.value.logs : []);

        const ctx = contextRes.status === 'fulfilled' && contextRes.value ? {
          pods: Array.isArray(contextRes.value.pods) ? contextRes.value.pods : [],
          cluster: contextRes.value.cluster as { id?: string; name?: string } | undefined,
          rules: Array.isArray(contextRes.value.rules) ? contextRes.value.rules : [],
        } : null;
        setContext(ctx);

        // Resolve capability references
        const capIds = Array.from(new Set((capsRes.status === 'fulfilled' ? capsRes.value : []).map((c: PodCapabilityDetail) => c.capabilityId).filter(Boolean))).slice(0, 5);
        const metaResults = await Promise.allSettled(capIds.map((id) => api.getCapabilityMetadataById(id)));
        if (!cancelled) {
          setReferences(Array.from(new Set(
            metaResults
              .filter((r): r is PromiseFulfilledResult<any> => r.status === 'fulfilled')
              .flatMap((r) => (r.value?.references || []))
              .filter((v: unknown) => typeof v === 'string' && /^https?:\/\//.test(v))
          )));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    return () => { cancelled = true; };
  }, [insight.id, sinceMinutesForApi]);

  /* ── focus trap + escape ──────────────────────────────────── */
  useEffect(() => {
    const root = drawerRef.current;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        if (pendingAction) {
          setPendingAction(null);
          setActionReason('');
        } else {
          onClose();
        }
        return;
      }
      const activeRoot = pendingAction ? actionDialogRef.current : root;
      if (e.key !== 'Tab' || !activeRoot) return;
      const focusable = Array.from(activeRoot.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      )).filter((el) => !el.hasAttribute('disabled') && el.offsetParent !== null);
      if (focusable.length === 0) return;
      const active = document.activeElement as HTMLElement | null;
      if (!active || !activeRoot.contains(active)) {
        e.preventDefault();
        focusable[0].focus();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey) {
        if (active === first) { e.preventDefault(); last.focus(); }
      } else if (active === last) { e.preventDefault(); first.focus(); }
    };
    window.addEventListener('keydown', onKeyDown, true);
    const t = window.setTimeout(() => {
      (pendingAction ? actionDialogRef.current : drawerRef.current)?.querySelector<HTMLElement>('button, [href], input, textarea')?.focus();
    }, 0);
    return () => { window.clearTimeout(t); window.removeEventListener('keydown', onKeyDown, true); };
  }, [onClose, pendingAction, tab]);

  /* ── workflow action helper ───────────────────────────────── */
  const openActionReview = (action: RiskWorkflowAction) => {
    setNotice(null);
    setActionReason('');
    setPendingAction(action);
  };

  const runAction = async (action: RiskWorkflowAction) => {
    if (actionBusy) return;
    const reason = actionReason.trim();
    if ((action === 'resolve' || action === 'dismiss') && reason.length < 8) {
      setNotice('Resolution or dismissal requires a reason of at least 8 characters.');
      return;
    }
    setActionBusy(action === 'acknowledge' ? 'ack' : action === 'resolve' ? 'resolve' : 'dismiss');
    try {
      const result = await api.bulkInsightsAction({
        action,
        insightIds: [insight.id],
        resolution: action === 'resolve' ? reason : undefined,
        reason: action === 'dismiss' ? reason : undefined,
      });
      const outcome = summarizeBulkFindingResult(result, [insight.id]);
      if (!outcome.complete) {
        setNotice(outcome.message);
        return;
      }
      setNotice(null);
      setPendingAction(null);
      setActionReason('');
      onActionComplete(action);
    } catch (e) {
      setNotice(String(e instanceof Error ? e.message : e));
    } finally {
      setActionBusy(null);
    }
  };

  const pendingActionLabel = pendingAction === 'acknowledge'
    ? 'Acknowledge'
    : pendingAction === 'resolve'
      ? 'Resolve'
      : pendingAction === 'dismiss'
        ? 'Dismiss'
        : '';
  const pendingActionRequiresReason = pendingAction === 'resolve' || pendingAction === 'dismiss';
  const pendingActionDisabled =
    actionBusy !== null || (pendingActionRequiresReason && actionReason.trim().length < 8);

  /* ── render ───────────────────────────────────────────────── */
  return (
    <>
      <div className="fixed inset-0 bg-black/40 z-overlay" onClick={onClose} aria-hidden="true" />
      <div
        ref={drawerRef}
        className="fixed top-0 right-0 bottom-0 w-full max-w-lg bg-surface border-l border-border shadow-xl z-modal overflow-y-auto overscroll-y-contain flex flex-col"
        role="dialog"
        aria-modal="true"
        aria-labelledby="risk-drawer-title"
      >
        {/* Header */}
        <div className="p-4 border-b border-border flex items-start justify-between shrink-0">
          <div className="min-w-0 pr-4">
            <div className="flex items-center gap-2 mb-1">
              {getSeverityIcon(finalRiskLevel)}
              <h2 id="risk-drawer-title" className="text-lg font-bold text-text truncate">{insight.title}</h2>
            </div>
            <div className="flex flex-wrap gap-x-3 gap-y-1 text-caption text-muted">
              {finalRiskLevel ? (
                <span className={`rounded border px-2 py-0.5 font-semibold uppercase ${getSeverityBadgeClass(finalRiskLevel)}`}>
                  Risk level: {finalRiskLevel}
                </span>
              ) : null}
              <span title="Prioritization score, not an exploit probability.">Score: <span className="text-text">{insight.score ?? '—'}/100</span></span>
              <span title="Factor band; inspect evidence before treating this as confirmed exploitability.">Exploitability: <span className="text-text">{formatRiskFactor(insight.exploitabilityScore)}</span></span>
              <span title="Factor band based on impacted resource context.">Business impact: <span className="text-text">{formatRiskFactor(insight.businessImpactScore)}</span></span>
              <span title="Recency/age adjustment factor, not a confidence score.">Time decay: <span className="text-text">{insight.timeDecay != null ? insight.timeDecay.toFixed(2) : '—'}</span></span>
              <span>Source: {insight.insightType === 'vulnerability' || insight.insightType === 'supply_chain_malware' ? 'Static scan' : 'Runtime behavior'}</span>
              <span>Evidence: <span className="text-text">{scoreEvidenceLabel}</span></span>
              <span>Workflow: <span className="uppercase">{statusLabelMap[insight.status || ''] ?? insight.status}</span></span>
            </div>
          </div>
          <button onClick={onClose} className="inline-flex min-h-10 min-w-10 items-center justify-center rounded text-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70" aria-label="Close"><X size={20} /></button>
        </div>

        {/* Body */}
        <div className="p-4 space-y-5 flex-1">
          {loading && (
            <div className="flex items-center gap-2 text-caption text-muted">
              <Loader2 className="w-4 h-4 animate-spin" />
              Loading linked risk, capability exposure, and evidence context...
            </div>
          )}

          {/* Tab bar */}
          <div className="border-b border-border mb-4">
            <nav className="flex gap-2 text-caption">
              {(['summary', 'evidence', 'pce'] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setTab(t)}
                  className={`rounded-md border px-3 py-1.5 transition-colors motion-reduce:transition-none ${tab === t ? 'border-brand/45 bg-brand/10 text-brand' : 'border-transparent text-muted hover:border-border hover:text-text'}`}
                >
                  {t === 'summary' ? 'Summary' : t === 'evidence' ? 'Evidence & Audit' : 'Capability Exposure & Attack Path'}
                </button>
              ))}
            </nav>
          </div>

          {/* Summary tab */}
          {tab === 'summary' && (
            <>
              <section>
                <div className="text-caption text-muted bg-base border border-border rounded px-3 py-2">
                  Relationship: <span className="text-text">Risk finding</span> → <span className="text-text">affected Pod</span> → <span className="text-text">capabilities</span> → <span className="text-text">runtime evidence &amp; references</span>.
                </div>
                <div className="mt-2 text-caption text-muted bg-surface/60 border border-border rounded px-3 py-2">
                  Score interpretation: <span className="text-text">prioritization signal</span>, not exploit probability. {scoreEvidenceLabel}; linked capabilities: {capabilities.length}.
                </div>
              </section>
              <section className="mt-4">
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Finding Summary</h3>
                <p className="text-text text-body leading-relaxed">
                  {detail?.riskExplanation || detail?.description || insight.description || '—'}
                </p>
                <div className="mt-2 text-caption text-muted">
                  Finding severity hint: {insight.severityHint ?? insight.severity} · First detected: {insight.timestamp ? new Date(insight.timestamp).toLocaleString() : '—'}
                </div>
                <div className="mt-3 grid grid-cols-2 gap-2 text-caption">
                  <div className="bg-base border border-border rounded px-2 py-1.5 text-muted">
                    Runtime evidence<div className="text-text font-semibold">{signals.length}</div>
                  </div>
                  <div className="bg-base border border-border rounded px-2 py-1.5 text-muted">
                    Linked capabilities<div className="text-text font-semibold">{capabilities.length}</div>
                  </div>
                </div>
              </section>
              <section className="mt-4">
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Recommended Actions</h3>
                {detail?.remediation ? (
                  <div className="space-y-1.5 text-caption text-text">
                    {Array.isArray(detail.remediation)
                      ? detail.remediation.map((item, idx) => (
                          <div key={idx} className="flex items-start gap-2 bg-base border border-border rounded px-2 py-1.5">
                            <span className="mt-0.5 h-1.5 w-1.5 rounded-full bg-brand shrink-0" />
                            <span className="whitespace-pre-line">{typeof item === 'string' ? item : JSON.stringify(item)}</span>
                          </div>
                        ))
                      : <div className="bg-base border border-border rounded px-2 py-1.5 whitespace-pre-line">
                          {typeof detail.remediation === 'string' ? detail.remediation : JSON.stringify(detail.remediation, null, 2)}
                        </div>
                    }
                  </div>
                ) : (
                  <p className="text-muted text-caption">No structured remediation steps available yet.</p>
                )}
              </section>
              <section className="mt-4">
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Impacted Resources</h3>
                <div className="flex flex-wrap gap-2">
                  {insight.affectedResources?.length ? insight.affectedResources.map((res, idx) => (
                    <div key={idx} className="flex items-center bg-base border border-border rounded px-3 py-2 text-body text-text">
                      {normalizeResourceKind(res.kind) === 'pod' && <Box className="w-4 h-4 mr-2 text-blue-400" />}
                      {normalizeResourceKind(res.kind) === 'serviceaccount' && <User className="w-4 h-4 mr-2 text-green-400" />}
                      <span className="font-medium">{res.name}</span>
                      <span className="ml-2 text-caption text-muted">({res.kind})</span>
                      {res.namespace && <span className="ml-2 text-caption text-muted">{res.namespace}</span>}
                    </div>
                  )) : <span className="text-muted text-body">—</span>}
                </div>
              </section>
              <section className="mt-4">
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Context Links</h3>
                <div className="flex flex-wrap gap-2 text-caption">
                  {context?.pods?.length ? (
                    <button
                      type="button"
                      onClick={() => {
                        const podUid = context.pods[0]?.uid || affectedPodUid(insight);
                        if (podUid) { onClose(); navigate(`/resources/pods/uid/${encodeURIComponent(String(podUid))}`); }
                      }}
                      className="inline-flex items-center px-2 py-1 rounded bg-base border border-border text-text hover:border-brand"
                    >
                      <Box className="w-3 h-3 mr-1 text-blue-400" /> View pod in Resources
                    </button>
                  ) : null}
                  {context?.cluster?.id && (
                    <button
                      type="button"
                      onClick={() => { onClose(); navigate(`/clusters/${encodeURIComponent(String(context.cluster!.id))}`); }}
                      className="inline-flex items-center px-2 py-1 rounded bg-base border border-border text-text hover:border-brand"
                    >
                      <span className="w-3 h-3 mr-1 rounded-full bg-muted" /> View cluster
                    </button>
                  )}
                  {context?.rules?.length
                    ? context.rules.slice(0, 3).map((rule: any) => (
                        <button key={rule.ruleId || rule.id} type="button"
                          onClick={() => { if (!rule.ruleId) return; onClose(); navigate(`/rules/uid/${encodeURIComponent(String(rule.ruleId))}`); }}
                          className="inline-flex items-center px-2 py-1 rounded bg-base border border-border text-text hover:border-brand"
                        >
                          <span className="w-3 h-3 mr-1 rounded-full bg-amber-500" /> Rule {rule.ruleId}
                        </button>
                      ))
                    : <span className="text-muted">No additional context links available yet.</span>
                  }
                </div>
              </section>
            </>
          )}

          {/* Evidence & Audit tab */}
          {tab === 'evidence' && (
            <>
              <section>
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Runtime Evidence</h3>
                {signals.length === 0 ? (
                  <p className="text-muted text-caption">No runtime evidence in current time window.</p>
                ) : (
                  <div className="space-y-1.5">
                    {signals.slice(0, 5).map((s) => (
                      <div key={s.id} className="text-caption bg-base border border-border rounded px-2 py-1.5 text-text">
                        <span className={`px-2 py-0.5 rounded text-caption font-semibold border mr-2 ${runtimeSignalVisual(s.signalType).signalClass}`}>{s.signalType}</span>
                        <span className={`px-2 py-0.5 rounded text-caption font-medium border mr-2 ${runtimeSignalVisual(s.signalType).severityClass}`}>{runtimeSignalVisual(s.signalType).severity}</span>
                        <span className="text-muted"> · {s.category} · {new Date(s.createdAt).toLocaleString()}</span>
                      </div>
                    ))}
                    <p className="text-caption text-muted mt-1">Showing {Math.min(5, signals.length)} of {signals.length} signal(s).</p>
                  </div>
                )}
              </section>
              <section className="mt-4">
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Audit Trail</h3>
                {auditLogs.length === 0 ? (
                  <p className="text-muted text-caption">No audit logs for this finding yet.</p>
                ) : (
                  <div className="max-h-40 overflow-y-auto border border-border rounded">
                    <table className={UI_TABLE}>
                      <thead className={UI_THEAD_STICKY}><tr>
                        <th className={UI_TH_COMPACT}>Time</th><th className={UI_TH_COMPACT}>User</th>
                        <th className={UI_TH_COMPACT}>Action</th><th className={UI_TH_COMPACT}>Details</th>
                      </tr></thead>
                      <tbody>
                        {auditLogs.map((log) => (
                          <tr key={log.id} className={UI_TR}>
                            <td className={`${UI_TD_COMPACT_TIGHT} text-muted whitespace-nowrap`}>
                              {log.timestamp ? (() => { try { return new Date(log.timestamp).toLocaleString(); } catch { return log.timestamp; } })() : '—'}
                            </td>
                            <td className={`${UI_TD_COMPACT_TIGHT} text-text`}>{log.user || log.actor || 'system'}</td>
                            <td className={`${UI_TD_COMPACT_TIGHT} text-text`}>{log.action}</td>
                            <td className={`${UI_TD_COMPACT_TIGHT} text-muted truncate max-w-[160px]`} title={log.details}>{log.details}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>
            </>
          )}

          {/* PCE & Attack Path tab */}
          {tab === 'pce' && (
            <>
              <section>
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Related Capability IDs</h3>
                {capabilities.length === 0 ? (
                  <p className="text-muted text-caption">No linked capability found on the affected pod.</p>
                ) : (
                  <div className="space-y-2">
                    {Array.from(new Set(capabilities.map((c) => c.capabilityId))).slice(0, 5).map((capId) => (
                      <div key={capId} className="text-caption text-text bg-base border border-border rounded px-2 py-1.5">
                        Capability: <span className="font-mono text-amber-300">{capId}</span>
                      </div>
                    ))}
                  </div>
                )}
              </section>
              <section className="mt-4">
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Capability Details</h3>
                {capabilities.length === 0 ? (
                  <p className="text-muted text-caption">No capability context available.</p>
                ) : (
                  <div className="max-h-40 overflow-y-auto border border-border rounded">
                    <table className={UI_TABLE}>
                      <thead className={UI_THEAD_STICKY}><tr>
                        <th className={UI_TH_COMPACT}>Capability</th><th className={UI_TH_COMPACT}>Severity</th><th className={UI_TH_COMPACT}>State</th>
                      </tr></thead>
                      <tbody>
                        {capabilities.slice(0, 8).map((c, idx) => (
                          <tr key={`${c.capabilityId}-${idx}`} className={UI_TR}>
                            <td className={`${UI_TD_COMPACT_TIGHT} text-text font-mono`}>{c.capabilityId}</td>
                            <td className={`${UI_TD_COMPACT_TIGHT} text-muted uppercase`}>{c.severity}</td>
                            <td className={`${UI_TD_COMPACT_TIGHT} text-muted`}>{c.state ?? 'detected'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>
              <section className="mt-4">
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">Capability Exposure Impact</h3>
                <p className="text-muted text-caption mb-2">Open the Capability Exposure tab for detailed filtering.</p>
                {onOpenPceTab && (
                  <button type="button" onClick={() => { onClose(); onOpenPceTab(); }} className="text-caption text-brand hover:underline">
                    Open capability exposure tab
                  </button>
                )}
                <div className="mt-3">
                  <Button size="sm" variant="secondary" onClick={() => {
                    const podUid = affectedPodUid(insight);
                    const q = new URLSearchParams();
                    q.set('insightId', insight.id);
                    if (podUid) q.set('podUid', podUid);
                    if (insight.clusterId) q.set('clusterId', insight.clusterId);
                    onClose();
                    navigate(`/attack-paths?${q.toString()}`);
                  }}>
                    Open attack path view
                  </Button>
                </div>
              </section>
              <section className="mt-4">
                <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-2">External References</h3>
                {references.length === 0 ? (
                  <p className="text-muted text-caption">No external references linked from capability catalog.</p>
                ) : (
                  <div className="space-y-1.5">
                    {references.slice(0, 5).map((url) => (
                      <a key={url} href={url} target="_blank" rel="noopener noreferrer"
                        className="flex items-center gap-1 text-caption text-brand hover:underline break-all">
                        <ExternalLink className="w-3 h-3 shrink-0" /> {url}
                      </a>
                    ))}
                  </div>
                )}
              </section>
            </>
          )}

          {notice && <p className="text-caption text-muted border border-border rounded-lg px-3 py-2 bg-base/80">{notice}</p>}

          {/* Actions */}
          <section className="pt-4 mt-4 border-t border-border flex flex-wrap gap-2">
            <PinToInvestigationButton entity={findingInvestigationEntity(insight)} />
            <Button size="sm" variant="secondary" onClick={onClose}>Close</Button>
            <Button size="sm" variant="secondary" onClick={() => { onClose(); navigate(`/risks/${insight.id}`); }}>Open full detail page</Button>
            <Button size="sm" variant="secondary" disabled={!canAck || actionBusy !== null} onClick={() => openActionReview('acknowledge')}>
              {actionBusy === 'ack' ? <span className="inline-flex items-center gap-1.5"><Loader2 className="w-3.5 h-3.5 animate-spin" />Acknowledge</span> : 'Acknowledge'}
            </Button>
            <Button size="sm" disabled={!canResolve || actionBusy !== null} onClick={() => openActionReview('resolve')}>
              {actionBusy === 'resolve' ? <span className="inline-flex items-center gap-1.5"><Loader2 className="w-3.5 h-3.5 animate-spin" />Resolve</span> : 'Resolve'}
            </Button>
            <Button size="sm" variant="secondary" disabled={!canDismiss || actionBusy !== null} onClick={() => openActionReview('dismiss')}>
              {actionBusy === 'dismiss' ? <span className="inline-flex items-center gap-1.5"><Loader2 className="w-3.5 h-3.5 animate-spin" />Dismiss</span> : 'Dismiss'}
            </Button>
          </section>
        </div>
        {pendingAction && (
          <div className="fixed inset-0 z-toast flex items-center justify-center bg-black/55 px-4">
            <div
              ref={actionDialogRef}
              role="dialog"
              aria-modal="true"
              aria-labelledby="risk-action-review-title"
              className="w-full max-w-md rounded-xl border border-border bg-surface p-4 shadow-2xl"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h3 id="risk-action-review-title" className="text-body font-semibold text-text">
                    Review {pendingActionLabel.toLowerCase()} action
                  </h3>
                  <p className="mt-1 text-caption text-muted">
                    This updates one finding and writes to the workflow audit trail.
                  </p>
                </div>
                <button
                  type="button"
                  className="inline-flex min-h-10 min-w-10 items-center justify-center rounded text-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                  aria-label="Cancel workflow action"
                  onClick={() => {
                    setPendingAction(null);
                    setActionReason('');
                  }}
                >
                  <X size={18} />
                </button>
              </div>

              <div className="mt-4 rounded-lg border border-border bg-base/70 px-3 py-2 text-caption text-muted">
                <div>Finding: <span className="text-text">{insight.title}</span></div>
                <div>Target state: <span className="text-text">{pendingActionLabel}</span></div>
                <div>Affected resources: <span className="text-text">{insight.affectedResources?.length ?? 0}</span></div>
                <div>Runtime evidence in window: <span className="text-text">{signals.length}</span></div>
              </div>

              <label className="mt-4 block text-caption font-semibold text-muted" htmlFor="risk-action-reason">
                {pendingActionRequiresReason ? 'Reason required' : 'Analyst note'}
              </label>
              <textarea
                id="risk-action-reason"
                value={actionReason}
                onChange={(event) => setActionReason(event.target.value)}
                className="mt-1 min-h-24 w-full rounded-lg border border-border bg-base px-3 py-2 text-body text-text outline-none focus:border-brand"
                placeholder={
                  pendingAction === 'resolve'
                    ? 'Describe the remediation or compensating control.'
                    : pendingAction === 'dismiss'
                      ? 'Explain why this finding is not actionable.'
                      : 'Optional local review note.'
                }
              />
              {pendingAction === 'acknowledge' && (
                <p className="mt-1 text-micro text-muted">
                  Current API persists acknowledgement state; notes are for review before submitting.
                </p>
              )}

              <div className="mt-4 flex flex-wrap justify-end gap-2">
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => {
                    setPendingAction(null);
                    setActionReason('');
                  }}
                >
                  Cancel
                </Button>
                <Button
                  size="sm"
                  disabled={pendingActionDisabled}
                  onClick={() => { if (pendingAction) void runAction(pendingAction); }}
                >
                  {actionBusy ? (
                    <span className="inline-flex items-center gap-1.5">
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      Submitting
                    </span>
                  ) : (
                    pendingActionLabel
                  )}
                </Button>
              </div>
            </div>
          </div>
        )}
      </div>
    </>
  );
};
