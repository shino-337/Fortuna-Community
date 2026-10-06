import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Search, X } from 'lucide-react';
import clsx from 'clsx';
import { api, isApiError } from '../lib/api';
import { useClusterStore } from '../store/clusterStore';
import { useClusters } from '../hooks/useClusters';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Table, type TableColumn } from '../design-system/components/Table';
import { Button } from '../components/ui/Button';
import { Pagination } from '../components/Pagination';
import { PageEmpty, PageError } from '../design-system/components/PageStatus';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { NetworkTopologyGraph } from '../components/NetworkTopologyGraph';
import { PAGE_TITLES } from '../lib/pageTitles';
import type { NetworkActivityDestinationRow, NetworkActivityTalkerRow } from '../types';
import type { SemanticVisibilityState } from '../lib/visibilityEngine';
import { podDetailPath } from '../lib/podRoute';
import { findingsForResourcePath } from '../lib/entityLinks';
import { useOperationalMaterialization } from '../hooks/useOperationalMaterialization';
import { FlowPanel, type FlowSelection } from './network/FlowPanel';
import { destKind, destLabel, flagFlow, flowKey, relativeTime, SINCE_OPTIONS, type FlowRow, type PodFacts } from './network/flows';

type NetworkDataIssueKind = 'unauthenticated' | 'forbidden' | 'cluster_scope' | 'load_failed';

type NetworkDataIssue = {
  kind: NetworkDataIssueKind;
  title: string;
  description: string;
  detail?: string;
  actionLabel: string;
};

function classifyNetworkDataIssue(error: unknown, fallbackTitle = 'Could not load network activity'): NetworkDataIssue {
  if (isApiError(error)) {
    if (error.status === 401) {
      const code = error.body?.code;
      const sessionDetail = code ? `Core returned ${code}.` : 'Core rejected the current JWT.';
      return {
        kind: 'unauthenticated',
        title: 'Session is not active',
        description: 'Sign in again to load runtime network activity.',
        detail: sessionDetail,
        actionLabel: 'Go to login',
      };
    }
    if (error.status === 403 && error.body?.reason === 'cluster_scope') {
      const cluster = error.body.required_cluster_id;
      return {
        kind: 'cluster_scope',
        title: 'Cluster outside your scope',
        description: cluster
          ? `Your account is not scoped for cluster ${cluster}. Select an allowed cluster or ask an administrator to update your scope.`
          : 'Your account is not scoped for this cluster. Select an allowed cluster or ask an administrator to update your scope.',
        detail: 'Required scope: cluster access.',
        actionLabel: 'Refresh',
      };
    }
    if (error.status === 403) {
      const required = error.body?.required_permission ?? error.body?.required_permissions?.join(', ');
      return {
        kind: 'forbidden',
        title: 'Permission required',
        description: required
          ? `This view requires ${required}. Ask an administrator to update your Fortuna role or permissions.`
          : 'Your account does not include permission for this network activity API.',
        detail: 'Core returned 403 forbidden.',
        actionLabel: 'Refresh',
      };
    }
    return {
      kind: 'load_failed',
      title: fallbackTitle,
      description: error.status >= 500
        ? 'Core returned a server error while loading network activity.'
        : error.message || 'The network activity request failed.',
      detail: `HTTP ${error.status}`,
      actionLabel: 'Retry',
    };
  }
  return {
    kind: 'load_failed',
    title: fallbackTitle,
    description: error instanceof Error ? error.message : 'The request failed before Core returned network activity data.',
    actionLabel: 'Retry',
  };
}


function NetworkDataIssuePanel({
  issue,
  onAction,
  className = '',
}: {
  issue: NetworkDataIssue;
  onAction: () => void;
  className?: string;
}) {
  if (issue.kind === 'load_failed') {
    return (
      <div className={`flex min-h-0 flex-1 items-center justify-center p-4 ${className}`}>
        <PageError
          title={issue.title}
          description={`${issue.description}${issue.detail ? ` ${issue.detail}` : ''}`}
          action={
            <Button variant="secondary" size="sm" type="button" onClick={onAction}>
              {issue.actionLabel}
            </Button>
          }
        />
      </div>
    );
  }

  const state: SemanticVisibilityState =
    issue.kind === 'cluster_scope'
      ? 'no_scope'
      : 'no_permission';

  return (
    <div className={`flex min-h-0 flex-1 items-center justify-center p-4 ${className}`}>
      <SemanticEmptyState
        state={state}
        title={issue.title}
        reason={`${issue.description}${issue.detail ? ` ${issue.detail}` : ''}`}
        action={
          <Button variant="secondary" size="sm" type="button" onClick={onAction}>
            {issue.actionLabel}
          </Button>
        }
      />
    </div>
  );
}


type View = 'table' | 'map';
type Group = 'pod' | 'dest';

/** Old `?tab=` values from the three-tab page. */
const LEGACY_TABS: Record<string, View> = { topology: 'map', pods: 'table', connections: 'table' };

/** Flows loaded once per filter change for the flagged strip, the summary line and the map. */
const OVERVIEW_EDGE_LIMIT = 500;
const MAP_SUMMARY_LIMIT = 200;
const PAGE_SIZES = [25, 50, 100, 200];

function intParam(v: string | null, fallback: number): number {
  const n = Number(v);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : fallback;
}

function KindTag({ kind }: { kind: string }) {
  if (kind === 'external') return <span className="rounded border border-amber-500/50 px-1 text-meta text-amber-300">external</span>;
  if (kind === 'service') return <span className="rounded border border-border px-1 text-meta text-muted">service</span>;
  return null;
}

function Segmented<T extends string>({ value, options, onChange, label }: { value: T; options: { id: T; label: string }[]; onChange: (v: T) => void; label: string }) {
  return (
    <div role="group" aria-label={label} className="inline-flex w-fit rounded-lg border border-border bg-base/60 p-1">
      {options.map((o) => (
        <button
          key={o.id}
          type="button"
          aria-pressed={value === o.id}
          onClick={() => onChange(o.id)}
          className={clsx(
            'min-h-8 rounded-md px-3 text-caption font-semibold transition-colors',
            value === o.id ? 'bg-surface-2 text-text shadow-sm' : 'text-muted hover:text-text',
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

/** Network: which pod talks to what, flagged flows first, each flow leading to its pod, findings and attack paths. */
export function NetworkActivity() {
  const [params, setParams] = useSearchParams();
  const { allowedRoutes } = useOperationalMaterialization();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const setSelectedClusterId = useClusterStore((s) => s.setSelectedClusterId);
  const { clusters } = useClusters();
  const clusterId = selectedClusterId ?? '';
  const prevCluster = useRef(clusterId);

  // Deep links: ?clusterId= selects the cluster in the header; old ?tab= becomes ?view=.
  useEffect(() => {
    const tab = params.get('tab');
    const linkedCluster = params.get('clusterId')?.trim();
    if (!tab && !linkedCluster) return;
    const next = new URLSearchParams(params);
    if (tab) {
      next.delete('tab');
      if (LEGACY_TABS[tab] === 'map') next.set('view', 'map');
    }
    if (linkedCluster) {
      next.delete('clusterId');
      prevCluster.current = linkedCluster;
      setSelectedClusterId(linkedCluster);
    }
    setParams(next, { replace: true });
  }, [params, setParams, setSelectedClusterId]);

  useEffect(() => {
    if (selectedClusterId == null && clusters.length === 1) setSelectedClusterId(String(clusters[0].id));
  }, [clusters, selectedClusterId, setSelectedClusterId]);

  const view: View = params.get('view') === 'map' ? 'map' : 'table';
  const group: Group = params.get('group') === 'dest' ? 'dest' : 'pod';
  const namespace = params.get('namespace') ?? '';
  const q = params.get('q') ?? '';
  const podUid = params.get('podUid') ?? '';
  const rawSince = intParam(params.get('since'), 15);
  const sinceMinutes = SINCE_OPTIONS.some((o) => o.value === rawSince) ? rawSince : 15;
  const page = intParam(params.get('page'), 1);
  const pageSize = PAGE_SIZES.includes(intParam(params.get('size'), 50)) ? intParam(params.get('size'), 50) : 50;

  const patch = useCallback(
    (values: Record<string, string | number>) => {
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          for (const [k, v] of Object.entries(values)) {
            const s = String(v);
            const isDefault = !s || (k === 'page' && s === '1') || (k === 'since' && s === '15') || (k === 'size' && s === '50') || (k === 'view' && s === 'table') || (k === 'group' && s === 'pod');
            if (isDefault) next.delete(k);
            else next.set(k, s);
          }
          return next;
        },
        { replace: true },
      );
    },
    [setParams],
  );

  // Pod and namespace filters belong to one cluster.
  useEffect(() => {
    if (prevCluster.current === clusterId) return;
    prevCluster.current = clusterId;
    patch({ namespace: '', podUid: '', q: '', page: 1 });
  }, [clusterId, patch]);

  const [searchDraft, setSearchDraft] = useState(q);
  useEffect(() => setSearchDraft(q), [q]);
  useEffect(() => {
    if (searchDraft === q) return;
    const t = window.setTimeout(() => patch({ q: searchDraft.trim(), page: 1 }), 400);
    return () => window.clearTimeout(t);
  }, [searchDraft, q, patch]);

  const [refreshKey, setRefreshKey] = useState(0);
  const [selection, setSelection] = useState<FlowSelection | null>(null);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [pods, setPods] = useState<Record<string, PodFacts>>({});

  // Names, namespaces and open-finding counts for the cluster's pods.
  useEffect(() => {
    if (!clusterId) return;
    let cancelled = false;
    api
      .getClusterInventoryStrict(clusterId)
      .then((inv) => !cancelled && setNamespaces([...inv.namespaces].sort()))
      .catch(() => !cancelled && setNamespaces([]));
    api
      .getPods({ cluster: clusterId, sortBy: 'risk_desc', page: 1, pageSize: 500 })
      .then((r) => {
        if (cancelled) return;
        const out: Record<string, PodFacts> = {};
        for (const p of r.pods) out[p.uid] = { name: p.name, namespace: p.namespace, riskCount: p.riskCount, finalLevel: p.finalLevel, score: p.unifiedScore ?? p.totalScore };
        setPods(out);
      })
      .catch(() => !cancelled && setPods({}));
    return () => {
      cancelled = true;
    };
  }, [clusterId, refreshKey]);

  const common = useMemo(
    () => ({ cluster: clusterId, namespace: namespace || undefined, podUid: podUid || undefined, q: q || undefined, sinceMinutes }),
    [clusterId, namespace, podUid, q, sinceMinutes],
  );

  const [overview, setOverview] = useState<{ rows: FlowRow[]; total: number } | null>(null);
  const [issue, setIssue] = useState<NetworkDataIssue | null>(null);
  useEffect(() => {
    if (!clusterId) return;
    let cancelled = false;
    api
      .getNetworkActivity({ ...common, view: 'edges', page: 1, pageSize: OVERVIEW_EDGE_LIMIT })
      .then((r) => {
        if (cancelled) return;
        setOverview({ rows: r.items as FlowRow[], total: r.total ?? 0 });
        setIssue(null);
      })
      .catch((e) => !cancelled && setIssue(classifyNetworkDataIssue(e)));
    return () => {
      cancelled = true;
    };
  }, [common, clusterId, refreshKey]);

  const [table, setTable] = useState<{ rows: Array<FlowRow | NetworkActivityDestinationRow>; total: number; group: Group } | null>(null);
  const [tableLoading, setTableLoading] = useState(false);
  useEffect(() => {
    if (!clusterId || view !== 'table') return;
    let cancelled = false;
    setTableLoading(true);
    api
      .getNetworkActivity({ ...common, view: group === 'dest' ? 'destinations' : 'edges', page, pageSize })
      .then((r) => !cancelled && setTable({ rows: r.items as Array<FlowRow | NetworkActivityDestinationRow>, total: r.total ?? 0, group }))
      .catch((e) => !cancelled && setIssue(classifyNetworkDataIssue(e)))
      .finally(() => !cancelled && setTableLoading(false));
    return () => {
      cancelled = true;
    };
  }, [common, clusterId, view, group, page, pageSize, refreshKey]);

  const [mapData, setMapData] = useState<{ destinations: NetworkActivityDestinationRow[]; talkers: NetworkActivityTalkerRow[] } | null>(null);
  useEffect(() => {
    if (!clusterId || view !== 'map') return;
    let cancelled = false;
    Promise.all([
      api.getNetworkActivity({ ...common, view: 'destinations', page: 1, pageSize: MAP_SUMMARY_LIMIT }),
      api.getNetworkActivity({ ...common, view: 'talkers', page: 1, pageSize: MAP_SUMMARY_LIMIT }),
    ])
      .then(([d, t]) => !cancelled && setMapData({ destinations: d.items as NetworkActivityDestinationRow[], talkers: t.items as NetworkActivityTalkerRow[] }))
      .catch((e) => !cancelled && setIssue(classifyNetworkDataIssue(e)));
    return () => {
      cancelled = true;
    };
  }, [common, clusterId, view, refreshKey]);

  const pollMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(() => setRefreshKey((k) => k + 1), pollMs, { refreshTrigger, enabled: Boolean(clusterId) });

  const flagged = useMemo(() => {
    if (!overview) return [];
    return overview.rows
      .map((row) => ({ row, flag: flagFlow(row, row.podUid ? pods[row.podUid] : undefined) }))
      .filter((x): x is { row: FlowRow; flag: NonNullable<ReturnType<typeof flagFlow>> } => x.flag !== null)
      .sort((a, b) => b.flag.weight - a.flag.weight || Number(b.row.observationCount ?? 0) - Number(a.row.observationCount ?? 0));
  }, [overview, pods]);
  const flaggedKeys = useMemo(() => new Set(flagged.map((f) => flowKey(f.row))), [flagged]);

  const podNames = useMemo(() => {
    const out: Record<string, string> = {};
    for (const [uid, p] of Object.entries(pods)) out[uid] = p.name;
    return out;
  }, [pods]);

  const summary = useMemo(() => {
    if (!overview) return '';
    const window = SINCE_OPTIONS.find((o) => o.value === sinceMinutes)?.label.toLowerCase() ?? '';
    const sources = new Set(overview.rows.map((r) => r.podUid)).size;
    const flows = `${overview.total.toLocaleString()} ${overview.total === 1 ? 'flow' : 'flows'}`;
    return overview.total <= overview.rows.length
      ? `${flows} from ${sources.toLocaleString()} ${sources === 1 ? 'pod' : 'pods'}, ${window}`
      : `${flows}, ${window}`;
  }, [overview, sinceMinutes]);

  const canFindings = allowedRoutes.includes('/risks');

  const flowColumns = useMemo<TableColumn<FlowRow>[]>(
    () => [
      {
        key: 'source',
        header: 'Source',
        cell: (r) => (
          <div className="min-w-0">
            <Link to={podDetailPath(r.podUid ?? '', clusterId)} className="font-medium text-text hover:text-brand hover:underline">
              {r.podName || (r.podUid ? pods[r.podUid]?.name : '') || r.podUid}
            </Link>
            <div className="text-caption text-muted">{r.namespace}</div>
          </div>
        ),
      },
      {
        key: 'dest',
        header: 'Destination',
        cell: (r) => {
          const d = destLabel({ destIp: r.destIp ?? '', destServiceName: r.destServiceName, destServiceNamespace: r.destServiceNamespace, destWorkloadName: r.destWorkloadName, destWorkloadNamespace: r.destWorkloadNamespace });
          return (
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-text">{d.name}</span>
                <KindTag kind={destKind({ destIp: r.destIp ?? '', destServiceName: r.destServiceName, destWorkloadName: r.destWorkloadName })} />
              </div>
              {d.detail ? <div className="text-caption text-muted">{d.detail}</div> : null}
            </div>
          );
        },
      },
      { key: 'port', header: 'Port', cell: (r) => <span className="font-mono text-caption text-text">{r.destPort}/{String(r.protocol ?? '').toUpperCase()}</span> },
      { key: 'obs', header: 'Seen', cell: (r) => <span className="tabular-nums">{Number(r.observationCount ?? 0).toLocaleString()}×</span> },
      {
        key: 'last',
        header: 'Last seen',
        className: 'hidden md:table-cell',
        headerClassName: 'hidden md:table-cell',
        cell: (r) => <span className="text-caption text-muted">{relativeTime(r.lastObservedAt ?? r.observedAt)}</span>,
      },
      {
        key: 'flags',
        header: 'Flags',
        cell: (r) => {
          const facts = r.podUid ? pods[r.podUid] : undefined;
          const flag = flagFlow(r, facts);
          return (
            <span className="flex flex-wrap items-center gap-1.5">
              {facts && facts.riskCount > 0 && canFindings ? (
                <Link to={findingsForResourcePath({ uid: r.podUid ?? '', clusterId })} className="rounded border border-border px-1 text-meta text-brand hover:underline">
                  {facts.riskCount} {facts.riskCount === 1 ? 'finding' : 'findings'}
                </Link>
              ) : null}
              {flag && flag.reasons.some((x) => x.startsWith('external on port')) ? (
                <span className="rounded border border-amber-500/50 px-1 text-meta text-amber-300">unusual port</span>
              ) : null}
            </span>
          );
        },
      },
    ],
    [clusterId, pods, canFindings],
  );

  const destColumns = useMemo<TableColumn<NetworkActivityDestinationRow>[]>(
    () => [
      {
        key: 'dest',
        header: 'Destination',
        cell: (r) => {
          const d = destLabel(r);
          return (
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="font-medium text-text">{d.name}</span>
                <KindTag kind={destKind(r)} />
              </div>
              {d.detail ? <div className="text-caption text-muted">{d.detail}</div> : null}
            </div>
          );
        },
      },
      { key: 'port', header: 'Port', cell: (r) => <span className="font-mono text-caption text-text">{r.destPort}/{String(r.protocol ?? '').toUpperCase()}</span> },
      { key: 'pods', header: 'Pods', cell: (r) => <span className="tabular-nums">{r.distinctPodCount.toLocaleString()}</span> },
      { key: 'obs', header: 'Seen', cell: (r) => <span className="tabular-nums">{r.observationCount.toLocaleString()}×</span> },
      {
        key: 'last',
        header: 'Last seen',
        className: 'hidden md:table-cell',
        headerClassName: 'hidden md:table-cell',
        cell: (r) => <span className="text-caption text-muted">{relativeTime(r.lastObservedAt)}</span>,
      },
    ],
    [],
  );

  const podChip = podUid ? pods[podUid]?.name || overview?.rows.find((r) => r.podUid === podUid)?.podName || podUid : '';
  const filtered = Boolean(q || namespace || podUid);

  const showDestination = useCallback(
    (destIp: string) => {
      setSelection(null);
      setSearchDraft(destIp);
      patch({ q: destIp, podUid: '', group: 'pod', view: 'table', page: 1 });
    },
    [patch],
  );

  return (
    <PageLayout title={PAGE_TITLES.networkActivity}>
      {!clusterId ? (
        <PageEmpty title="No cluster selected" description="Pick a cluster in the header to see its traffic." className="py-10" />
      ) : issue ? (
        <NetworkDataIssuePanel issue={issue} onAction={() => setRefreshKey((k) => k + 1)} />
      ) : (
        <div className="flex flex-col gap-4">
          {flagged.length > 0 ? (
            <section aria-labelledby="flagged-title" className="rounded-xl border border-amber-500/30 bg-amber-500/5 p-3">
              <h2 id="flagged-title" className="mb-2 text-body font-semibold text-text">
                Worth a look <span className="font-normal text-muted">({flagged.length.toLocaleString()})</span>
              </h2>
              <ul className="divide-y divide-border/60">
                {flagged.slice(0, 5).map(({ row, flag }) => {
                  const d = destLabel({ destIp: row.destIp ?? '', destServiceName: row.destServiceName, destServiceNamespace: row.destServiceNamespace, destWorkloadName: row.destWorkloadName, destWorkloadNamespace: row.destWorkloadNamespace });
                  return (
                    <li key={flowKey(row)}>
                      <button
                        type="button"
                        onClick={() => setSelection({ kind: 'flow', row })}
                        className="flex w-full flex-wrap items-baseline gap-x-3 gap-y-0.5 py-2 text-left hover:text-brand"
                      >
                        <span className="min-w-0 text-body text-text">
                          {row.namespace}/{row.podName || (row.podUid ? pods[row.podUid]?.name : '')} → {d.name}:{row.destPort}
                        </span>
                        <span className="text-caption text-amber-200">{flag.reasons.join(' · ')}</span>
                        <span className="ml-auto text-caption tabular-nums text-muted">{Number(row.observationCount ?? 0).toLocaleString()}×</span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            </section>
          ) : null}

          <div className="flex flex-col gap-2">
            <div className="grid grid-cols-1 gap-2 md:grid-cols-[minmax(0,1fr)_auto_auto]">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden />
                <input
                  id="na-search-input"
                  value={searchDraft}
                  onChange={(e) => setSearchDraft(e.target.value)}
                  placeholder="Pod, IP, service or port"
                  aria-label="Search flows"
                  className="min-h-9 w-full rounded-lg border border-border bg-base pl-9 pr-3 text-body text-text outline-none focus:border-brand"
                />
              </div>
              <select
                id="na-namespace-input"
                value={namespace}
                onChange={(e) => patch({ namespace: e.target.value, page: 1 })}
                aria-label="Namespace"
                className="min-h-9 rounded-lg border border-border bg-base px-3 text-body text-text"
              >
                <option value="">All namespaces</option>
                {namespaces.map((ns) => (
                  <option key={ns} value={ns}>
                    {ns}
                  </option>
                ))}
              </select>
              <select
                id="na-since-select"
                value={sinceMinutes}
                onChange={(e) => patch({ since: e.target.value, page: 1 })}
                aria-label="Time range"
                className="min-h-9 rounded-lg border border-border bg-base px-3 text-body text-text"
              >
                {SINCE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Segmented label="View" value={view} onChange={(v) => patch({ view: v })} options={[{ id: 'table', label: 'Table' }, { id: 'map', label: 'Map' }]} />
              {view === 'table' ? (
                <Segmented label="Group by" value={group} onChange={(g) => patch({ group: g, page: 1 })} options={[{ id: 'pod', label: 'By pod' }, { id: 'dest', label: 'By destination' }]} />
              ) : null}
              {podUid ? (
                <button
                  type="button"
                  onClick={() => patch({ podUid: '', page: 1 })}
                  className="inline-flex min-h-8 items-center gap-1 rounded-lg border border-brand/50 bg-brand/10 px-2.5 text-caption font-semibold text-text"
                >
                  Pod: {podChip}
                  <X className="h-3.5 w-3.5" aria-label="Remove pod filter" />
                </button>
              ) : null}
              {filtered ? (
                <button
                  type="button"
                  onClick={() => {
                    setSearchDraft('');
                    patch({ q: '', namespace: '', podUid: '', page: 1 });
                  }}
                  className="text-caption font-semibold text-muted hover:text-text"
                >
                  Reset filters
                </button>
              ) : null}
              <span className="ml-auto text-caption text-muted">{summary}</span>
            </div>
          </div>

          {view === 'map' ? (
            <div className="relative overflow-hidden rounded-xl border border-border bg-base">
              {overview && mapData ? (
                <NetworkTopologyGraph
                  destinations={mapData.destinations}
                  talkers={mapData.talkers}
                  connections={overview.rows}
                  podNamesByUid={podNames}
                  maxNodes={120}
                  showTrustOverlay={false}
                  onNodeClick={(id, kind) => {
                    if (kind === 'pod') {
                      patch({ podUid: id, view: 'table', group: 'pod', page: 1 });
                      return;
                    }
                    const dest = mapData.destinations.find((d) => `dest:${d.destIp}:${d.destPort}/${d.protocol ?? 'tcp'}` === id);
                    if (dest) setSelection({ kind: 'dest', row: dest });
                  }}
                  className="h-[min(65dvh,640px)] min-h-[24rem] w-full"
                />
              ) : (
                <p className="p-6 text-body text-muted">Loading map…</p>
              )}
            </div>
          ) : group === 'dest' ? (
            <Table
              columns={destColumns}
              data={table?.group === 'dest' ? (table.rows as NetworkActivityDestinationRow[]) : []}
              loading={tableLoading && (!table || table.group !== 'dest')}
              rowKey={(r) => `${r.destIp}|${r.destPort}|${r.protocol}`}
              onRowClick={(r) => setSelection({ kind: 'dest', row: r })}
              isRowSelected={(r) => selection?.kind === 'dest' && selection.row.destIp === r.destIp && selection.row.destPort === r.destPort}
              scrollClassName="overflow-x-auto"
              emptyTitle={filtered ? 'No destinations match these filters' : 'No traffic in this time range'}
              emptyDescription={filtered ? 'Clear a filter or pick a longer time range.' : 'Flows appear when the agent reports sockets from running pods.'}
            />
          ) : (
            <Table
              columns={flowColumns}
              data={table?.group === 'pod' ? (table.rows as FlowRow[]) : []}
              loading={tableLoading && (!table || table.group !== 'pod')}
              rowKey={(r) => flowKey(r)}
              onRowClick={(r) => setSelection({ kind: 'flow', row: r })}
              isRowSelected={(r) => selection?.kind === 'flow' && flowKey(selection.row) === flowKey(r)}
              rowClassName={(r) => (flaggedKeys.has(flowKey(r)) ? 'border-l-2 border-l-amber-500/70' : undefined)}
              scrollClassName="overflow-x-auto"
              emptyTitle={filtered ? 'No flows match these filters' : 'No traffic in this time range'}
              emptyDescription={filtered ? 'Clear a filter or pick a longer time range.' : 'Flows appear when the agent reports sockets from running pods.'}
            />
          )}
          {view === 'table' && table && table.total > 0 ? (
            <Pagination
              page={page}
              pageSize={pageSize}
              total={table.total}
              onPageChange={(p) => patch({ page: p })}
              onPageSizeChange={(s) => patch({ size: s, page: 1 })}
              pageSizeOptions={PAGE_SIZES}
              itemLabel={group === 'dest' ? 'destinations' : 'flows'}
            />
          ) : null}
        </div>
      )}

      <FlowPanel
        selection={selection}
        clusterId={clusterId}
        sinceMinutes={sinceMinutes}
        pods={pods}
        allowedRoutes={allowedRoutes}
        onClose={() => setSelection(null)}
        onShowDestination={showDestination}
      />
    </PageLayout>
  );
}
