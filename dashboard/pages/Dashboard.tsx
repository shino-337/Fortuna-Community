import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { ArrowRight, FileText, Plus } from 'lucide-react';
import { Card } from '../design-system/components/Card';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Button } from '../components/ui/Button';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { useClusterStore } from '../store/clusterStore';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { useAuthStore } from '../store/authStore';
import { useOperationalMaterialization } from '../hooks/useOperationalMaterialization';
import { useInvestigationCases } from '../hooks/useInvestigationCases';
import { usePermUser } from '../hooks/usePermUser';
import { can, P } from '../lib/permissions';
import { PAGE_TITLES } from '../lib/pageTitles';
import { deriveUnifiedRiskLevelFromScore, getSeverityBarClass, getSeverityBadgeClass } from '../lib/severity';
import type { InvestigationCase } from '../store/investigationStore';
import type { Insight, PipelineHealth, ThreatVelocityPoint } from '../types';
import { ExecutiveBriefPanel } from './home/ExecutiveBriefPanel';
import { TREND_DAYS, useHomeData, type HomeSection } from './home/useHomeData';

const CLOSED_CASE_STATUSES: InvestigationCase['status'][] = ['RESOLVED', 'ARCHIVED'];

const CASE_STATUS_LABEL: Record<InvestigationCase['status'], string> = {
  OPEN: 'Open',
  TRIAGED: 'Triaged',
  ACTIVE: 'Active',
  CONTAINED: 'Contained',
  REMEDIATING: 'Remediating',
  RESOLVED: 'Resolved',
  ARCHIVED: 'Archived',
};

/** "5 min", "3 h", "2 d": how long ago an ISO timestamp was. */
function formatAge(iso?: string | null, now = Date.now()): string | null {
  if (!iso) return null;
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return null;
  const minutes = Math.max(0, Math.round((now - then) / 60000));
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} h`;
  return `${Math.round(hours / 24)} d`;
}

function windowText(sinceMinutes?: number): string {
  if (!sinceMinutes) return 'across all time';
  if (sinceMinutes % 1440 === 0) return `over the last ${sinceMinutes / 1440 === 1 ? '24 hours' : `${sinceMinutes / 1440} days`}`;
  if (sinceMinutes % 60 === 0) return `over the last ${sinceMinutes / 60 === 1 ? 'hour' : `${sinceMinutes / 60} hours`}`;
  return `over the last ${sinceMinutes} minute${sinceMinutes === 1 ? '' : 's'}`;
}

function plural(n: number, word: string): string {
  return `${n} ${word}${n === 1 ? '' : 's'}`;
}

function findingLevel(f: Insight): string {
  return (f.finalLevel || deriveUnifiedRiskLevelFromScore(f.totalScore ?? f.score) || '').toLowerCase();
}

function findingResource(f: Insight): string | null {
  const r = f.affectedResources?.[0];
  if (!r?.name) return f.clusterName ?? null;
  return r.namespace ? `${r.namespace}/${r.name}` : r.name;
}

const LoadFailed: React.FC<{ what: string; onRetry: () => void }> = ({ what, onRetry }) => (
  <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-6 text-body text-muted" role="alert">
    <span>{what} could not be loaded. Fortuna Core may be unreachable.</span>
    <Button size="sm" variant="secondary" onClick={onRetry}>Try again</Button>
  </div>
);

const RowsLoading: React.FC<{ rows: number }> = ({ rows }) => (
  <div className="divide-y divide-border/60" aria-busy="true" aria-label="Loading">
    {Array.from({ length: rows }, (_, i) => (
      <div key={i} className="px-4 py-3">
        <div className="h-4 w-2/3 animate-pulse rounded bg-surface-2" />
      </div>
    ))}
  </div>
);

interface Kpi {
  label: string;
  section: HomeSection<number>;
  hint: string;
  to?: string;
  tone?: 'critical';
}

const KpiTile: React.FC<{ kpi: Kpi }> = ({ kpi }) => {
  const { section } = kpi;
  const value = section.data != null ? section.data.toLocaleString() : section.loading ? '…' : '—';
  const body = (
    <>
      <span className="text-caption font-medium text-muted">{kpi.label}</span>
      <span
        className={`mt-1 block font-mono text-3xl font-semibold ${
          kpi.tone === 'critical' && (section.data ?? 0) > 0 ? 'text-red-400' : 'text-text'
        }`}
      >
        {value}
      </span>
      <span className="mt-1 block text-meta text-muted-2">{section.failed && section.data == null ? 'Could not load' : kpi.hint}</span>
    </>
  );
  const shell = 'block rounded-xl border border-border/80 bg-surface/40 px-4 py-3';
  return kpi.to ? (
    <Link to={kpi.to} className={`${shell} transition-colors hover:border-brand/50 hover:bg-surface-2/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70`}>
      {body}
    </Link>
  ) : (
    <div className={shell}>{body}</div>
  );
};

const LevelPill: React.FC<{ level: string }> = ({ level }) => (
  <span className={`inline-flex w-20 shrink-0 justify-center rounded border px-1.5 py-0.5 text-meta font-semibold capitalize ${getSeverityBadgeClass(level)}`}>
    {level || 'Unscored'}
  </span>
);

const TREND_SERIES = [
  { key: 'critical', label: 'Critical', bar: getSeverityBarClass('critical'), value: (p: ThreatVelocityPoint) => p.critical },
  { key: 'high', label: 'High', bar: getSeverityBarClass('high'), value: (p: ThreatVelocityPoint) => p.high },
  { key: 'medium', label: 'Medium and low', bar: getSeverityBarClass('medium'), value: (p: ThreatVelocityPoint) => p.medium + p.low },
  { key: 'unscored', label: 'Not scored yet', bar: 'bg-muted-2/60', value: (p: ThreatVelocityPoint) => p.unscored ?? 0 },
] as const;

/** New findings per day, stacked by risk level. */
const ExposureTrend: React.FC<{ points: ThreatVelocityPoint[] }> = ({ points }) => {
  const totals = points.map((p) => TREND_SERIES.reduce((sum, s) => sum + s.value(p), 0));
  const max = Math.max(1, ...totals);
  const grand = totals.reduce((a, b) => a + b, 0);
  const critical = points.reduce((a, p) => a + p.critical, 0);
  if (grand === 0) {
    return <p className="px-4 py-8 text-center text-body text-muted">No new findings in the last {TREND_DAYS} days.</p>;
  }
  return (
    <div className="px-4 pb-4 pt-3">
      <div className="mb-3 flex flex-wrap gap-x-4 gap-y-1 text-meta text-muted">
        {TREND_SERIES.map((s) => (
          <span key={s.key} className="inline-flex items-center gap-1.5">
            <span className={`h-2.5 w-2.5 rounded-sm ${s.bar}`} aria-hidden />
            {s.label}
          </span>
        ))}
      </div>
      <div
        className="flex h-40 items-end gap-[3px]"
        role="img"
        aria-label={`${plural(grand, 'new finding')} in the last ${TREND_DAYS} days, ${critical} at the critical risk level.`}
      >
        {points.map((p, i) => (
          <div
            key={p.date}
            className="flex h-full min-w-0 flex-1 flex-col-reverse"
            title={`${p.date}: ${plural(totals[i], 'new finding')} (${p.critical} critical, ${p.high} high)`}
          >
            {TREND_SERIES.map((s) => {
              const v = s.value(p);
              return v > 0 ? <div key={s.key} className={`${s.bar} first:rounded-b-sm last:rounded-t-sm`} style={{ height: `${(v / max) * 100}%` }} /> : null;
            })}
          </div>
        ))}
      </div>
      <div className="mt-2 flex justify-between text-meta text-muted-2">
        <span>{points[0]?.date}</span>
        <span>Today</span>
      </div>
    </div>
  );
};

const PIPELINE_STAGES: { label: string; pick: (p: PipelineHealth) => { status?: string; at: string | null } }[] = [
  { label: 'Findings and scans', pick: (p) => ({ status: p.layer1.status, at: p.layer1.lastRiskEngineEval ?? p.layer1.lastPceEval }) },
  { label: 'Runtime confirmation', pick: (p) => ({ status: p.layer2.status, at: p.layer2.lastStateChange }) },
  { label: 'Attack paths', pick: (p) => ({ status: p.layer3.status, at: p.layer3.lastPathComputation }) },
  { label: 'Risk scores', pick: (p) => ({ status: p.layer4.status, at: p.layer4.lastScoreCalc }) },
];

function pipelineBehind(p: PipelineHealth): string[] {
  return PIPELINE_STAGES.filter((s) => {
    const status = s.pick(p).status;
    return status === 'stale' || status === 'degraded';
  }).map((s) => s.label.toLowerCase());
}

/** Home: what needs the signed-in person's attention in the header scope, and where to go next. */
export const Dashboard: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);
  const sinceMinutes = timeWindowMinutes > 0 ? timeWindowMinutes : undefined;
  const user = useAuthStore((s) => s.user);
  const permUser = usePermUser();
  const { shellVariant, allowedRoutes } = useOperationalMaterialization();
  const canOpen = useCallback((path: string) => allowedRoutes.includes(path), [allowedRoutes]);
  const canReadPipeline = can(permUser, P.observabilityMetricsRead);
  const showPlatformCard = shellVariant === 'admin' && canReadPipeline;

  const home = useHomeData(selectedClusterId, sinceMinutes, { canReadPipeline, canTriage: can(permUser, P.findingsAck) });
  const cases = useInvestigationCases();

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  const { refresh } = home;
  usePolling(refresh, intervalMs, { refreshTrigger });
  // usePolling only re-runs on its interval, so reload as soon as the header scope changes.
  const [scopeKey, setScopeKey] = useState(`${selectedClusterId}|${sinceMinutes}`);
  useEffect(() => {
    const next = `${selectedClusterId}|${sinceMinutes}`;
    if (next === scopeKey) return;
    setScopeKey(next);
    void refresh();
  }, [selectedClusterId, sinceMinutes, scopeKey, refresh]);

  // `/reports` and the "Executive brief" links land on `/?section=brief`.
  const briefOpen = searchParams.get('section') === 'brief';
  const setBriefOpen = useCallback(
    (open: boolean) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (open) next.set('section', 'brief');
          else next.delete('section');
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );
  useEffect(() => {
    if (!briefOpen) return;
    const frame = window.requestAnimationFrame(() => {
      document.getElementById('executive-brief')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });
    return () => window.cancelAnimationFrame(frame);
  }, [briefOpen]);

  const scopeParams = useMemo(() => {
    const params = new URLSearchParams();
    if (selectedClusterId?.trim()) params.set('clusterId', selectedClusterId.trim());
    if (sinceMinutes) params.set('sinceMinutes', String(sinceMinutes));
    return params;
  }, [selectedClusterId, sinceMinutes]);
  const findingsUrl = (extra?: Record<string, string>) => {
    const params = new URLSearchParams(scopeParams);
    Object.entries(extra ?? {}).forEach(([k, v]) => params.set(k, v));
    const qs = params.toString();
    return qs ? `/risks?${qs}` : '/risks';
  };

  const me = user?.username ?? '';
  const myCases = useMemo(
    () =>
      cases.cases
        .filter((c) => !CLOSED_CASE_STATUSES.includes(c.status))
        .filter((c) => !!me && (c.owner === me || c.collaboration?.assignees?.includes(me)))
        .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt)),
    [cases.cases, me],
  );
  const now = Date.now();
  const myCasesPastDue = myCases.filter((c) => c.slaDueAt && new Date(c.slaDueAt).getTime() < now).length;
  const casesReady = cases.canRead && cases.source === 'server';

  const scopeName = selectedClusterId ? home.inventory.data?.clusterName ?? selectedClusterId : 'all your clusters';
  const queue = home.queue.data;

  const kpis: Kpi[] = [
    {
      label: 'Needs triage',
      section: { ...home.queue, data: queue ? queue.total : null },
      hint: 'Open and not yet acknowledged',
      to: canOpen('/risks') ? findingsUrl() : undefined,
    },
    {
      label: 'Critical open',
      section: home.criticalOpen,
      hint: 'Risk score 70 or higher',
      to: canOpen('/risks') ? findingsUrl({ finalLevel: 'critical' }) : undefined,
      tone: 'critical',
    },
    ...(cases.canRead
      ? [
          {
            label: 'My open cases',
            section: {
              data: casesReady ? myCases.length : null,
              loading: cases.loading,
              failed: !!cases.error,
            },
            hint: myCasesPastDue > 0 ? `${myCasesPastDue} past due` : 'Owned by or assigned to you',
            to: canOpen('/investigation') ? '/investigation' : undefined,
          },
        ]
      : []),
    {
      label: 'Exposed workloads',
      section: home.exposedWorkloads,
      hint: 'Workloads at high or critical risk',
      to: canOpen('/resources') ? '/resources' : undefined,
    },
  ];

  const pipeline = home.pipeline.data;
  const behind = pipeline ? pipelineBehind(pipeline) : [];
  const agents = home.inventory.data?.agents;
  const freshness = (() => {
    if (pipeline) {
      const scored = formatAge(pipeline.layer4.lastScoreCalc ?? pipeline.layer1.lastRiskEngineEval, now);
      if (behind.length > 0) return { tone: 'warning' as const, text: `Some data is behind: ${behind.join(', ')}.` };
      return {
        tone: 'ok' as const,
        text: `Data is fresh${agents != null ? `: ${plural(agents, 'agent')} reporting` : ''}${scored ? `, risk scores updated ${scored === 'just now' ? 'just now' : `${scored} ago`}` : ''}.`,
      };
    }
    if (!home.loadedAt) return null;
    return {
      tone: 'ok' as const,
      text: `${agents != null ? `${plural(agents, 'agent')} reporting. ` : ''}Updated ${home.loadedAt.toLocaleTimeString()}.`,
    };
  })();

  return (
    <PageLayout
      title={PAGE_TITLES.home}
      description={`What needs your attention in ${scopeName} ${windowText(sinceMinutes)}.`}
      actions={
        <Button variant="secondary" size="sm" onClick={() => setBriefOpen(true)}>
          <FileText className="mr-1.5 h-4 w-4" aria-hidden />
          Export brief
        </Button>
      }
    >
      <div className="grid gap-5">
        <div className={`grid grid-cols-2 gap-3 ${kpis.length === 4 ? 'xl:grid-cols-4' : 'xl:grid-cols-3'}`}>
          {kpis.map((kpi) => (
            <KpiTile key={kpi.label} kpi={kpi} />
          ))}
        </div>

        <div className="grid gap-5 xl:grid-cols-3">
          <Card
            className="xl:col-span-2"
            contentClassName="p-0"
            title="Needs your attention"
            description="Open findings nobody has acknowledged, highest risk first."
            actions={
              canOpen('/risks') ? (
                <div className="flex flex-wrap items-center gap-3">
                  {home.assignedToMe.data ? (
                    <Link to={findingsUrl({ view: 'mine' })} className="text-caption font-semibold text-text hover:text-brand">
                      {home.assignedToMe.data.toLocaleString()} assigned to you
                    </Link>
                  ) : null}
                  <Link to={findingsUrl()} className="inline-flex items-center gap-1 text-caption font-semibold text-brand hover:text-brand/90">
                    Open queue <ArrowRight className="h-3.5 w-3.5" aria-hidden />
                  </Link>
                </div>
              ) : null
            }
          >
            {home.queue.failed && !queue ? (
              <LoadFailed what="Findings" onRetry={() => void refresh()} />
            ) : !queue ? (
              <RowsLoading rows={5} />
            ) : queue.items.length === 0 ? (
              <p className="px-4 py-8 text-center text-body text-muted">
                Nothing needs triage {windowText(sinceMinutes)}.
                {sinceMinutes ? ' Widen the time window in the header to see older findings.' : ''}
              </p>
            ) : (
              <ul className="divide-y divide-border/60">
                {queue.items.map((f) => {
                  const resource = findingResource(f);
                  const age = formatAge(f.timestamp, now);
                  return (
                    <li key={f.id}>
                      <Link
                        to={`/risks/${encodeURIComponent(f.id)}`}
                        className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-surface-2/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand/70"
                      >
                        <LevelPill level={findingLevel(f)} />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-body font-medium text-text">{f.title}</span>
                          {resource ? <span className="block truncate font-mono text-meta text-muted">{resource}</span> : null}
                        </span>
                        {age ? <span className="shrink-0 text-meta text-muted-2">{age}</span> : null}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            )}
          </Card>

          <div className="grid content-start gap-5">
            {cases.canRead ? (
              <Card
                contentClassName="p-0"
                title="My cases"
                actions={
                  canOpen('/investigation') ? (
                    <Link to="/investigation" className="inline-flex items-center gap-1 text-caption font-semibold text-brand hover:text-brand/90">
                      All cases <ArrowRight className="h-3.5 w-3.5" aria-hidden />
                    </Link>
                  ) : null
                }
              >
                {cases.error && !casesReady ? (
                  <LoadFailed what="Cases" onRetry={() => void cases.refresh()} />
                ) : !casesReady ? (
                  <RowsLoading rows={3} />
                ) : myCases.length === 0 ? (
                  <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-6 text-body text-muted">
                    <span>No open cases are assigned to you.</span>
                    {cases.canWrite && canOpen('/investigation') ? (
                      <Link to="/investigation" className="inline-flex items-center gap-1 text-caption font-semibold text-brand hover:text-brand/90">
                        <Plus className="h-3.5 w-3.5" aria-hidden /> New case
                      </Link>
                    ) : null}
                  </div>
                ) : (
                  <ul className="divide-y divide-border/60">
                    {myCases.slice(0, 5).map((c) => {
                      const findings = c.entities.filter((e) => e.type === 'finding').length;
                      const pastDue = c.slaDueAt && new Date(c.slaDueAt).getTime() < now;
                      return (
                        <li key={c.id}>
                          <Link
                            to={`/investigation?case=${encodeURIComponent(c.id)}`}
                            className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-surface-2/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand/70"
                          >
                            <span className="min-w-0 flex-1">
                              <span className="block truncate text-body font-medium text-text">{c.title || 'Untitled case'}</span>
                              <span className="block text-meta text-muted">
                                {plural(findings, 'finding')}
                                {pastDue ? <span className="ml-2 font-semibold text-amber-400">Past due</span> : null}
                              </span>
                            </span>
                            <span className="shrink-0 rounded border border-border px-1.5 py-0.5 text-meta text-muted">{CASE_STATUS_LABEL[c.status]}</span>
                          </Link>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </Card>
            ) : null}

            {showPlatformCard ? (
              <Card
                contentClassName="p-0"
                title="Platform"
                actions={
                  canOpen('/monitoring') ? (
                    <Link to="/monitoring" className="inline-flex items-center gap-1 text-caption font-semibold text-brand hover:text-brand/90">
                      Open Platform <ArrowRight className="h-3.5 w-3.5" aria-hidden />
                    </Link>
                  ) : null
                }
              >
                {home.pipeline.failed && !pipeline ? (
                  <LoadFailed what="Platform health" onRetry={() => void refresh()} />
                ) : !pipeline ? (
                  <RowsLoading rows={4} />
                ) : (
                  <dl className="divide-y divide-border/60 text-body">
                    <div className="flex items-center justify-between gap-3 px-4 py-2.5">
                      <dt className="text-muted">Agents reporting</dt>
                      <dd className="font-mono text-text">{agents ?? '—'}</dd>
                    </div>
                    {PIPELINE_STAGES.map((stage) => {
                      const { status, at } = stage.pick(pipeline);
                      const age = formatAge(at, now);
                      const bad = status === 'stale' || status === 'degraded';
                      return (
                        <div key={stage.label} className="flex items-center justify-between gap-3 px-4 py-2.5">
                          <dt className="text-muted">{stage.label}</dt>
                          <dd className={bad ? 'font-semibold text-amber-400' : 'text-text'}>
                            {bad ? 'Behind' : 'Up to date'}
                            {age ? <span className="ml-2 text-meta font-normal text-muted-2">{age === 'just now' ? age : `${age} ago`}</span> : null}
                          </dd>
                        </div>
                      );
                    })}
                  </dl>
                )}
              </Card>
            ) : null}

            {freshness && !showPlatformCard ? (
              <p
                className={`flex flex-wrap items-center gap-x-2 gap-y-1 rounded-lg border px-3 py-2 text-caption ${
                  freshness.tone === 'warning' ? 'border-amber-500/30 bg-amber-500/10 text-amber-100' : 'border-border/80 bg-surface/30 text-muted'
                }`}
              >
                <span className={`h-2 w-2 shrink-0 rounded-full ${freshness.tone === 'warning' ? 'bg-amber-400' : 'bg-emerald-500'}`} aria-hidden />
                <span className="min-w-0 flex-1">{freshness.text}</span>
                {canOpen('/monitoring') ? (
                  <Link to="/monitoring" className="font-semibold text-brand hover:text-brand/90">Platform</Link>
                ) : null}
              </p>
            ) : null}
          </div>
        </div>

        <Card contentClassName="p-0" title={`Exposure trend, ${TREND_DAYS} days`} description="New findings per day, by risk level.">
          {home.trend.failed && !home.trend.data ? (
            <LoadFailed what="The trend" onRetry={() => void refresh()} />
          ) : !home.trend.data ? (
            <div className="h-48 animate-pulse" aria-busy="true" aria-label="Loading" />
          ) : (
            <ExposureTrend points={home.trend.data} />
          )}
        </Card>

        <ExecutiveBriefPanel open={briefOpen} onToggle={setBriefOpen} />
      </div>
    </PageLayout>
  );
};
