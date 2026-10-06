import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { RefreshCw } from 'lucide-react';
import { PageLayout } from '../../design-system/layouts/PageLayout';
import { Button } from '../../components/ui/Button';
import { usePolling, REFRESH_INTERVALS } from '../../hooks/usePolling';
import { useRefreshIntervalStore } from '../../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../../store/refreshTriggerStore';
import { usePermUser } from '../../hooks/usePermUser';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';
import { can, P } from '../../lib/permissions';
import { formatDateTime } from '../../lib/display';
import { PAGE_TITLES } from '../../lib/pageTitles';
import type { Agent, ErrorLog } from '../../types';
import { usePlatformData } from './usePlatformData';
import { ErrorLogTable } from './ErrorLogTable';

type Tone = 'ok' | 'warn' | 'bad' | 'unknown';

const TONE_DOT: Record<Tone, string> = {
  ok: 'bg-emerald-400',
  warn: 'bg-amber-400',
  bad: 'bg-critical',
  unknown: 'bg-muted',
};
const TONE_TEXT: Record<Tone, string> = {
  ok: 'text-emerald-300',
  warn: 'text-amber-300',
  bad: 'text-critical',
  unknown: 'text-muted',
};

/** Inventory older than this is called behind. */
const INVENTORY_STALE_MINUTES = 60;
const CERT_WARN_DAYS = 30;
const DAY_MS = 24 * 60 * 60 * 1000;

function ago(iso: string | null | undefined, now: number): string {
  if (!iso) return 'never';
  const t = Date.parse(iso);
  if (!Number.isFinite(t)) return 'unknown';
  const s = Math.max(0, Math.round((now - t) / 1000));
  if (s < 60) return `${s} s ago`;
  const m = Math.round(s / 60);
  if (m < 60) return `${m} min ago`;
  const h = Math.round(m / 60);
  if (h < 48) return `${h} h ago`;
  return `${Math.round(h / 24)} days ago`;
}

function minutesSince(iso: string | null | undefined, now: number): number | null {
  if (!iso) return null;
  const t = Date.parse(iso);
  return Number.isFinite(t) ? (now - t) / 60000 : null;
}

function layerTone(status: string | undefined): Tone {
  if (status === 'healthy') return 'ok';
  if (status === 'degraded' || status === 'stale') return 'warn';
  return 'unknown';
}

interface ClusterRow {
  id: string;
  name: string;
  up: number;
  total: number;
  lastHeartbeat: string | null;
  /** The oldest heartbeat among agents that are not up: when the lagging agent was last heard from. */
  laggingHeartbeat: string | null;
  versions: string[];
  tone: Tone;
  label: string;
}

function clusterRows(agents: Agent[]): ClusterRow[] {
  const by = new Map<string, Agent[]>();
  for (const a of agents) {
    const key = a.clusterId || 'unassigned';
    by.set(key, [...(by.get(key) ?? []), a]);
  }
  return [...by.entries()]
    .map(([id, list]) => {
      const up = list.filter((a) => a.status === 'up').length;
      const down = list.filter((a) => a.status === 'down').length;
      const last = list.map((a) => a.lastHeartbeat).filter(Boolean).sort().at(-1) ?? null;
      const lagging = list.filter((a) => a.status !== 'up').map((a) => a.lastHeartbeat).filter(Boolean).sort()[0] ?? null;
      const tone: Tone = down === list.length ? 'bad' : up === list.length ? 'ok' : 'warn';
      return {
        id,
        name: list.find((a) => a.clusterName)?.clusterName ?? id,
        up,
        total: list.length,
        lastHeartbeat: last,
        laggingHeartbeat: lagging,
        versions: [...new Set(list.map((a) => a.version).filter((v): v is string => Boolean(v)))].sort(),
        tone,
        label: tone === 'ok' ? 'Reporting' : tone === 'bad' ? 'Not reporting' : `${list.length - up} agent${list.length - up === 1 ? '' : 's'} behind`,
      };
    })
    .sort((a, b) => a.name.localeCompare(b.name));
}

interface Issue {
  tone: Tone;
  text: string;
  to?: string;
  action?: string;
}

function Dot({ tone }: { tone: Tone }) {
  return <span className={`inline-block h-2 w-2 shrink-0 rounded-full ${TONE_DOT[tone]}`} aria-hidden />;
}

function Panel({ title, aside, children, id }: { title: string; aside?: React.ReactNode; children: React.ReactNode; id?: string }) {
  return (
    <section id={id} className="rounded-lg border border-border bg-surface" aria-label={title}>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3">
        <h2 className="text-body font-semibold text-text">{title}</h2>
        {aside ? <div className="text-caption text-muted">{aside}</div> : null}
      </div>
      {children}
    </section>
  );
}

function Unavailable({ what }: { what: string }) {
  return <p className="px-4 py-3 text-caption text-amber-200">{what} could not be loaded. The last known values are shown if there are any.</p>;
}

/** Is Fortuna collecting complete, fresh data from every cluster? */
export const Platform: React.FC = () => {
  const permUser = usePermUser();
  const { allowedRoutes } = useOperationalMaterialization();
  const access = {
    metrics: can(permUser, P.observabilityMetricsRead),
    agents: can(permUser, P.observabilityAgentsRead),
    logs: can(permUser, P.observabilityLogsRead),
    certificates: can(permUser, P.inventoryRead),
    clusterScoped: (permUser?.operationalScope?.clusters?.length ?? 0) > 0,
  };
  const canOpenCertificates = allowedRoutes.includes('/monitoring/certificates');
  const canOpenAudit = allowedRoutes.includes('/governance');
  const data = usePlatformData(access);
  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(data.refresh, intervalMs, { refreshTrigger });

  const [now, setNow] = useState(() => Date.now());
  useEffect(() => setNow(Date.now()), [data.loadedAt]);

  const clusters = useMemo(() => clusterRows(data.agents.data ?? []), [data.agents.data]);
  const pipeline = data.pipeline.data;
  const catalog = data.integrity.data?.catalog ?? null;
  const runtime = data.integrity.data?.runtime ?? null;
  const lastScan = data.sync.data?.lastScan ?? null;
  const inventoryAge = minutesSince(lastScan, now);

  const stages: { label: string; at: string | null | undefined; tone: Tone; detail: string }[] = [];
  if (data.sync.data || data.sync.failed) {
    stages.push({
      label: 'Inventory',
      at: lastScan,
      tone: lastScan == null ? 'unknown' : (inventoryAge ?? 0) > INVENTORY_STALE_MINUTES ? 'warn' : 'ok',
      detail: data.sync.data ? `${data.sync.data.resources.pods.toLocaleString()} pods, ${data.sync.data.resources.sas.toLocaleString()} service accounts` : '',
    });
  }
  if (runtime) {
    stages.push({
      label: 'Runtime events',
      at: runtime.lastRuntimeEventAt,
      tone: runtime.status === 'healthy' ? 'ok' : runtime.status === 'unavailable' ? 'bad' : 'warn',
      detail: `${runtime.runtimeEventsCount.toLocaleString()} events, Falco ${runtime.falcoStatus === 'active' ? 'active' : runtime.falcoStatus === 'no-events' ? 'quiet' : 'unavailable'}`,
    });
  }
  if (pipeline) {
    stages.push(
      { label: 'Findings', at: pipeline.layer1.lastRiskEngineEval ?? pipeline.layer1.lastPceEval, tone: layerTone(pipeline.layer1.status), detail: `${pipeline.layer1.insightCount} findings` },
      { label: 'Runtime state', at: pipeline.layer2.lastStateChange, tone: layerTone(pipeline.layer2.status), detail: `${pipeline.layer2.exploitedCapCount} exploited capabilities` },
      { label: 'Attack paths', at: pipeline.layer3.lastPathComputation, tone: layerTone(pipeline.layer3.status), detail: `${pipeline.layer3.totalPaths} paths, ${pipeline.layer3.criticalPaths} critical` },
      { label: 'Risk scores', at: pipeline.layer4.lastScoreCalc, tone: layerTone(pipeline.layer4.status), detail: `${pipeline.layer4.resourcesScored} resources scored` },
    );
  }
  if (catalog) {
    stages.push({
      label: 'Vulnerability feed',
      at: catalog.mirrorUpdatedAt ?? catalog.lastCveUpdatedAt,
      tone: catalog.status === 'healthy' ? 'ok' : catalog.status === 'unavailable' ? 'bad' : 'warn',
      detail: `${catalog.cvesCount.toLocaleString()} CVEs`,
    });
  }

  const recent: ErrorLog[] = useMemo(
    () => (data.recentErrors.data ?? []).filter((l) => {
      const t = Date.parse(l.time);
      return Number.isFinite(t) && now - t <= DAY_MS && String(l.level).toLowerCase() === 'error';
    }),
    [data.recentErrors.data, now],
  );
  const groupedErrors = useMemo(() => {
    const m = new Map<string, { message: string; source: string; count: number; last: string }>();
    for (const l of recent) {
      const key = `${l.source ?? ''}|${l.message}`;
      const cur = m.get(key);
      if (cur) {
        cur.count += 1;
        if (l.time > cur.last) cur.last = l.time;
      } else m.set(key, { message: l.message, source: l.source ?? '', count: 1, last: l.time });
    }
    return [...m.values()].sort((a, b) => b.count - a.count).slice(0, 6);
  }, [recent]);

  const certs = data.certificates.data ?? [];

  const issues: Issue[] = [];
  for (const c of clusters) {
    if (c.tone !== 'ok') issues.push({ tone: c.tone, text: `${c.name}: ${c.label.toLowerCase()}, last heard ${ago(c.laggingHeartbeat, now)}.` });
  }
  for (const s of stages) {
    if (s.tone === 'warn' || s.tone === 'bad') issues.push({ tone: s.tone, text: `${s.label} last updated ${ago(s.at, now)}.` });
  }
  for (const c of certs) {
    if (c.status === 'expired') issues.push({ tone: 'bad', text: `${c.name} has expired.`, to: canOpenCertificates ? '/monitoring/certificates' : undefined, action: 'Review certificate' });
    else if (c.daysRemaining != null && c.daysRemaining <= CERT_WARN_DAYS)
      issues.push({ tone: 'warn', text: `${c.name} expires in ${c.daysRemaining} days.`, to: canOpenCertificates ? '/monitoring/certificates' : undefined, action: 'Review certificate' });
  }
  const failedSections = [data.agents, data.pipeline, data.sync, data.integrity, data.certificates, data.recentErrors].filter((s) => s.failed).length;
  const worst: Tone = issues.some((i) => i.tone === 'bad') ? 'bad' : issues.length > 0 ? 'warn' : failedSections > 0 || !data.loadedAt ? 'unknown' : 'ok';
  const verdict =
    !data.loadedAt
      ? 'Checking…'
      : worst === 'bad'
        ? 'Needs attention'
        : issues.length > 0
          ? `Healthy, ${issues.length} warning${issues.length === 1 ? '' : 's'}`
          : failedSections > 0
            ? 'Partly unknown'
            : 'Healthy';

  return (
    <PageLayout
      title={PAGE_TITLES.monitoring}
      description="Is Fortuna collecting complete, fresh data from every cluster?"
      actions={
        <div className="flex flex-wrap gap-2">
          {canOpenAudit ? (
            <Link to="/governance" className="inline-flex min-h-9 items-center rounded-lg border border-border px-3 text-caption font-semibold text-text hover:border-muted">
              Audit log
            </Link>
          ) : null}
          <Button size="sm" variant="secondary" onClick={() => void data.refresh()}>
            <RefreshCw className="mr-1 h-4 w-4" aria-hidden />
            Refresh
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-5">
        <section className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-border bg-surface px-4 py-3" aria-label="Platform status">
          <span className={`flex items-center gap-2 text-section-title ${TONE_TEXT[worst]}`}>
            <Dot tone={worst} />
            {verdict}
          </span>
          <span className="min-w-[14rem] flex-1 text-body text-muted">
            {issues[0]?.text ??
              (failedSections > 0
                ? `${failedSections} section${failedSections === 1 ? '' : 's'} could not be loaded.`
                : data.loadedAt
                  ? 'Every cluster is reporting and every stage is current.'
                  : '')}
            {issues.length > 1 ? ` ${issues.length - 1} more below.` : ''}
          </span>
          {issues[0]?.to ? (
            <Link to={issues[0].to} className="text-caption font-semibold text-brand hover:underline">
              {issues[0].action}
            </Link>
          ) : null}
          {data.loadedAt ? <span className="text-meta text-muted">Checked {formatDateTime(data.loadedAt.toISOString())}</span> : null}
        </section>

        {access.agents ? (
          <Panel title="Clusters and agents" aside={clusters.length > 0 ? `${clusters.reduce((n, c) => n + c.total, 0)} agents` : undefined}>
            {data.agents.failed ? <Unavailable what="Agent status" /> : null}
            {clusters.length === 0 && !data.agents.failed ? (
              <p className="px-4 py-3 text-caption text-muted">
                No agent has reported yet. Install the agent in a cluster; it appears here after its first heartbeat.
              </p>
            ) : clusters.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[36rem] text-left text-body">
                  <thead className="text-meta uppercase tracking-wider text-muted">
                    <tr className="border-b border-border">
                      <th scope="col" className="px-4 py-2 font-semibold">Cluster</th>
                      <th scope="col" className="px-4 py-2 font-semibold">Agents</th>
                      <th scope="col" className="px-4 py-2 font-semibold">Last heartbeat</th>
                      <th scope="col" className="px-4 py-2 font-semibold">Agent version</th>
                      <th scope="col" className="px-4 py-2 font-semibold">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {clusters.map((c) => (
                      <tr key={c.id} className="border-b border-border/60 last:border-b-0">
                        <td className="px-4 py-2.5 font-semibold text-text">
                          {c.id === 'unassigned' ? c.name : <Link to={`/clusters/${encodeURIComponent(c.id)}`} className="hover:text-brand">{c.name}</Link>}
                        </td>
                        <td className="px-4 py-2.5 tabular-nums">{c.up} / {c.total}</td>
                        <td className="px-4 py-2.5 font-mono text-caption text-muted">{ago(c.lastHeartbeat, now)}</td>
                        <td className="px-4 py-2.5 font-mono text-caption text-muted">{c.versions.join(', ') || 'unknown'}</td>
                        <td className="px-4 py-2.5">
                          <span className={`inline-flex items-center gap-1.5 text-caption ${TONE_TEXT[c.tone]}`}>
                            <Dot tone={c.tone} />
                            {c.label}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : null}
          </Panel>
        ) : null}

        {access.metrics ? (
          <Panel title="Pipeline" aside="When each stage last produced data">
            {data.pipeline.failed || data.sync.failed || data.integrity.failed ? <Unavailable what="Part of the pipeline status" /> : null}
            {stages.length === 0 ? (
              <p className="px-4 py-3 text-caption text-muted">{data.loadedAt ? 'No pipeline data yet.' : 'Loading…'}</p>
            ) : (
              <ol className="grid gap-3 p-4 sm:grid-cols-2 lg:grid-cols-4 2xl:grid-cols-7">
                {stages.map((s, i) => (
                  <li key={s.label} className="flex flex-col gap-1 rounded-lg border border-border bg-base/40 p-3">
                    <span className="text-meta text-muted">{i + 1} · {s.label}</span>
                    <span className="text-card-title text-text">{ago(s.at, now)}</span>
                    <span className={`flex items-center gap-1.5 text-meta ${TONE_TEXT[s.tone]}`}>
                      <Dot tone={s.tone} />
                      {s.tone === 'ok' ? 'On time' : s.tone === 'unknown' ? 'No data yet' : s.tone === 'bad' ? 'Unavailable' : 'Behind'}
                    </span>
                    {s.detail ? <span className="text-meta text-muted">{s.detail}</span> : null}
                  </li>
                ))}
              </ol>
            )}
          </Panel>
        ) : null}

        <div className="grid gap-5 lg:grid-cols-2">
          {access.certificates ? (
            <Panel
              title="Certificates"
              aside={canOpenCertificates ? <Link to="/monitoring/certificates" className="font-semibold text-brand hover:underline">Manage certificates</Link> : undefined}
            >
              {data.certificates.failed ? <Unavailable what="Certificates" /> : null}
              {certs.length === 0 && !data.certificates.failed ? (
                <p className="px-4 py-3 text-caption text-muted">TLS is not enabled on Core, so there is no certificate to track.</p>
              ) : (
                <ul>
                  {certs.map((c) => {
                    const tone: Tone = c.status === 'expired' ? 'bad' : c.daysRemaining != null && c.daysRemaining <= CERT_WARN_DAYS ? 'warn' : 'ok';
                    return (
                      <li key={c.id} className="flex items-center gap-3 border-b border-border/60 px-4 py-2.5 text-body last:border-b-0">
                        <Dot tone={tone} />
                        <span className="min-w-0 flex-1 truncate text-text">{c.name}</span>
                        <span className={`text-caption ${tone === 'ok' ? 'text-muted' : TONE_TEXT[tone]}`}>
                          {c.status === 'expired' ? 'Expired' : c.daysRemaining != null ? `${c.daysRemaining} days left` : 'Unknown expiry'}
                        </span>
                      </li>
                    );
                  })}
                </ul>
              )}
            </Panel>
          ) : null}

          {access.logs ? (
            <Panel title="Errors, last 24 hours" aside={recent.length > 0 ? `${recent.length} total` : undefined}>
              {data.recentErrors.failed ? <Unavailable what="Error logs" /> : null}
              {groupedErrors.length === 0 && !data.recentErrors.failed ? (
                <p className="px-4 py-3 text-caption text-muted">{data.loadedAt ? 'No errors in the last 24 hours.' : 'Loading…'}</p>
              ) : (
                <ul>
                  {groupedErrors.map((g) => (
                    <li key={`${g.source}|${g.message}`} className="flex items-center gap-3 border-b border-border/60 px-4 py-2.5 text-body last:border-b-0">
                      <span className="w-12 shrink-0 font-mono text-caption text-muted">×{g.count}</span>
                      <span className="min-w-0 flex-1 truncate text-text" title={g.message}>{g.message}</span>
                      <span className="shrink-0 font-mono text-meta text-muted">{g.source || 'core'}</span>
                    </li>
                  ))}
                </ul>
              )}
            </Panel>
          ) : null}
        </div>

        {access.logs ? <ErrorLogTable /> : null}
      </div>
    </PageLayout>
  );
};
