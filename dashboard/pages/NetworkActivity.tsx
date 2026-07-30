import React, { useState, useEffect, useCallback, useMemo, useId, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  RefreshCw,
  Share2,
  List,
  Table2,
  ExternalLink,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  Copy,
  Check,
  X,
  Search,
  Activity,
  SlidersHorizontal,
} from 'lucide-react';
import { api, isApiError } from '../lib/api';
import { useClusterStore } from '../store/clusterStore';
import { useClusters } from '../hooks/useClusters';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { PageEmpty, PageError } from '../design-system/components/PageStatus';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { useConfirm } from '../design-system/components/ConfirmDialog';
import { NetworkTopologyGraph, type NetworkTopologySelection } from '../components/NetworkTopologyGraph';
import { GraphTrustLegend } from '../components/GraphTrustLegend';
import { useGraphTrustContext } from '../hooks/useGraphTrustContext';
import { buildGraphTrustPosture, type GraphTrustPosture, type GraphTrustContext } from '../lib/graphTrustSemantics';
import { PAGE_TITLES } from '../lib/pageTitles';
import type {
  NetworkActivityConnectionRow,
  NetworkActivityDestinationRow,
  NetworkActivityTalkerRow,
  NetworkActivityWorkloadRow,
} from '../types';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH_COMPACT, UI_TR, UI_TD_COMPACT_TIGHT } from '../lib/tableChrome';
import type { SemanticVisibilityState } from '../lib/visibilityEngine';

type NetworkMainTab = 'topology' | 'pods' | 'connections';

/** Poll topology: only loads edges (edges/connections). Full: destinations + talkers + inventory + edges. */
type TopologyFetchOpts = { mode: 'quick' | 'full'; manual?: boolean };

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

function shouldStopNetworkFallback(error: unknown): boolean {
  return isApiError(error) && (error.status === 401 || error.status === 403);
}

const TABLE_PAGE_SIZES = [25, 50, 100, 200] as const;

function podTableLabel(name?: string, uid?: string): string {
  const n = (name ?? '').trim();
  if (n) return n;
  const u = (uid ?? '').trim();
  if (!u) return '—';
  return u.length <= 12 ? u : `${u.slice(0, 12)}…`;
}

function formatObservedAt(iso?: string): string {
  if (!iso) return '—';
  try {
    return new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'medium' });
  } catch {
    return iso;
  }
}

/** Tooltip: clarifies local vs ISO to avoid confusion with UTC bucket. */
function observedAtTooltip(iso?: string): string {
  if (!iso) return '';
  try {
    const d = new Date(iso);
    const local = d.toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'medium' });
    return `Observation time — displayed: local time (${local}). ISO (UTC): ${d.toISOString()}`;
  } catch {
    return iso;
  }
}

/** Display 5-min bucket start in UTC (short + full ISO tooltip). */
function formatBucket5mLine(iso?: string): { short: string; title: string } {
  if (!iso) return { short: '—', title: '' };
  try {
    const d = new Date(iso);
    const full = d.toISOString();
    return {
      short: `${full.slice(0, 10)} ${full.slice(11, 16)} UTC`,
      title: `5-min bucket starts: ${full}`,
    };
  } catch {
    return { short: iso, title: iso };
  }
}

function isListenRow(row: NetworkActivityConnectionRow): boolean {
  const st = (row.state ?? '').toUpperCase();
  return st === 'LISTEN' || st === 'LISTENING';
}

/** Socket LISTEN — shown in Local column; Remote = — (correct semantics). */
function formatListenLocal(row: NetworkActivityConnectionRow): string {
  const p = row.destPort ?? row.sourcePort;
  const dip = (row.destIp ?? '').trim();
  const bind =
    dip && dip !== '0.0.0.0' && dip !== '::' && dip !== '*' && dip !== '[::]'
      ? ` @${dip}`
      : '';
  return `LISTEN :${p ?? '?'}${bind}`;
}

function formatLocalEndpoint(row: NetworkActivityConnectionRow): string {
  if (isListenRow(row)) return formatListenLocal(row);
  return `${(row.sourceIp ?? '—').trim()}:${row.sourcePort ?? '—'}`;
}

function formatRemoteEndpoint(row: NetworkActivityConnectionRow): string {
  if (isListenRow(row)) return '—';
  const dip = (row.destIp ?? '').trim();
  const dp = row.destPort;
  if (!dip && (dp == null || Number.isNaN(Number(dp)))) return '—';
  return `${dip || '—'}:${dp ?? '—'}`;
}

/** Copy string — matches Remote column. */
function remoteEndpointClipboard(row: NetworkActivityConnectionRow): string {
  return formatRemoteEndpoint(row);
}

/** Copy string — matches Local column (including LISTEN). */
function localEndpointClipboard(row: NetworkActivityConnectionRow): string {
  return formatLocalEndpoint(row);
}

async function copyTextWithFallback(text: string): Promise<boolean> {
  const t = text.trim();
  if (!t || t === '—') return false;
  try {
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(t);
      return true;
    }
  } catch {
    /* fallback */
  }
  try {
    const ta = document.createElement('textarea');
    ta.value = t;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}

/** Core view=edges: topology sample cap. Keep lower than API max so graph load stays responsive. */
const TOPOLOGY_EDGE_PAGE_SIZE = 500;
/** Pagination for talkers/destinations; matches NETWORK_ACTIVITY_MAX_PAGE_SIZE (default 200, max 500). */
const NETWORK_SUMMARY_PAGE_SIZE = 200;
/** Cap on talker pages per topology load. Topology needs labels/context, not complete evidence. */
const MAX_TALKER_FETCH_PAGES_CAP = 3;
/** Topology tab: slower polling than tables to reduce periodic load (still has Refresh + refreshTrigger). */
const TOPOLOGY_POLL_INTERVAL_MS = 3 * 60 * 1000;

const SINCE_OPTIONS: { label: string; value: number | '' }[] = [
  { label: '15 min', value: 15 },
  { label: '1 hour', value: 60 },
  { label: '6 hours', value: 360 },
  { label: '24 hours', value: 1440 },
  { label: 'All time (debug — may be slow)', value: '' },
];

function sinceRangeHuman(sinceMinutes: number | ''): string {
  if (sinceMinutes === '') return 'All time';
  switch (sinceMinutes) {
    case 15:
      return '15 min';
    case 60:
      return '1 hour';
    case 360:
      return '6 hours';
    case 1440:
      return '24 hours';
    default:
      return `${sinceMinutes} min`;
  }
}

function sinceRangeMeta(sinceMinutes: number | ''): string {
  if (sinceMinutes === '') return 'all time';
  if (sinceMinutes < 60) return `${sinceMinutes}m`;
  if (sinceMinutes % 1440 === 0) return `${sinceMinutes / 1440}d`;
  if (sinceMinutes % 60 === 0) return `${sinceMinutes / 60}h`;
  return `${sinceMinutes}m`;
}

function parseAppliedPort(q: string): number | null {
  const t = q.trim();
  if (!/^\d+$/.test(t)) return null;
  const n = parseInt(t, 10);
  if (n < 1 || n > 65535) return null;
  return n;
}

/** Pod name from inventory — matches uid with topology when network-activity API lacks podName. */
async function fetchInventoryPodNamesByUid(clusterId: string, namespace?: string): Promise<Record<string, string>> {
  const map: Record<string, string> = {};
  const pageSize = 500;
  for (let page = 1; page <= 30; page++) {
    const { pods, total } = await api.getPodsStrict({
      cluster: clusterId,
      namespace: namespace || undefined,
      page,
      pageSize,
    });
    for (const p of pods) {
      const u = (p.uid ?? '').trim();
      const n = (p.name ?? '').trim();
      if (u && n) map[u] = n;
    }
    if (pods.length < pageSize || page * pageSize >= total) break;
  }
  return map;
}

const inventoryPodNamesCache = new Map<string, { at: number; data: Record<string, string> }>();
const INVENTORY_POD_NAMES_CACHE_TTL_MS = 30_000;
const INVENTORY_CACHE_MAX_KEYS = 24;

function clearInventoryPodNamesCache(): void {
  inventoryPodNamesCache.clear();
}

function trimInventoryPodNamesCache(): void {
  while (inventoryPodNamesCache.size > INVENTORY_CACHE_MAX_KEYS) {
    let oldestK: string | null = null;
    let oldestAt = Infinity;
    for (const [k, v] of inventoryPodNamesCache) {
      if (v.at < oldestAt) {
        oldestAt = v.at;
        oldestK = k;
      }
    }
    if (oldestK == null) break;
    inventoryPodNamesCache.delete(oldestK);
  }
}

async function fetchInventoryPodNamesByUidCached(
  clusterId: string,
  namespace?: string,
): Promise<Record<string, string>> {
  const key = `${clusterId}\x1f${namespace ?? ''}`;
  const now = Date.now();
  const hit = inventoryPodNamesCache.get(key);
  if (hit && now - hit.at < INVENTORY_POD_NAMES_CACHE_TTL_MS) return hit.data;
  const data = await fetchInventoryPodNamesByUid(clusterId, namespace);
  inventoryPodNamesCache.set(key, { at: now, data });
  trimInventoryPodNamesCache();
  return data;
}

type TopologyStatus = {
  title: string;
  body: string;
  variant: 'loading' | 'warning' | 'info' | 'success' | 'neutral';
  actionLabel: string | null;
};

const NETWORK_FILTER_CONTROL_CLASS =
  'h-9 w-full rounded-md border border-border bg-base px-2.5 text-caption text-text placeholder:text-muted-2 focus:border-brand focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60 disabled:cursor-not-allowed disabled:opacity-55';

function NetworkFiltersPanel({
  title,
  children,
  compact = false,
}: {
  title: string;
  children: React.ReactNode;
  compact?: boolean;
}) {
  return (
    <div className={`grid gap-2 rounded-lg border border-border/70 bg-surface/35 ${compact ? 'p-2' : 'p-2.5'}`} title={title}>
      {children}
    </div>
  );
}

function FilterRow({
  search,
  namespace,
  time,
}: {
  search: React.ReactNode;
  namespace: React.ReactNode;
  time: React.ReactNode;
}) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,1fr)_180px] lg:grid-cols-[minmax(320px,1fr)_minmax(220px,320px)_180px] lg:items-end">
      <div className="sm:col-span-2 lg:col-span-1">{search}</div>
      <div>{namespace}</div>
      <div>{time}</div>
    </div>
  );
}

function FilterSection({
  title,
  htmlFor,
  hint,
  labelVisibility = 'visible',
  children,
}: {
  title: string;
  htmlFor: string;
  hint?: React.ReactNode;
  labelVisibility?: 'visible' | 'srOnly';
  children: React.ReactNode;
}) {
  return (
    <section className="grid gap-1.5">
      <label className={labelVisibility === 'srOnly' ? 'sr-only' : 'ui-micro-label'} htmlFor={htmlFor}>
        {title}
      </label>
      {children}
      {hint ? <p className="text-caption leading-relaxed text-muted-2">{hint}</p> : null}
    </section>
  );
}

function SearchFilter({
  value,
  onChange,
  onKeyDown,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  onKeyDown: React.KeyboardEventHandler<HTMLInputElement>;
  placeholder: string;
}) {
  return (
    <FilterSection title="Search" htmlFor="na-search-input" labelVisibility="srOnly">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden />
        <input
          id="na-search-input"
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder={placeholder}
          autoComplete="off"
          className={`${NETWORK_FILTER_CONTROL_CLASS} pl-9`}
        />
      </div>
    </FilterSection>
  );
}

function NamespaceFilter({
  value,
  onChange,
  onCommit,
  onKeyDown,
  options,
  disabled,
  title,
  datalistId,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  onCommit: (value: string) => void;
  onKeyDown: React.KeyboardEventHandler<HTMLInputElement>;
  options: string[];
  disabled: boolean;
  title: string;
  datalistId: string;
  placeholder: string;
}) {
  const exactValue = options.includes(value) ? value : '';
  return (
    <FilterSection title="Namespace" htmlFor="na-namespace-input">
      <div className="grid grid-cols-[minmax(0,1fr)_8.5rem] gap-2 max-[520px]:grid-cols-1">
        <input
          id="na-namespace-input"
          type="text"
          title={title}
          disabled={disabled}
          list={options.length > 0 ? datalistId : undefined}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder={placeholder}
          autoComplete="off"
          className={NETWORK_FILTER_CONTROL_CLASS}
        />
        <select
          aria-label="Choose namespace"
          title={options.length > 0 ? 'Choose a namespace from inventory' : 'Namespace inventory is still loading'}
          value={exactValue}
          disabled={disabled || options.length === 0}
          onChange={(e) => {
            onChange(e.target.value);
            onCommit(e.target.value);
          }}
          className={`${NETWORK_FILTER_CONTROL_CLASS} px-2 text-caption`}
        >
          <option value="">All</option>
          {options.map((ns) => (
            <option key={ns} value={ns}>
              {ns}
            </option>
          ))}
        </select>
      </div>
      {options.length > 0 ? (
        <datalist id={datalistId}>
          {options.map((ns) => (
            <option key={ns} value={ns} />
          ))}
        </datalist>
      ) : null}
    </FilterSection>
  );
}

function TimeFilter({
  value,
  onChange,
  warnAllTime,
}: {
  value: number | '';
  onChange: (value: number | '') => void | Promise<void>;
  warnAllTime: boolean;
}) {
  return (
    <FilterSection title="Time" htmlFor="na-since-select">
      <select
        id="na-since-select"
        value={value === '' ? '' : String(value)}
        onChange={(e) => {
          const raw = e.target.value;
          void onChange(raw === '' ? '' : Number(raw));
        }}
        className={NETWORK_FILTER_CONTROL_CLASS}
      >
        {SINCE_OPTIONS.map((o) => (
          <option key={o.label} value={o.value === '' ? '' : String(o.value)}>
            {o.label}
          </option>
        ))}
      </select>
      {warnAllTime ? <p className="text-micro leading-tight text-amber-600/90">No time cap. Queries may be slow.</p> : null}
    </FilterSection>
  );
}

function PodUidFilter({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <FilterSection
      title="Pod UID"
      htmlFor="na-poduid-input"
      hint={
        <>
          Numeric search terms filter by port.
        </>
      }
    >
      <input
        id="na-poduid-input"
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="Exact pod UID"
        autoComplete="off"
        className={NETWORK_FILTER_CONTROL_CLASS}
      />
    </FilterSection>
  );
}

function AppliedFilterMeta({
  text,
  syncing,
  onApply,
  onReset,
  showReset,
  showApply,
}: {
  text: string;
  syncing: boolean;
  onApply: () => void;
  onReset: () => void;
  showReset: boolean;
  showApply: boolean;
}) {
  return (
    <div className="grid gap-2 border-t border-border/60 pt-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
      <p className="text-meta leading-relaxed text-muted-2">{text}</p>
      <div className="flex flex-wrap gap-2 sm:justify-end">
        <Button
          variant="secondary"
          size="sm"
          type="button"
          className="h-8 text-caption"
          title="Apply namespace and search filters now"
          onClick={onApply}
          disabled={!showApply}
        >
          Apply filters
        </Button>
        {showReset ? (
          <Button
            variant="secondary"
            size="sm"
            type="button"
            className="h-8 text-caption"
            title="Clear namespace / search / podUid"
            onClick={onReset}
          >
            Reset
          </Button>
        ) : null}
      </div>
      {syncing ? (
        <p
          className="text-meta text-amber-500/90 sm:col-span-2"
          title="Filters apply automatically after debounce; Enter applies immediately."
        >
          Syncing…
        </p>
      ) : null}
    </div>
  );
}

function AdvancedFilterDisclosure({ children }: { children: React.ReactNode }) {
  return (
    <details className="group px-1">
      <summary className="flex cursor-pointer list-none items-center gap-1 text-meta font-medium text-muted-2 select-none hover:text-muted [&::-webkit-details-marker]:hidden">
        <ChevronDown className="h-3.5 w-3.5 shrink-0 opacity-70 transition-transform group-open:rotate-180 motion-reduce:transition-none" />
        Advanced
      </summary>
      <div className="mt-1.5 space-y-1.5 border-l border-border/50 pl-3 text-meta leading-relaxed text-muted-2">
        {children}
      </div>
    </details>
  );
}

function CountBadge({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex h-8 items-center rounded-lg border border-border bg-surface/70 px-2.5 text-caption text-muted tabular-nums">
      {children}
    </span>
  );
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

function TopologyLoadingSurface() {
  return (
    <div className="flex min-h-[30rem] flex-col justify-between bg-base/45 px-4 py-4 lg:min-h-[calc(100dvh-19rem)] xl:min-h-[40rem] 2xl:min-h-[48rem]">
      <div className="flex items-center justify-between gap-3">
        <div className="inline-flex items-center gap-2 rounded-lg border border-sky-500/20 bg-sky-950/20 px-3 py-2 text-caption text-sky-100">
          <Activity className="h-4 w-4 text-sky-300" aria-hidden />
          <span className="font-medium">Loading topology</span>
        </div>
        <div className="hidden text-meta text-muted-2 sm:block">Edges first, labels after</div>
      </div>
      <div className="relative mx-auto h-[20rem] w-full max-w-4xl" aria-hidden>
        <div className="topology-loading-grid absolute inset-0 rounded-xl border border-border/50" />
        <div className="absolute left-[13%] top-[28%] h-9 w-9 rounded-full border border-cyan-300/40 bg-cyan-300/20" />
        <div className="absolute left-[38%] top-[48%] h-11 w-11 rounded-full border border-violet-300/35 bg-violet-300/18" />
        <div className="absolute right-[18%] top-[30%] h-10 w-10 rounded-full border border-amber-300/35 bg-amber-300/18" />
        <div className="absolute left-[18%] top-[39%] h-px w-[25%] -rotate-6 bg-emerald-300/35" />
        <div className="absolute left-[48%] top-[50%] h-px w-[28%] rotate-12 bg-emerald-300/35" />
      </div>
      <div className="grid gap-2 text-caption text-muted-2 sm:grid-cols-3">
        <div className="rounded-md border border-border/50 bg-base/30 px-3 py-2">Sampled edge rows</div>
        <div className="rounded-md border border-border/50 bg-base/30 px-3 py-2">Pod names hydrate</div>
        <div className="rounded-md border border-border/50 bg-base/30 px-3 py-2">Graph fit runs after layout</div>
      </div>
    </div>
  );
}

function TopologyTrustStatusStrip({
  status,
  onRefresh,
  refreshing,
}: {
  status: TopologyStatus | null;
  onRefresh: () => void;
  refreshing: boolean;
}) {
  if (!status) return null;
  const stripClass =
    status.variant === 'success'
      ? 'border-sky-500/25 bg-sky-500/10 text-sky-100'
      : status.variant === 'warning'
        ? 'border-amber-600/35 bg-amber-950/25 text-amber-100'
        : status.variant === 'info'
          ? 'border-sky-600/35 bg-sky-950/25 text-sky-100'
          : status.variant === 'loading'
            ? 'border-sky-500/20 bg-sky-950/20 text-sky-100'
          : 'border-border/70 bg-surface/60 text-text';

  return (
    <div className={`rounded-lg border px-2.5 py-2 text-caption leading-snug ${stripClass}`} role="status">
      <div className="grid min-w-0 gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
        <div className="min-w-0 flex-1 space-y-0.5">
          <p className="font-semibold text-text">{status.title}</p>
          <p className="text-muted-2 text-caption leading-snug">{status.body}</p>
        </div>
        {status.actionLabel ? (
          <Button variant="secondary" size="sm" type="button" className="h-8 text-caption" onClick={onRefresh} disabled={refreshing}>
            <RefreshCw className={`w-3.5 h-3.5 mr-1 ${refreshing ? 'animate-spin motion-reduce:animate-none' : ''}`} />
            {status.actionLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function TopologyMetaPanel({
  posture,
  selection,
  onOpenPod,
}: {
  posture: GraphTrustPosture | null;
  selection: NetworkTopologySelection | null;
  onOpenPod?: (podUid: string) => void;
}) {
  return (
    <div className="grid gap-3 md:grid-cols-[minmax(0,1.1fr)_minmax(18rem,0.9fr)]">
      <div className="rounded-lg border border-border/70 bg-surface/35 p-3">
        <div className="mb-2 text-caption font-semibold text-text">Legend</div>
        {posture ? <GraphTrustLegend posture={posture} compact /> : <p className="text-caption text-muted-2">Legend appears once the graph posture is ready.</p>}
      </div>
      <div className="rounded-lg border border-border/70 bg-surface/35 p-3">
        <div className="mb-2 text-caption font-semibold text-text">Selected node</div>
        {selection ? (
          <div className="space-y-1 text-caption leading-snug">
            <p className="text-muted">{selection.typeLabel}</p>
            <p className="break-words font-semibold text-text">{selection.line1}</p>
            <p className="break-words text-muted-2">{selection.line2}</p>
            <p className="break-all text-muted-2">{selection.fullLabel}</p>
            {selection.ownerLabel ? <p className="text-muted-2">Owner: {selection.ownerLabel}</p> : null}
            {selection.kind === 'pod' && onOpenPod ? (
              <Button
                variant="secondary"
                size="sm"
                type="button"
                className="mt-2 h-8 text-caption"
                onClick={() => onOpenPod(selection.id.replace(/^pod:/, ''))}
              >
                <ExternalLink className="mr-1 h-3.5 w-3.5" />
                Open pod detail
              </Button>
            ) : null}
          </div>
        ) : (
          <p className="text-caption text-muted-2">Select a node to inspect it here.</p>
        )}
      </div>
    </div>
  );
}

function TopologyMetaDrawer({
  open,
  posture,
  selection,
  onClose,
  onOpenPod,
}: {
  open: boolean;
  posture: GraphTrustPosture | null;
  selection: NetworkTopologySelection | null;
  onClose: () => void;
  onOpenPod?: (podUid: string) => void;
}) {
  const drawerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const root = drawerRef.current;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
        return;
      }
      if (e.key !== 'Tab' || !root) return;
      const focusable = Array.from(root.querySelectorAll<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])')).filter((el) => !el.hasAttribute('disabled') && el.offsetParent !== null);
      if (focusable.length === 0) return;
      const active = document.activeElement as HTMLElement | null;
      if (!active || !root.contains(active)) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey) {
        if (active === first) {
          e.preventDefault();
          last.focus();
        }
      } else if (active === last) {
        e.preventDefault();
        first.focus();
      }
    };
    window.addEventListener('keydown', onKeyDown, true);
    const t = window.setTimeout(() => {
      root?.querySelector<HTMLElement>('button, [href], input')?.focus();
    }, 0);
    return () => {
      window.clearTimeout(t);
      window.removeEventListener('keydown', onKeyDown, true);
    };
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 lg:hidden">
      <button type="button" aria-label="Close topology details" className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div ref={drawerRef} role="dialog" aria-modal="true" aria-labelledby="topology-meta-drawer-title" className="absolute bottom-0 left-0 right-0 max-h-[88svh] overflow-y-auto rounded-t-2xl border-t border-border bg-surface shadow-2xl">
        <div className="flex items-center justify-between border-b border-border px-3 py-2">
          <h2 id="topology-meta-drawer-title" className="text-caption font-semibold text-text">Legend and selection</h2>
          <Button variant="ghost" size="sm" type="button" className="min-h-10 min-w-10 p-0" onClick={onClose} aria-label="Close legend drawer">
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="p-3">
          <TopologyMetaPanel posture={posture} selection={selection} onOpenPod={onOpenPod} />
        </div>
      </div>
    </div>
  );
}

export function NetworkActivity() {
  const confirm = useConfirm();
  const networkGraphTrust = useGraphTrustContext('network_activity');
  const navigate = useNavigate();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const setSelectedClusterId = useClusterStore((s) => s.setSelectedClusterId);
  const { clusters } = useClusters();
  const [namespaceDraft, setNamespaceDraft] = useState('');
  const [searchDraft, setSearchDraft] = useState('');
  const [namespaceApplied, setNamespaceApplied] = useState('');
  const [searchApplied, setSearchApplied] = useState('');
  /** Exact filter by source pod (query podUid on Core); drill when podName is unavailable. */
  const [podUidApplied, setPodUidApplied] = useState('');
  const [sinceMinutes, setSinceMinutes] = useState(15 as number | '');
  const [loading, setLoading] = useState(false);
  const [topologyHydrating, setTopologyHydrating] = useState(false);
  const [refreshSpin, setRefreshSpin] = useState(false);
  const [graphDestinations, setGraphDestinations] = useState([] as NetworkActivityDestinationRow[]);
  const [graphTalkers, setGraphTalkers] = useState([] as NetworkActivityTalkerRow[]);
  const [graphConnections, setGraphConnections] = useState([] as NetworkActivityConnectionRow[]);
  const [topologyPodNamesByUid, setTopologyPodNamesByUid] = useState<Record<string, string>>({});
  /** Notice when view API is missing / fallback — avoids confusion with "no data". */
  const [topologySupportIssue, setTopologySupportIssue] = useState<string | null>(null);
  const [topologyDataIssue, setTopologyDataIssue] = useState<NetworkDataIssue | null>(null);
  const [destTotal, setDestTotal] = useState(0);
  const [talkerTotal, setTalkerTotal] = useState(0);
  const [clusterNamespaces, setClusterNamespaces] = useState([] as string[]);
  const [clusterNsLoading, setClusterNsLoading] = useState(false);
  const [topologySelection, setTopologySelection] = useState<NetworkTopologySelection | null>(null);
  const [legendDrawerOpen, setLegendDrawerOpen] = useState(false);
  const [topologyFiltersCollapsed, setTopologyFiltersCollapsed] = useState(true);

  const [mainTab, setMainTab] = useState<NetworkMainTab>('topology');
  const filterDebounceMs = useMemo(
    () => (mainTab === 'topology' ? 880 : 700),
    [mainTab],
  );
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(50);
  const [podsRows, setPodsRows] = useState([] as NetworkActivityWorkloadRow[]);
  const [podsTotal, setPodsTotal] = useState(0);
  const [connRows, setConnRows] = useState([] as NetworkActivityConnectionRow[]);
  const [connTotal, setConnTotal] = useState(0);
  const [tableLoading, setTableLoading] = useState(false);
  const [tableDataIssue, setTableDataIssue] = useState<NetworkDataIssue | null>(null);

  const namespaceDatalistId = useId();
  /** Prevents stale request (still pending) from overwriting state after clearing filters / changing filter. */
  const fetchReqIdRef = useRef(0);
  const fetchTableReqIdRef = useRef(0);
  const [copiedTableKey, setCopiedTableKey] = useState<string | null>(null);
  const [copyErrorToast, setCopyErrorToast] = useState<string | null>(null);
  const [sinceCancelToast, setSinceCancelToast] = useState<string | null>(null);
  /** Hint when drilling from a pod without a name (avoids "q" = uid prefix not matching backend). */
  const [connectionsScopeNote, setConnectionsScopeNote] = useState<string | null>(null);
  const copyFeedbackTimerRef = useRef<number | null>(null);
  const copyErrorTimerRef = useRef<number | null>(null);
  const sinceCancelTimerRef = useRef<number | null>(null);

  const copyToClipboard = useCallback((key: string, text: string) => {
    void copyTextWithFallback(text).then((ok) => {
      if (ok) {
        if (copyFeedbackTimerRef.current) window.clearTimeout(copyFeedbackTimerRef.current);
        setCopiedTableKey(key);
        copyFeedbackTimerRef.current = window.setTimeout(() => setCopiedTableKey(null), 2000);
      } else {
        if (copyErrorTimerRef.current) window.clearTimeout(copyErrorTimerRef.current);
        setCopyErrorToast(
          'Could not copy (browser permissions, clipboard policy, or insecure context).',
        );
        copyErrorTimerRef.current = window.setTimeout(() => setCopyErrorToast(null), 4500);
      }
    });
  }, []);

  useEffect(
    () => () => {
      if (copyFeedbackTimerRef.current) window.clearTimeout(copyFeedbackTimerRef.current);
      if (copyErrorTimerRef.current) window.clearTimeout(copyErrorTimerRef.current);
      if (sinceCancelTimerRef.current) window.clearTimeout(sinceCancelTimerRef.current);
    },
    [],
  );

  useEffect(() => {
    if (selectedClusterId != null) return;
    if (clusters.length !== 1) return;
    setSelectedClusterId(String(clusters[0].id));
  }, [clusters, selectedClusterId, setSelectedClusterId]);

  useEffect(() => {
    clearInventoryPodNamesCache();
    setPodUidApplied('');
    setTopologyDataIssue(null);
    setTableDataIssue(null);
    setTopologySelection(null);
    setLegendDrawerOpen(false);
  }, [selectedClusterId]);

  useEffect(() => {
    if (mainTab !== 'connections') setConnectionsScopeNote(null);
  }, [mainTab]);

  useEffect(() => {
    if (connectionsScopeNote && searchApplied.trim().length > 0) setConnectionsScopeNote(null);
  }, [searchApplied, connectionsScopeNote]);


  useEffect(() => {
    if (mainTab !== 'topology') {
      setTopologySelection(null);
      setLegendDrawerOpen(false);
    }
  }, [mainTab]);

  const appliedPort = useMemo(() => parseAppliedPort(searchApplied), [searchApplied]);
  const hasTextFilters = Boolean(namespaceApplied || searchApplied || podUidApplied.trim());
  const hasDraftTextFilters = Boolean(namespaceDraft.trim() || searchDraft.trim());
  const textDraftDiffersFromApplied =
    namespaceDraft.trim() !== namespaceApplied || searchDraft.trim() !== searchApplied;
  const sinceHuman = useMemo(() => sinceRangeHuman(sinceMinutes), [sinceMinutes]);

  /** Full applied query state (tooltip only — not primary UI). */
  const appliedContextTooltip = useMemo(() => {
    const lines = [
      `time: ${sinceHuman}`,
      namespaceApplied.trim() ? `namespace: ${namespaceApplied.trim()}` : 'namespace: (all)',
      searchApplied.trim() ? `q: ${searchApplied.trim()}` : 'q: (empty)',
    ];
    if (podUidApplied.trim()) lines.push(`podUid: ${podUidApplied.trim()}`);
    if (podUidApplied.trim() && searchApplied.trim()) {
      lines.push('podUid + q: AND (intersection)');
    }
    return lines.join('\n');
  }, [sinceHuman, namespaceApplied, searchApplied, podUidApplied]);

  const filterMetaText = useMemo(() => {
    const parts = [
      sinceRangeMeta(sinceMinutes),
      namespaceApplied.trim() ? namespaceApplied.trim() : 'all namespaces',
      searchApplied.trim() ? `q=${searchApplied.trim()}` : 'q=(empty)',
    ];
    if (podUidApplied.trim()) parts.push(`podUid=${podUidApplied.trim()}`);
    return `Filters: ${parts.join(' · ')}`;
  }, [sinceMinutes, namespaceApplied, searchApplied, podUidApplied]);
  const showFilterMeta = textDraftDiffersFromApplied || hasTextFilters || hasDraftTextFilters;
  const showPodUidInPrimaryFilters = Boolean(podUidApplied.trim());

  /** Flush draft → applied immediately (debounce skip), e.g. Enter key. */
  const flushPendingFilters = useCallback(() => {
    setNamespaceApplied(namespaceDraft.trim());
    setSearchApplied(searchDraft.trim());
  }, [namespaceDraft, searchDraft]);

  const clearTextFilters = useCallback(() => {
    setNamespaceDraft('');
    setSearchDraft('');
    setNamespaceApplied('');
    setSearchApplied('');
    setPodUidApplied('');
    setConnectionsScopeNote(null);
  }, []);

  /**
   * Auto-apply: namespace + search sync after debounce (topology tab uses longer debounce).
   */
  useEffect(() => {
    const ns = namespaceDraft.trim();
    const sq = searchDraft.trim();
    const t = window.setTimeout(() => {
      setNamespaceApplied((p) => (p === ns ? p : ns));
      setSearchApplied((p) => (p === sq ? p : sq));
    }, filterDebounceMs);
    return () => window.clearTimeout(t);
  }, [namespaceDraft, searchDraft, filterDebounceMs]);

  useEffect(() => {
    if (!selectedClusterId) {
      setClusterNamespaces([]);
      return;
    }
    let cancelled = false;
    setClusterNsLoading(true);
    void api
      .getClusterInventoryStrict(selectedClusterId)
      .then((inv) => {
        if (cancelled) return;
        const raw = inv?.namespaces ?? [];
        setClusterNamespaces([...raw].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' })));
      })
      .catch((err) => {
        if (cancelled) return;
        setClusterNamespaces([]);
        if (shouldStopNetworkFallback(err)) {
          setTopologyDataIssue(classifyNetworkDataIssue(err, 'Failed to load cluster inventory'));
        }
      })
      .finally(() => {
        if (!cancelled) setClusterNsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedClusterId]);

  const fetchTopology = useCallback(
    async (opts: TopologyFetchOpts) => {
      const { mode, manual = false } = opts;
      const isFull = mode === 'full';

      if (!selectedClusterId) {
        fetchReqIdRef.current += 1;
        setGraphDestinations([]);
        setGraphTalkers([]);
        setGraphConnections([]);
        setTopologyPodNamesByUid({});
        setDestTotal(0);
        setTalkerTotal(0);
        setTopologySupportIssue(null);
        setLoading(false);
        setTopologyHydrating(false);
        setRefreshSpin(false);
        return;
      }
      const myId = ++fetchReqIdRef.current;
      if (manual) setRefreshSpin(true);
      if (isFull) {
        setLoading(true);
        setTopologyHydrating(false);
        setTopologyDataIssue(null);
        if (!manual) {
          setGraphDestinations([]);
          setGraphTalkers([]);
          setGraphConnections([]);
          setTopologyPodNamesByUid({});
          setDestTotal(0);
          setTalkerTotal(0);
          setTopologySupportIssue(null);
        }
      }
      if (manual && isFull) {
        const invKey = `${selectedClusterId}\x1f${namespaceApplied ?? ''}`;
        inventoryPodNamesCache.delete(invKey);
      }
      let supportIssue: string | null = null;
      try {
        const commonList = {
          cluster: selectedClusterId,
          namespace: namespaceApplied || undefined,
          q: searchApplied || undefined,
          podUid: podUidApplied.trim() || undefined,
          sinceMinutes: sinceMinutes === '' ? undefined : sinceMinutes,
        };

        if (!isFull) {
          let edgeOrLegacy: Awaited<ReturnType<typeof api.getNetworkActivity>>;
          try {
            edgeOrLegacy = await api.getNetworkActivity({
              ...commonList,
              view: 'edges',
              page: 1,
              pageSize: TOPOLOGY_EDGE_PAGE_SIZE,
            });
          } catch (err) {
            if (shouldStopNetworkFallback(err)) throw err;
            try {
              edgeOrLegacy = await api.getNetworkActivity({
                ...commonList,
                view: 'connections',
                page: 1,
                pageSize: NETWORK_SUMMARY_PAGE_SIZE,
              });
            } catch (fallbackErr) {
              if (shouldStopNetworkFallback(fallbackErr)) throw fallbackErr;
              if (myId !== fetchReqIdRef.current) return;
              return;
            }
          }
          if (myId !== fetchReqIdRef.current) return;
          setTopologyDataIssue(null);
          setGraphConnections((edgeOrLegacy.items as NetworkActivityConnectionRow[]) ?? []);
          return;
        }

        let edgeOrLegacy: Awaited<ReturnType<typeof api.getNetworkActivity>>;
        try {
          edgeOrLegacy = await api.getNetworkActivity({
            ...commonList,
            view: 'edges',
            page: 1,
            pageSize: TOPOLOGY_EDGE_PAGE_SIZE,
          });
        } catch (err) {
          if (shouldStopNetworkFallback(err)) throw err;
          try {
            edgeOrLegacy = await api.getNetworkActivity({
              ...commonList,
              view: 'connections',
              page: 1,
              pageSize: NETWORK_SUMMARY_PAGE_SIZE,
            });
            if (!supportIssue) {
              supportIssue =
                'Using view "connections" instead of "edges"; topology graph may be limited.';
            }
          } catch (fallbackErr) {
            if (shouldStopNetworkFallback(fallbackErr)) throw fallbackErr;
            supportIssue =
              supportIssue ??
              'Core does not support "edges"/"connections" for topology or load error.';
            edgeOrLegacy = {
              view: 'connections',
              clusterId: selectedClusterId,
              total: 0,
              page: 1,
              pageSize: NETWORK_SUMMARY_PAGE_SIZE,
              items: [],
            };
          }
        }

        if (myId !== fetchReqIdRef.current) return;
        const edgeRows = (edgeOrLegacy.items as NetworkActivityConnectionRow[]) ?? [];
        setGraphConnections(edgeRows);
        if (edgeRows.length > 0) {
          setLoading(false);
          setTopologyHydrating(true);
        }

        const destPromise = api.getNetworkActivity({
          ...commonList,
          view: 'destinations',
          page: 1,
          pageSize: NETWORK_SUMMARY_PAGE_SIZE,
        }).catch((err) => {
          if (shouldStopNetworkFallback(err)) throw err;
          supportIssue =
            supportIssue ??
            'Core did not return view "destinations" (requires Core version supporting topology) or network error.';
          return {
            view: 'destinations',
            clusterId: selectedClusterId,
            total: 0,
            page: 1,
            pageSize: NETWORK_SUMMARY_PAGE_SIZE,
            items: [],
          } as Awaited<ReturnType<typeof api.getNetworkActivity>>;
        });

        const invNamesPromise = fetchInventoryPodNamesByUidCached(
          selectedClusterId,
          namespaceApplied || undefined,
        ).catch((err) => {
          if (shouldStopNetworkFallback(err)) throw err;
          supportIssue =
            supportIssue ??
            'Pod inventory labels could not be loaded. Topology remains usable, but pod names may be incomplete.';
          return {};
        });

        const talkersPromise = (async () => {
          const talkerRows: NetworkActivityTalkerRow[] = [];
          let talkerTotalAcc = 0;
          let maxTalkerPages = MAX_TALKER_FETCH_PAGES_CAP;
          for (let page = 1; page <= maxTalkerPages; page += 1) {
            const talkerData = await api.getNetworkActivity({
              ...commonList,
              view: 'talkers',
              page,
              pageSize: NETWORK_SUMMARY_PAGE_SIZE,
            });
            if (page === 1) {
              talkerTotalAcc = talkerData.total ?? 0;
              const needed = Math.ceil(
                Math.max(0, talkerTotalAcc) / NETWORK_SUMMARY_PAGE_SIZE,
              );
              maxTalkerPages = Math.min(MAX_TALKER_FETCH_PAGES_CAP, Math.max(1, needed));
            }
            const batch = (talkerData.items as NetworkActivityTalkerRow[]) ?? [];
            talkerRows.push(...batch);
            if (batch.length < NETWORK_SUMMARY_PAGE_SIZE) break;
          }
          return { rows: talkerRows, total: talkerTotalAcc };
        })().catch((err) => {
          if (shouldStopNetworkFallback(err)) throw err;
          supportIssue =
            supportIssue ??
            'Core did not return view "talkers" (requires Core version supporting topology) or load error.';
          return { rows: [] as NetworkActivityTalkerRow[], total: 0 };
        });

        const [destData, invNames, talkerData] = await Promise.all([
          destPromise,
          invNamesPromise,
          talkersPromise,
        ]);

        if (myId !== fetchReqIdRef.current) return;
        setGraphDestinations((destData.items as NetworkActivityDestinationRow[]) ?? []);
        setGraphTalkers(talkerData.rows);
        setGraphConnections(edgeRows);
        setTopologyPodNamesByUid(invNames);
        setDestTotal(destData.total ?? 0);
        setTalkerTotal(talkerData.total);
        setTopologySupportIssue(supportIssue);
        setTopologyDataIssue(null);
        setTopologyHydrating(false);
      } catch (err) {
        if (myId !== fetchReqIdRef.current) return;
        setGraphDestinations([]);
        setGraphTalkers([]);
        setGraphConnections([]);
        setTopologyPodNamesByUid({});
        setDestTotal(0);
        setTalkerTotal(0);
        setTopologySupportIssue(null);
        setTopologyDataIssue(classifyNetworkDataIssue(err, 'Failed to load topology data'));
        setTopologyHydrating(false);
      } finally {
        if (myId === fetchReqIdRef.current) {
          if (isFull) setLoading(false);
          if (manual) setRefreshSpin(false);
        }
      }
    },
    [selectedClusterId, namespaceApplied, searchApplied, podUidApplied, sinceMinutes],
  );

  const fetchTableData = useCallback(
    async (manual = false) => {
      if (!selectedClusterId || (mainTab !== 'pods' && mainTab !== 'connections')) {
        fetchTableReqIdRef.current += 1;
        setPodsRows([]);
        setPodsTotal(0);
        setConnRows([]);
        setConnTotal(0);
        setTableLoading(false);
        setTableDataIssue(null);
        if (manual) setRefreshSpin(false);
        return;
      }
      const myId = ++fetchTableReqIdRef.current;
      if (manual) setRefreshSpin(true);
      setTableLoading(true);
      setTableDataIssue(null);
      try {
        const commonList = {
          cluster: selectedClusterId,
          namespace: namespaceApplied || undefined,
          q: searchApplied || undefined,
          podUid: podUidApplied.trim() || undefined,
          sinceMinutes: sinceMinutes === '' ? undefined : sinceMinutes,
          page: tablePage,
          pageSize: tablePageSize,
        };
        const view = mainTab === 'pods' ? 'pods' : 'connections';
        const data = await api.getNetworkActivity({ ...commonList, view });
        if (myId !== fetchTableReqIdRef.current) return;
        if (view === 'pods') {
          setPodsRows((data.items as NetworkActivityWorkloadRow[]) ?? []);
          setPodsTotal(data.total ?? 0);
        } else {
          setConnRows((data.items as NetworkActivityConnectionRow[]) ?? []);
          setConnTotal(data.total ?? 0);
        }
      } catch (err) {
        if (myId !== fetchTableReqIdRef.current) return;
        if (mainTab === 'pods') {
          setPodsRows([]);
          setPodsTotal(0);
        } else {
          setConnRows([]);
          setConnTotal(0);
        }
        setTableDataIssue(classifyNetworkDataIssue(err, 'Failed to load network table'));
      } finally {
        if (myId === fetchTableReqIdRef.current) {
          setTableLoading(false);
          if (manual) setRefreshSpin(false);
        }
      }
    },
    [
      selectedClusterId,
      mainTab,
      namespaceApplied,
      searchApplied,
      podUidApplied,
      sinceMinutes,
      tablePage,
      tablePageSize,
    ],
  );

  useEffect(() => {
    if (mainTab !== 'topology') return;
    void fetchTopology({ mode: 'full' });
  }, [mainTab, fetchTopology]);

  useEffect(() => {
    if (mainTab !== 'pods' && mainTab !== 'connections') return;
    void fetchTableData(false);
  }, [mainTab, fetchTableData]);

  useEffect(() => {
    setTablePage(1);
  }, [namespaceApplied, searchApplied, podUidApplied, sinceMinutes, selectedClusterId, mainTab, tablePageSize]);

  const listPollIntervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(
    () => {
      void fetchTableData(false);
    },
    listPollIntervalMs,
    { refreshTrigger, enabled: mainTab !== 'topology' },
  );

  useEffect(() => {
    if (mainTab !== 'topology') return undefined;
    const id = window.setInterval(() => {
      void fetchTopology({ mode: 'quick' });
    }, TOPOLOGY_POLL_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [mainTab, fetchTopology]);

  /** Full app refresh: topology loads fully (not just quick edge polling). */
  useEffect(() => {
    if (refreshTrigger <= 0) return;
    if (mainTab !== 'topology') return;
    void fetchTopology({ mode: 'full' });
  }, [refreshTrigger, mainTab, fetchTopology]);

  const goPod = (podUid: string) => {
    navigate(`/resources/pods/uid/${encodeURIComponent(podUid)}`);
  };

  const drillConnectionsForPod = useCallback((row: NetworkActivityWorkloadRow) => {
    const ns = (row.namespace ?? '').trim();
    const name = (row.podName ?? '').trim();
    const uid = (row.podUid ?? '').trim();
    setNamespaceDraft(ns);
    setNamespaceApplied(ns);
    if (name) {
      setPodUidApplied('');
      setSearchDraft(name);
      setSearchApplied(name);
      setConnectionsScopeNote(null);
    } else {
      setSearchDraft('');
      setSearchApplied('');
      setPodUidApplied(uid);
      setConnectionsScopeNote(
        uid
          ? 'Pod name unavailable; namespace + podUid only. Add "q" to narrow further.'
          : null,
      );
    }
    setMainTab('connections');
  }, []);

  const handleManualRefresh = useCallback(() => {
    if (mainTab === 'topology') void fetchTopology({ mode: 'full', manual: true });
    else void fetchTableData(true);
  }, [mainTab, fetchTopology, fetchTableData]);

  const topologyBusy = (loading || topologyHydrating) && mainTab === 'topology';
  const tableListBusy = tableLoading && (mainTab === 'pods' || mainTab === 'connections');

  /** Topology: edges from view=edges (or connections if older Core); orphan pods from fully paginated talkers. */
  const hasGraphData =
    graphConnections.length > 0 || (graphDestinations.length > 0 && graphTalkers.length > 0);
  /** Graph inferred from edges only — no aggregate destinations/talkers (Core limited or view error). */
  const topologyLimitedFromConnectionsOnly =
    graphConnections.length > 0 && graphDestinations.length === 0 && graphTalkers.length === 0;
  const emptyContextLine = `Time range: ${sinceHuman}.`;

  const topologyStatus = useMemo(() => {
    if (mainTab !== 'topology') return null;
    const isInitialLoading =
      loading && graphDestinations.length === 0 && graphTalkers.length === 0 && graphConnections.length === 0;
    if (isInitialLoading) {
      return {
        title: 'Loading topology',
        body: 'Fetching graph data for the current filters.',
        variant: 'loading' as const,
        actionLabel: null,
      };
    }
    if (topologyHydrating && hasGraphData) {
      return {
        title: 'Topology preview',
        body: `Rendering ${graphConnections.length} sampled edge rows while labels and summary counts finish loading.`,
        variant: 'loading' as const,
        actionLabel: null,
      };
    }
    if (topologySupportIssue) {
      return {
        title: topologyLimitedFromConnectionsOnly ? 'Simplified topology' : 'Topology unavailable',
        body: topologySupportIssue,
        variant: 'warning' as const,
        actionLabel: 'Full refresh',
      };
    }
    if (topologyLimitedFromConnectionsOnly) {
      return {
        title: 'Simplified topology',
        body: `Built from ${graphConnections.length} edge rows only, no aggregate destinations or sources for the current filters.`,
        variant: 'info' as const,
        actionLabel: 'Full refresh',
      };
    }
    if (hasGraphData) {
      return {
        title: 'Topology complete',
        body: `${destTotal} destinations, ${talkerTotal} sources, ${graphConnections.length} edges in this view.`,
        variant: 'success' as const,
        actionLabel: null,
      };
    }
    return {
      title: 'No topology data',
      body: `Need both top destinations and top sources after filtering. ${emptyContextLine}`,
      variant: 'neutral' as const,
      actionLabel: 'Full refresh',
    };
  }, [
    mainTab,
    loading,
    graphDestinations.length,
    graphTalkers.length,
    graphConnections.length,
    topologyHydrating,
    topologySupportIssue,
    topologyLimitedFromConnectionsOnly,
    hasGraphData,
    destTotal,
    talkerTotal,
    emptyContextLine,
  ]);

  const topologyPosture = useMemo(() => {
    if (mainTab !== 'topology' || !networkGraphTrust) return null;
    const ctx: GraphTrustContext = {
      ...networkGraphTrust,
      clustered: graphDestinations.length + graphTalkers.length + graphConnections.length > 180,
    };
    return buildGraphTrustPosture(
      ctx,
      Math.max(graphDestinations.length + graphTalkers.length, 1),
      Math.max(graphConnections.length, 1),
    );
  }, [
    mainTab,
    networkGraphTrust,
    graphDestinations.length,
    graphTalkers.length,
    graphConnections.length,
  ]);

  const podUidLocksNamespace = Boolean(podUidApplied.trim());
  const namespaceLockedByPodUidTitle = 'Pod UID locks namespace. Use Reset to change it.';
  const topologyControlsCollapsed = mainTab === 'topology' && topologyFiltersCollapsed;
  const topologyGraphHeightClass = topologyControlsCollapsed
    ? 'lg:min-h-[calc(100dvh-12rem)] xl:min-h-[46rem] 2xl:min-h-[54rem]'
    : 'lg:min-h-[calc(100dvh-18rem)] xl:min-h-[42rem] 2xl:min-h-[48rem]';

  const tableTotal = mainTab === 'pods' ? podsTotal : connTotal;
  const tableRowsLen = mainTab === 'pods' ? podsRows.length : connRows.length;
  const tableTotalPages = Math.max(1, Math.ceil(Math.max(0, tableTotal) / tablePageSize));

  function formatQueueBytes(n?: number): string {
    if (n == null || Number.isNaN(n)) return '—';
    return n.toLocaleString();
  }

  const handleIssueAction = (issue: NetworkDataIssue) => {
    if (issue.kind === 'unauthenticated') {
      navigate('/login');
      return;
    }
    void handleManualRefresh();
  };

  return (
    <PageLayout
      compact
      fillHeight
      className="!gap-2"
      title={PAGE_TITLES.networkActivity}
      description="Observed runtime flows, workload sources, and socket evidence for the selected cluster."
    >
      {!selectedClusterId ? (
        <PageEmpty
          title="No cluster selected"
          description="Select a cluster from the header menu to load network topology and connections."
          className="py-10"
        />
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-2">
          <Card
            className="relative flex min-h-0 flex-1 flex-col overflow-hidden border border-border/70 bg-surface/35 shadow-none"
            contentClassName="flex min-h-0 flex-1 flex-col p-0"
          >
            <div className="shrink-0 space-y-2 border-b border-border/60 bg-base/55 px-3 py-2 sm:px-4">
              {/* (B) View tabs + Refresh */}
              <div className="grid min-w-0 gap-2 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
                <div className="grid grid-cols-3 gap-1 rounded-lg border border-border bg-base/60 p-1 lg:w-fit" role="tablist" aria-label="Network view mode">
                  {(
                    [
                      { id: 'topology' as const, label: 'Topology', icon: Share2 },
                      { id: 'pods' as const, label: 'Pods', icon: List },
                      { id: 'connections' as const, label: 'Connections', icon: Table2 },
                    ] as const
                  ).map(({ id, label, icon: Icon }) => (
                    <button
                      key={id}
                      type="button"
                      role="tab"
                      aria-selected={mainTab === id}
                      aria-label={`Show ${label.toLowerCase()} view`}
                      className={`inline-flex min-h-9 min-w-0 items-center justify-center gap-1.5 rounded-md border px-2.5 py-1.5 text-caption font-semibold transition-colors motion-reduce:transition-none ${
                        mainTab === id
                          ? 'border-brand/70 bg-surface text-text ring-1 ring-brand/35'
                          : 'border-transparent bg-transparent text-muted hover:border-border hover:bg-surface/50 hover:text-text'
                      }`}
                      onClick={() => setMainTab(id)}
                    >
                      <Icon className={`w-3.5 h-3.5 shrink-0 ${mainTab === id ? 'text-brand' : 'opacity-80'}`} />
                      <span className="truncate">{label}</span>
                    </button>
                  ))}
                </div>
                <div className="grid min-w-0 gap-2 sm:grid-flow-col sm:auto-cols-max sm:items-center lg:justify-end">
                  {mainTab === 'pods' && !tableLoading && (
                    <CountBadge>
                      {podsTotal} pods · p{tablePage}/{tableTotalPages}
                    </CountBadge>
                  )}
                  {mainTab === 'connections' && !tableLoading && (
                    <CountBadge>
                      {connTotal} rows · p{tablePage}/{tableTotalPages}
                    </CountBadge>
                  )}
                  {mainTab === 'topology' && hasGraphData && (
                    <CountBadge>
                      {Math.max(graphTalkers.length, 0)} src · {Math.max(graphDestinations.length, 0)} dest · {graphConnections.length} edges
                    </CountBadge>
                  )}
                  {mainTab === 'topology' ? (
                    <Button
                      variant="secondary"
                      size="sm"
                      type="button"
                      className="h-9 text-caption"
                      onClick={() => setTopologyFiltersCollapsed((v) => !v)}
                      aria-expanded={!topologyFiltersCollapsed}
                      title={topologyFiltersCollapsed ? 'Show network filters' : 'Hide network filters'}
                    >
                      <SlidersHorizontal className="mr-1 h-3.5 w-3.5" />
                      {topologyFiltersCollapsed ? 'Show filters' : 'Hide filters'}
                    </Button>
                  ) : null}
                  <Button
                    variant="secondary"
                    size="sm"
                    className="h-9 text-caption"
                    onClick={() => void handleManualRefresh()}
                    disabled={topologyBusy || tableListBusy}
                  >
                    <RefreshCw className={`w-3.5 h-3.5 mr-1 ${refreshSpin ? 'animate-spin motion-reduce:animate-none' : ''}`} />
                    Refresh
                  </Button>
                </div>
              </div>

              {/* (A) Query context — grid-owned filters */}
              {topologyControlsCollapsed ? (
                <div className="flex flex-col gap-2 rounded-lg border border-border/70 bg-surface/30 px-3 py-2 text-caption text-muted-2 sm:flex-row sm:items-center sm:justify-between">
                  <span className="min-w-0 truncate" title={appliedContextTooltip}>
                    {filterMetaText}
                  </span>
                  {(hasTextFilters || hasDraftTextFilters) ? (
                    <Button variant="secondary" size="sm" type="button" className="h-8 shrink-0 text-caption" onClick={clearTextFilters}>
                      Reset filters
                    </Button>
                  ) : null}
                </div>
              ) : (
                <>
                  <NetworkFiltersPanel title={appliedContextTooltip} compact={mainTab === 'topology'}>
                    <FilterRow
                      search={
                        <SearchFilter
                          value={searchDraft}
                          onChange={setSearchDraft}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') flushPendingFilters();
                          }}
                          placeholder={podUidLocksNamespace
                            ? 'Search within selected pod'
                            : 'Search pods, IPs, ports, UIDs'}
                        />
                      }
                      namespace={
                        <NamespaceFilter
                          value={namespaceDraft}
                          onChange={setNamespaceDraft}
                          onCommit={(value) => {
                            setNamespaceDraft(value);
                            setNamespaceApplied(value.trim());
                          }}
                          options={clusterNamespaces}
                          disabled={podUidLocksNamespace}
                          title={podUidLocksNamespace
                            ? namespaceLockedByPodUidTitle
                            : 'Type to filter; pick from suggestions; empty = all namespaces'}
                          datalistId={namespaceDatalistId}
                          placeholder={clusterNsLoading ? 'Loading…' : 'All namespaces'}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') flushPendingFilters();
                          }}
                        />
                      }
                      time={
                        <TimeFilter
                          value={sinceMinutes}
                          warnAllTime={sinceMinutes === ''}
                          onChange={async (value) => {
                            if (value === '') {
                              const confirmed = await confirm({
                                title: 'Disable time filter',
                                description: 'All time may cause very slow queries and load a large amount of data.',
                                confirmLabel: 'Use all time',
                                variant: 'danger',
                              });
                              if (confirmed) {
                                setSinceMinutes('');
                              } else {
                                if (sinceCancelTimerRef.current) window.clearTimeout(sinceCancelTimerRef.current);
                                setSinceCancelToast('Cancelled, keeping current time window.');
                                sinceCancelTimerRef.current = window.setTimeout(() => setSinceCancelToast(null), 3200);
                              }
                              return;
                            }
                            setSinceMinutes(value);
                          }}
                        />
                      }
                    />

                    {showPodUidInPrimaryFilters ? <PodUidFilter value={podUidApplied} onChange={setPodUidApplied} /> : null}

                    {showFilterMeta ? (
                      <AppliedFilterMeta
                        text={filterMetaText}
                        syncing={textDraftDiffersFromApplied}
                        showApply={textDraftDiffersFromApplied}
                        showReset={hasTextFilters || hasDraftTextFilters}
                        onApply={flushPendingFilters}
                        onReset={clearTextFilters}
                      />
                    ) : null}
                  </NetworkFiltersPanel>

                  <AdvancedFilterDisclosure>
                    {!showPodUidInPrimaryFilters ? <PodUidFilter value={podUidApplied} onChange={setPodUidApplied} /> : null}
                    <p>
                      <span className="text-muted font-medium">Topology polling:</span> ~3 min edge refresh. Use{' '}
                      <span className="text-text">Refresh</span> for a full reload.
                    </p>
                    <p>
                      <span className="text-muted font-medium">Search:</span> pod, namespace, IP, port, or short UID. Topology loads at most{' '}
                      {TOPOLOGY_EDGE_PAGE_SIZE} edge groups; talkers up to {MAX_TALKER_FETCH_PAGES_CAP} pages.
                    </p>
                  </AdvancedFilterDisclosure>
                </>
              )}

              {mainTab === 'connections' && connectionsScopeNote && (
                <div
                  className="rounded-md border border-amber-600/40 bg-amber-950/25 px-2 py-1.5 text-meta text-amber-100/95 leading-snug"
                  role="status"
                >
                  {connectionsScopeNote}
                </div>
              )}
            </div>

            {appliedPort != null ? (
              <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-border/60 bg-base/45 px-4 py-2 text-caption">
                <span className="inline-flex items-center rounded-full bg-surface-2 text-text px-2.5 py-0.5 border border-border">
                  Port = {appliedPort}
                </span>
                <span className="text-muted-2">Port filter (index-friendly)</span>
              </div>
            ) : null}

            {mainTab === 'topology' && topologyStatus && topologyStatus.variant !== 'success' ? (
              <div className="shrink-0 px-3 pt-2 sm:px-4">
                <TopologyTrustStatusStrip
                  status={topologyStatus}
                  refreshing={loading || refreshSpin}
                  onRefresh={handleManualRefresh}
                />
              </div>
            ) : null}

            <div className="flex min-h-0 flex-1 flex-col gap-2 p-2 sm:p-3">
              {mainTab === 'topology' && (
                <section className="flex min-h-0 flex-1 flex-col gap-2">
                  <div className="grid min-h-0 flex-1 gap-2">
                    <div className={`relative min-h-[32rem] min-w-0 overflow-hidden rounded-lg border border-border bg-surface/25 ${topologyGraphHeightClass}`}>
                      {topologyBusy &&
                      graphDestinations.length === 0 &&
                      graphTalkers.length === 0 &&
                      graphConnections.length === 0 ? (
                        <TopologyLoadingSurface />
                      ) : topologyDataIssue ? (
                        <NetworkDataIssuePanel
                          issue={topologyDataIssue}
                          onAction={() => handleIssueAction(topologyDataIssue)}
                          className={topologyGraphHeightClass}
                        />
                      ) : !hasGraphData ? (
                        <div className={`flex min-h-[32rem] flex-col overflow-y-auto px-4 py-8 ${topologyGraphHeightClass}`}>
                          <PageEmpty
                            title={topologyStatus?.title ?? 'No topology data'}
                            description={topologyStatus?.body ?? `No destinations or sources matched these filters. ${emptyContextLine}`}
                            className="py-8"
                          />
                          {hasTextFilters && (
                            <div className="mt-4 flex justify-center">
                              <Button variant="secondary" size="sm" onClick={clearTextFilters}>
                                Clear filters (ns / q / podUid)
                              </Button>
                            </div>
                          )}
                        </div>
                      ) : (
                        <div className={`h-full min-h-[32rem] p-1.5 sm:p-2 ${topologyGraphHeightClass}`}>
                          <NetworkTopologyGraph
                            key={`${selectedClusterId}|${namespaceApplied}|${searchApplied}|${podUidApplied}|${sinceMinutes}`}
                            className={`h-full min-h-[32rem] w-full ${topologyGraphHeightClass}`}
                            destinations={graphDestinations}
                            talkers={graphTalkers}
                            connections={graphConnections}
                            supplementTalkers={graphTalkers}
                            maxNodes={120}
                            showCompactLegend
                            showTrafficParticles={false}
                            podNamesByUid={topologyPodNamesByUid}
                            graphTrust={networkGraphTrust}
                            showTrustOverlay={false}
                            onSelectionChange={setTopologySelection}
                          />
                          {topologySelection ? (
                            <div className="absolute bottom-3 right-3 z-20 hidden max-w-[22rem] rounded-lg border border-border/80 bg-base/95 p-2.5 text-caption text-text shadow-lg backdrop-blur lg:block">
                              <div className="mb-1 font-semibold">Selected node</div>
                              <p className="text-muted">{topologySelection.typeLabel}</p>
                              <p className="break-words font-medium">{topologySelection.line1}</p>
                              <p className="break-words text-muted-2">{topologySelection.line2}</p>
                              {topologySelection.ownerLabel ? <p className="text-muted-2">Owner: {topologySelection.ownerLabel}</p> : null}
                              {topologySelection.kind === 'pod' ? (
                                <Button
                                  variant="secondary"
                                  size="sm"
                                  type="button"
                                  className="mt-2 h-8 text-caption"
                                  onClick={() => goPod(topologySelection.id.replace(/^pod:/, ''))}
                                >
                                  <ExternalLink className="mr-1 h-3.5 w-3.5" />
                                  Open pod
                                </Button>
                              ) : null}
                            </div>
                          ) : null}
                        </div>
                      )}
                    </div>
                  </div>
                  {hasGraphData ? (
                    <div className="flex justify-end">
                      <Button variant="secondary" size="sm" type="button" className="h-8 w-full text-caption sm:w-auto" onClick={() => setLegendDrawerOpen(true)}>
                        Open legend and selection
                      </Button>
                    </div>
                  ) : null}
                  <TopologyMetaDrawer
                    open={legendDrawerOpen}
                    posture={topologyPosture}
                    selection={topologySelection}
                    onClose={() => setLegendDrawerOpen(false)}
                    onOpenPod={goPod}
                  />
                </section>
              )}

              {(mainTab === 'pods' || mainTab === 'connections') && (
                <div
                  className="relative flex min-h-0 w-full flex-1 flex-col overflow-hidden rounded-lg border border-border bg-surface/35"
                >
                  {tableListBusy && (mainTab === 'pods' ? podsRows.length === 0 : connRows.length === 0) ? (
                    <div className="flex flex-col flex-1 min-h-0 justify-center items-center space-y-3 py-8">
                      <div className="h-10 w-10 animate-spin rounded-full border-2 border-brand border-t-transparent motion-reduce:animate-none" />
                      <span className="text-muted text-body">Loading…</span>
                    </div>
                  ) : tableDataIssue ? (
                    <NetworkDataIssuePanel
                      issue={tableDataIssue}
                      onAction={() => handleIssueAction(tableDataIssue)}
                    />
                  ) : mainTab === 'pods' && podsRows.length === 0 ? (
                    <div className="flex flex-1 min-h-0 items-center justify-center p-4">
                      <PageEmpty title="No pods" description={emptyContextLine} className="py-6 max-w-md" />
                    </div>
                  ) : mainTab === 'connections' && connRows.length === 0 ? (
                    <div className="flex flex-1 min-h-0 items-center justify-center p-4">
                      <PageEmpty title="No connections" description={emptyContextLine} className="py-6 max-w-md" />
                    </div>
                  ) : (
                    <>
                      {tableListBusy && tableRowsLen > 0 && (
                        <div className="absolute top-2 right-2 z-20 flex items-center gap-2 rounded-md bg-surface/95 border border-border px-2 py-1 text-caption text-text shadow-lg">
                          <RefreshCw className="w-3 h-3 animate-spin shrink-0 motion-reduce:animate-none" />
                          Refreshing…
                        </div>
                      )}
                      <div className="min-h-0 flex-1 overflow-auto p-2 sm:p-3">
                        {mainTab === 'pods' ? (
                          <table className={`${UI_TABLE} min-w-[640px] text-caption`}>
                            <thead className={UI_THEAD_STICKY}>
                              <tr>
                                <th className={UI_TH_COMPACT}>Namespace</th>
                                <th className={UI_TH_COMPACT}>Pod</th>
                                <th
                                  className={`${UI_TH_COMPACT} cursor-help`}
                                  title="Snapshot record count per 5-min bucket, not unique connection count."
                                >
                                  Observed (bucket)
                                </th>
                                <th className={UI_TH_COMPACT}>Updated</th>
                                <th className={UI_TH_COMPACT}>Node</th>
                                <th
                                  className={`${UI_TH_COMPACT} text-right`}
                                  title="Copy UID · Connections · Pod detail"
                                >
                                  Actions
                                </th>
                              </tr>
                            </thead>
                            <tbody>
                              {podsRows.map((row) => (
                                <tr
                                  key={row.podUid}
                                  role="button"
                                  tabIndex={0}
                                  className={`${UI_TR} cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-inset`}
                                  onClick={(e) => {
                                    if ((e.target as HTMLElement).closest('button')) return;
                                    drillConnectionsForPod(row);
                                  }}
                                  onKeyDown={(event) => {
                                    if (event.key === 'Enter' || event.key === ' ') {
                                      event.preventDefault();
                                      drillConnectionsForPod(row);
                                    }
                                  }}
                                  title="Click row (outside buttons) to open Connections tab filtered by pod"
                                >
                                  <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-text`}>{row.namespace}</td>
                                  <td className={UI_TD_COMPACT_TIGHT}>{podTableLabel(row.podName, row.podUid)}</td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} tabular-nums`}>{row.connectionCount}</td>
                                  <td
                                    className={`${UI_TD_COMPACT_TIGHT} text-muted whitespace-nowrap`}
                                    title={observedAtTooltip(row.lastObservedAt)}
                                  >
                                    {formatObservedAt(row.lastObservedAt)}
                                  </td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} text-muted`}>{row.nodeName ?? '—'}</td>
                                  <td className={`${UI_TD_COMPACT_TIGHT} text-right whitespace-nowrap`}>
                                    <Button
                                      variant="ghost"
                                      size="sm"
                                      type="button"
                                      className="h-8 w-8 p-0 text-muted hover:text-text"
                                      title="Copy pod UID"
                                      aria-label="Copy pod UID"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        copyToClipboard(`na-pod-${row.podUid}`, row.podUid);
                                      }}
                                    >
                                      {copiedTableKey === `na-pod-${row.podUid}` ? (
                                        <Check className="w-3.5 h-3.5 text-emerald-400" />
                                      ) : (
                                        <Copy className="w-3.5 h-3.5" />
                                      )}
                                    </Button>
                                    <Button
                                      variant="ghost"
                                      size="sm"
                                      type="button"
                                      className="h-8 px-2 text-caption"
                                      title="View connections (filtered by pod)"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        drillConnectionsForPod(row);
                                      }}
                                    >
                                      <ChevronRight className="mr-1 h-3.5 w-3.5" />
                                      Connections
                                    </Button>
                                    <Button
                                      variant="ghost"
                                      size="sm"
                                      type="button"
                                      className="h-8 px-2 text-caption"
                                      title="Pod detail"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        goPod(row.podUid);
                                      }}
                                    >
                                      <ExternalLink className="mr-1 h-3.5 w-3.5" />
                                      Detail
                                    </Button>
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        ) : (
                          <table className={`${UI_TABLE} min-w-[48rem] md:min-w-[56rem] lg:min-w-full text-caption`}>
                            <thead className={UI_THEAD_STICKY}>
                              <tr>
                                <th
                                  className={`${UI_TH_COMPACT} min-w-[9rem] cursor-help`}
                                  title="observedAt when Core received (cell tooltip: local time + ISO UTC); subline is 5-min bucket start UTC."
                                >
                                  Time
                                </th>
                                <th className={UI_TH_COMPACT}>NS</th>
                                <th className={UI_TH_COMPACT} title="Copy pod UID button">
                                  Pod
                                </th>
                                <th
                                  className={UI_TH_COMPACT}
                                  title="Source: IP:port. LISTEN state: shows listening socket (LISTEN :port @bind); copy button copies the same content."
                                >
                                  Local
                                </th>
                                <th
                                  className={UI_TH_COMPACT}
                                  title="Remote destination. LISTEN: not used (shows —); copy only when destination exists."
                                >
                                  Remote
                                </th>
                                <th className={UI_TH_COMPACT}>Proto</th>
                                <th className={UI_TH_COMPACT}>State</th>
                                <th
                                  className={`${UI_TH_COMPACT} text-right cursor-help`}
                                  title="tx_queue from /proc/net (snapshot), not total sent traffic."
                                >
                                  Tx queue
                                </th>
                                <th
                                  className={`${UI_TH_COMPACT} text-right cursor-help`}
                                  title="rx_queue from /proc/net (snapshot), not total received traffic."
                                >
                                  Rx queue
                                </th>
                                <th className={UI_TH_COMPACT}>Owner</th>
                                <th className={UI_TH_COMPACT}>Node</th>
                                <th
                                  className={`${UI_TH_COMPACT} text-right w-[3.25rem]`}
                                  title="Open Pod detail"
                                >
                                  Detail
                                </th>
                              </tr>
                            </thead>
                            <tbody>
                              {connRows.map((row, i) => {
                                const bucket = formatBucket5mLine(row.bucket5m);
                                const k =
                                  row.id != null
                                    ? `c-${row.id}`
                                    : `c-${i}-${row.podUid}-${row.bucket5m ?? ''}-${row.destIp}-${row.destPort}-${row.protocol}`;
                                return (
                                  <tr key={k} className={UI_TR}>
                                    <td className={`${UI_TD_COMPACT_TIGHT} align-top`}>
                                      <div
                                        className="text-text whitespace-nowrap"
                                        title={observedAtTooltip(row.observedAt)}
                                      >
                                        {formatObservedAt(row.observedAt)}
                                      </div>
                                      <div className="text-caption text-muted mt-0.5 font-mono" title={bucket.title}>
                                        {bucket.short}
                                      </div>
                                    </td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-text align-top`}>{row.namespace ?? '—'}</td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} align-top`}>
                                      <div className="flex items-center gap-0.5 min-w-0">
                                        <span className="min-w-0">{podTableLabel(row.podName, row.podUid)}</span>
                                        {(row.podUid ?? '').trim() ? (
                                          <Button
                                            variant="ghost"
                                            size="sm"
                                            type="button"
                                            className="h-8 w-8 p-0 shrink-0 text-muted hover:text-text"
                                            title="Copy pod UID"
                                            aria-label="Copy pod UID"
                                            onClick={() =>
                                              copyToClipboard(`na-cu-${k}`, (row.podUid ?? '').trim())
                                            }
                                          >
                                            {copiedTableKey === `na-cu-${k}` ? (
                                              <Check className="w-3.5 h-3.5 text-emerald-400" />
                                            ) : (
                                              <Copy className="w-3.5 h-3.5" />
                                            )}
                                          </Button>
                                        ) : null}
                                      </div>
                                    </td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} align-top`}>
                                      <div className="flex items-start gap-0.5 min-w-0">
                                        <span className="font-mono text-muted whitespace-pre-wrap break-all min-w-0">
                                          {formatLocalEndpoint(row)}
                                        </span>
                                        {localEndpointClipboard(row) !== '—' ? (
                                          <Button
                                            variant="ghost"
                                            size="sm"
                                            type="button"
                                            className="h-8 w-8 p-0 shrink-0 text-muted hover:text-text"
                                            title="Copy Local (as displayed)"
                                            aria-label="Copy Local"
                                            onClick={() =>
                                              copyToClipboard(`na-cl-${k}`, localEndpointClipboard(row))
                                            }
                                          >
                                            {copiedTableKey === `na-cl-${k}` ? (
                                              <Check className="w-3.5 h-3.5 text-emerald-400" />
                                            ) : (
                                              <Copy className="w-3.5 h-3.5" />
                                            )}
                                          </Button>
                                        ) : null}
                                      </div>
                                    </td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} align-top`}>
                                      <div className="flex items-start gap-0.5 min-w-0">
                                        <span className="font-mono text-text whitespace-pre-wrap break-all min-w-0">
                                          {formatRemoteEndpoint(row)}
                                        </span>
                                        {remoteEndpointClipboard(row) !== '—' ? (
                                          <Button
                                            variant="ghost"
                                            size="sm"
                                            type="button"
                                            className="h-8 w-8 p-0 shrink-0 text-muted hover:text-text"
                                            title="Copy Remote (as displayed)"
                                            aria-label="Copy Remote"
                                            onClick={() =>
                                              copyToClipboard(`na-cr-${k}`, remoteEndpointClipboard(row))
                                            }
                                          >
                                            {copiedTableKey === `na-cr-${k}` ? (
                                              <Check className="w-3.5 h-3.5 text-emerald-400" />
                                            ) : (
                                              <Copy className="w-3.5 h-3.5" />
                                            )}
                                          </Button>
                                        ) : null}
                                      </div>
                                    </td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} align-top`}>{row.protocol ?? '—'}</td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} align-top`}>{row.state ?? '—'}</td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums align-top`}>{formatQueueBytes(row.bytesSent)}</td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} text-right tabular-nums align-top`}>{formatQueueBytes(row.bytesRecv)}</td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} text-muted align-top`}>
                                      {row.ownerKind || row.ownerName
                                        ? `${row.ownerKind ?? ''}${row.ownerKind && row.ownerName ? '/' : ''}${row.ownerName ?? ''}`
                                        : '—'}
                                    </td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} text-muted align-top`}>{row.nodeName ?? '—'}</td>
                                    <td className={`${UI_TD_COMPACT_TIGHT} text-right align-top whitespace-nowrap`}>
                                      {(row.podUid ?? '').trim() ? (
                                        <Button
                                          variant="ghost"
                                          size="sm"
                                          type="button"
                                          className="h-8 w-8 p-0"
                                          title="Open Pod detail"
                                          aria-label="Open Pod detail"
                                          onClick={() => goPod((row.podUid ?? '').trim())}
                                        >
                                          <ExternalLink className="w-3.5 h-3.5" />
                                        </Button>
                                      ) : (
                                        <span className="text-muted-2">—</span>
                                      )}
                                    </td>
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        )}
                      </div>
                      <div className="flex shrink-0 flex-col gap-2 border-t border-border bg-base/90 px-3 py-2 sm:flex-row sm:items-center sm:justify-between">
                        <div className="flex items-center justify-between gap-2 text-caption text-muted sm:justify-start">
                          <span>Rows/page</span>
                          <select
                            value={tablePageSize}
                            onChange={(e) => setTablePageSize(Number(e.target.value))}
                            className="h-9 rounded border border-border bg-surface px-2 text-text focus:border-brand focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
                          >
                            {TABLE_PAGE_SIZES.map((n) => (
                              <option key={n} value={n}>
                                {n}
                              </option>
                            ))}
                          </select>
                        </div>
                        <div className="flex items-center justify-between gap-1 sm:justify-end">
                          <Button
                            variant="secondary"
                            size="sm"
                            type="button"
                            className="h-8 px-2"
                            disabled={tablePage <= 1 || tableListBusy}
                            onClick={() => setTablePage((p) => Math.max(1, p - 1))}
                          >
                            <ChevronLeft className="w-4 h-4" />
                          </Button>
                          <span className="text-caption text-muted tabular-nums min-w-[5rem] text-center">
                            {tablePage} / {tableTotalPages}
                          </span>
                          <Button
                            variant="secondary"
                            size="sm"
                            type="button"
                            className="h-8 px-2"
                            disabled={tablePage >= tableTotalPages || tableListBusy}
                            onClick={() => setTablePage((p) => p + 1)}
                          >
                            <ChevronRight className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </>
                  )}
                </div>
              )}
            </div>
            {copyErrorToast && (
              <div
                className="pointer-events-none absolute bottom-3 left-1/2 z-modal max-w-[min(100%,22rem)] -translate-x-1/2 rounded-lg border border-red-500/40 bg-base/95 px-3 py-2 text-center text-caption leading-snug text-red-200 shadow-lg"
                role="alert"
              >
                {copyErrorToast}
              </div>
            )}
            {sinceCancelToast && (
              <div
                className="pointer-events-none absolute z-popover max-w-[min(100%,22rem)] -translate-x-1/2 rounded-lg border border-border/60 bg-surface/95 px-3 py-2 text-center text-caption leading-snug text-text shadow-lg left-1/2"
                style={{ bottom: copyErrorToast ? '4.25rem' : '0.75rem' }}
                role="status"
              >
                {sinceCancelToast}
              </div>
            )}
          </Card>
        </div>
      )}
    </PageLayout>
  );
}
