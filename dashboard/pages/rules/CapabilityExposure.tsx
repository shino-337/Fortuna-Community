import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { api, getAvailabilityIssue, type AvailabilityIssue } from '../../lib/api';
import type {
  PodCapabilityDetail,
  PodCapabilitySummaryNamespace,
  PodCapabilitySummarySeverity,
  PodCapabilityTrendPoint,
} from '../../types';
import { Button } from '../../components/ui/Button';
import { ResetFiltersButton } from '../../components/ResetFiltersButton';
import { AvailabilityNotice } from '../../components/AvailabilityNotice';
import { Pagination } from '../../components/Pagination';
import { PageLayout } from '../../design-system/layouts/PageLayout';
import { RULES_CATALOG_SECTIONS, SectionNav } from '../../components/SectionNav';
import { PAGE_TITLES } from '../../lib/pageTitles';
import { usePolling, REFRESH_INTERVALS } from '../../hooks/usePolling';
import { useClusters } from '../../hooks/useClusters';
import { useClusterStore } from '../../store/clusterStore';
import { useRefreshIntervalStore } from '../../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../../store/refreshTriggerStore';
import { getSeverityBadgeClass, getSeverityTextClass } from '../../lib/severity';
import { getChartThemeColors } from '../../lib/chartTheme';
import { downloadText, toCsv } from '../../lib/download';
import { UI_TABLE, UI_TD_COMPACT_TIGHT, UI_TH_COMPACT, UI_TR, UI_THEAD_STICKY } from '../../lib/tableChrome';

type PceSort = 'severity_desc' | 'severity_asc' | 'capability_asc' | 'pod_asc' | 'namespace_asc';

const severityRank: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1 };

function summarizePceEvidence(ev: Record<string, unknown> | undefined): { label: string; title: string } {
  if (!ev || typeof ev !== 'object' || Object.keys(ev).length === 0) {
    return { label: '', title: '' };
  }
  const full = JSON.stringify(ev);
  const keys = Object.keys(ev).slice(0, 5);
  const label = keys
    .map((k) => {
      const v = ev[k];
      const vs = v != null && typeof v === 'object' ? JSON.stringify(v).slice(0, 32) : String(v ?? '').slice(0, 32);
      return `${k}: ${vs}`;
    })
    .join(' · ');
  return { label: label.length > 90 ? `${label.slice(0, 87)}…` : label, title: full };
}

/**
 * Capability exposure: which pods hold which capabilities from the catalog, by namespace and severity.
 * Moved here from Findings, where it sat beside the triage queue; it reads like the catalog's other half.
 */
export const CapabilityExposure: React.FC = () => {
  const { selectedClusterId } = useClusterStore();
  const { clusters } = useClusters();
  const chartTheme = getChartThemeColors();
  const [pceSummary, setPceSummary] = useState<PodCapabilitySummarySeverity[]>([]);
  const [pceDetails, setPceDetails] = useState<PodCapabilityDetail[]>([]);
  const [pceIssue, setPceIssue] = useState<AvailabilityIssue | null>(null);
  const pceRequestRef = useRef(0);
  const [pceClusterId, setPceClusterId] = useState('');
  const [pceNamespace, setPceNamespace] = useState('');
  const [pceCapabilityId, setPceCapabilityId] = useState('');
  const [pcePodName, setPcePodName] = useState('');
  const [pceSeverityFilter, setPceSeverityFilter] = useState<string>('');
  /** When user clicks a heatmap cell, filter drill-down table by this namespace + severity. */
  const [pceHeatmapFilter, setPceHeatmapFilter] = useState<{ namespace: string; severity: string } | null>(null);
  /** Drill-down server pagination (GET /inventory/pod-capabilities returns total). */
  const [pceListPage, setPceListPage] = useState(1);
  const [pceListPageSize, setPceListPageSize] = useState(25);
  const [pceListTotal, setPceListTotal] = useState(0);
  const [pceSort, setPceSort] = useState<PceSort>('severity_desc');
  const [pceTrend, setPceTrend] = useState<PodCapabilityTrendPoint[]>([]);
  const [pceHeatmap, setPceHeatmap] = useState<PodCapabilitySummaryNamespace[]>([]);
  const [pceTrendDays, setPceTrendDays] = useState(7);
  const [heatmapShowAll, setHeatmapShowAll] = useState(false);
  const effectiveClusterId = selectedClusterId ?? undefined;

  useEffect(() => {
    pceRequestRef.current += 1;
    setPceSummary([]);
    setPceDetails([]);
    setPceListTotal(0);
    setPceTrend([]);
    setPceHeatmap([]);
    setPceIssue(null);
  }, [effectiveClusterId, pceClusterId]);

  const scopeClusterDisplay = effectiveClusterId
    ? (clusters.find((c) => c.id === effectiveClusterId)?.name || effectiveClusterId)
    : null;

  const fetchData = useCallback(async () => {
    const requestId = ++pceRequestRef.current;
    const clusterId = effectiveClusterId;
    const pceDrillCluster = (pceClusterId || '').trim() || clusterId;
    const pceResults = await Promise.allSettled([
      api.getPceSummaryBySeverity({ clusterId: pceDrillCluster || undefined }),
      api.getPceCapabilities({
        clusterId: pceDrillCluster || undefined,
        namespace: pceNamespace.trim() || undefined,
        severity: pceSeverityFilter || undefined,
        podName: pcePodName.trim() || undefined,
        capabilityId: pceCapabilityId.trim() || undefined,
        limit: pceListPageSize,
        offset: (pceListPage - 1) * pceListPageSize,
      }),
      api.getPceTrend(pceTrendDays, { clusterId: clusterId ?? undefined }),
      api.getPceSummaryByNamespace({ clusterId: clusterId ?? undefined }),
    ]);
    if (requestId !== pceRequestRef.current) return;
    const [pceSummaryResult, pceListResult, pceTrendResult, pceHeatmapResult] = pceResults;
    const failed = pceResults.find((result) => result.status === 'rejected');
    setPceIssue(failed?.status === 'rejected' ? getAvailabilityIssue(failed.reason, 'Capability data') : null);
    if (pceSummaryResult.status === 'fulfilled') setPceSummary(pceSummaryResult.value);
    if (pceListResult.status === 'fulfilled') {
      setPceDetails(pceListResult.value.capabilities);
      setPceListTotal(pceListResult.value.total);
    }
    if (pceTrendResult.status === 'fulfilled') setPceTrend(Array.isArray(pceTrendResult.value) ? pceTrendResult.value : []);
    if (pceHeatmapResult.status === 'fulfilled') setPceHeatmap(Array.isArray(pceHeatmapResult.value) ? pceHeatmapResult.value : []);
  }, [effectiveClusterId, pceClusterId, pceNamespace, pceSeverityFilter, pcePodName, pceCapabilityId, pceListPage, pceListPageSize, pceTrendDays]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.SBOM_RISK_LIST));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(fetchData, intervalMs, { refreshTrigger });

  const loadPceDetails = async (params: Parameters<typeof api.getPceCapabilities>[0]) => {
    const requestId = ++pceRequestRef.current;
    try {
      const result = await api.getPceCapabilities(params);
      if (requestId !== pceRequestRef.current) return;
      setPceDetails(result.capabilities);
      setPceListTotal(result.total);
    } catch (err) {
      if (requestId !== pceRequestRef.current) return;
      setPceIssue(getAvailabilityIssue(err, 'Capability inventory'));
    }
  };

  const sortedPceDetails = useMemo(() => {
    const out = [...pceDetails];
    out.sort((a, b) => {
      const aSev = (a.severity || '').toLowerCase();
      const bSev = (b.severity || '').toLowerCase();
      switch (pceSort) {
        case 'severity_asc':
          return (severityRank[aSev] ?? 0) - (severityRank[bSev] ?? 0);
        case 'capability_asc':
          return (a.capabilityId || '').localeCompare(b.capabilityId || '');
        case 'pod_asc':
          return (a.podName || a.podUid || '').localeCompare(b.podName || b.podUid || '');
        case 'namespace_asc':
          return (a.namespace || '').localeCompare(b.namespace || '');
        case 'severity_desc':
        default:
          return (severityRank[bSev] ?? 0) - (severityRank[aSev] ?? 0);
      }
    });
    return out;
  }, [pceDetails, pceSort]);

  /** True when any drill-down filter or sort differs from its default. */
  const hasPceFilters =
    pceClusterId !== '' ||
    pceNamespace !== '' ||
    pceCapabilityId !== '' ||
    pcePodName !== '' ||
    pceSeverityFilter !== '' ||
    pceHeatmapFilter != null ||
    pceSort !== 'severity_desc';

  /** Clears the drill-down filters and sort; the list refetches from page 1. */
  const resetPceFilters = useCallback(() => {
    setPceClusterId('');
    setPceNamespace('');
    setPceCapabilityId('');
    setPcePodName('');
    setPceSeverityFilter('');
    setPceHeatmapFilter(null);
    setPceSort('severity_desc');
    setPceListPage(1);
  }, []);

  return (
    <PageLayout
      title={PAGE_TITLES.policyRules}
      description="Which pods hold the capabilities in the catalog, by namespace and severity. Use it to find over-privileged workloads before they show up as findings."
    >
      <SectionNav sections={RULES_CATALOG_SECTIONS} ariaLabel="Rules and catalog sections" />
    <div className="space-y-6">
      {pceIssue ? <AvailabilityNotice issue={pceIssue} onRetry={pceIssue.retryable ? () => { void fetchData(); } : undefined} /> : null}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <h2 className="text-section-title text-text">Pods by capability and severity</h2>
        {pceHeatmap.length > 0 && (
          <Button
            variant="secondary"
            size="sm"
            onClick={() => {
              const csv = toCsv([
                ['namespace', 'severity', 'count'],
                ...pceHeatmap.map((r) => [r.namespace ?? '', r.severity ?? '', r.count]),
              ]);
              downloadText(csv, `pce-heatmap-${new Date().toISOString().slice(0, 10)}.csv`, 'text/csv;charset=utf-8');
            }}
            title="Export heatmap data (namespace, severity, count) as CSV"
          >
            Export Heatmap
          </Button>
        )}
      </div>
      {/* Capability exposure summary: capability counts, not risk counts */}
      <div className="bg-surface border border-border p-4 rounded-lg">
	                <h2 className="text-body font-semibold text-muted uppercase tracking-wider mb-1">
	              Capability exposure summary
        </h2>
        <p className="text-caption text-muted mb-3">
          Counts by severity from current pod capability inventory. These numbers are separate from active findings and do not use the Findings time window.
          {scopeClusterDisplay ? ` Scoped to cluster: ${scopeClusterDisplay}.` : ' All clusters.'}
        </p>
        {pceSummary.length === 0 && !pceIssue ? (
          <p className="text-body text-muted">No capability exposure data available for this scope.</p>
        ) : pceSummary.length > 0 ? (
          <div className="grid grid-cols-1 gap-3 min-[420px]:grid-cols-2 lg:grid-cols-4">
            {pceSummary.map((row) => (
              <div key={row.severity} className="bg-base border border-border rounded p-3 flex items-center justify-between">
                <span className={`text-caption uppercase ${getSeverityTextClass(row.severity)}`}>{row.severity}</span>
                <span className="text-body font-bold text-text">{row.count}</span>
              </div>
            ))}
          </div>
        ) : null}
      </div>

      {/* Capability exposure trend */}
      {pceTrend.length > 0 && (
        <div className="bg-surface border border-border p-4 rounded-lg">
          <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
            <h2 className="text-body font-semibold text-muted uppercase tracking-wider">Capability exposure trend (last {pceTrendDays === 1 ? '24h' : `${pceTrendDays}d`})</h2>
            <div className="flex items-center gap-1 text-caption text-muted">
              <span>Range:</span>
              {[1, 7, 30].map((d) => (
                <button
                  key={d}
                  type="button"
                  onClick={() => setPceTrendDays(d)}
                  className={`px-2 py-0.5 rounded-full border ${
                    pceTrendDays === d
                      ? 'border-brand text-brand/90 bg-brand/10'
                      : 'border-border text-muted hover:border-muted'
                  }`}
                >
                  {d === 1 ? '24h' : `${d}d`}
                </button>
              ))}
            </div>
          </div>
          <p className="text-caption text-muted mb-3">Capability exposure counts by day. Same cluster scope as this route.</p>
          {pceTrend.length >= 2 ? (
            <div className="h-[220px] min-h-[220px] w-full min-w-0">
              <ResponsiveContainer width="100%" height="100%" minWidth={1} minHeight={220}>
                <AreaChart data={pceTrend.map((p) => ({ ...p, total: (p.critical ?? 0) + (p.high ?? 0) + (p.medium ?? 0) + (p.low ?? 0) }))}>
                  <CartesianGrid strokeDasharray="3 3" stroke={chartTheme.grid} />
                  <XAxis dataKey="date" tick={{ fill: chartTheme.axis, fontSize: 11 }} />
                  <YAxis tick={{ fill: chartTheme.axis, fontSize: 11 }} />
                  <Tooltip contentStyle={{ backgroundColor: chartTheme.tooltipBg, border: `1px solid ${chartTheme.tooltipBorder}` }} labelStyle={{ color: chartTheme.axis }} />
                  <Area type="monotone" dataKey="critical" stackId="1" stroke={chartTheme.critical} fill={chartTheme.critical} fillOpacity={0.6} name="Critical" />
                  <Area type="monotone" dataKey="high" stackId="1" stroke={chartTheme.high} fill={chartTheme.high} fillOpacity={0.6} name="High" />
                  <Area type="monotone" dataKey="medium" stackId="1" stroke={chartTheme.medium} fill={chartTheme.medium} fillOpacity={0.6} name="Medium" />
                  <Area type="monotone" dataKey="low" stackId="1" stroke={chartTheme.low} fill={chartTheme.low} fillOpacity={0.6} name="Low" />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <div className="flex h-[220px] items-center justify-center rounded-lg border border-border/60 bg-base/25 px-4 text-center text-body text-muted">
              Trend needs at least two time buckets for this scope and range.
            </div>
          )}
        </div>
      )}

      {/* PCE Heatmap: namespace × severity – click cell to filter drill-down table (Phase 3) */}
      {pceHeatmap.length > 0 && (() => {
        const byNs: Record<string, { critical: number; high: number; medium: number; low: number }> = {};
        pceHeatmap.forEach((r) => {
          if (!byNs[r.namespace]) byNs[r.namespace] = { critical: 0, high: 0, medium: 0, low: 0 };
          const sev = (r.severity || '').toLowerCase();
          if (sev in byNs[r.namespace]) (byNs[r.namespace] as Record<string, number>)[sev] = r.count;
        });
        const namespaces = Object.keys(byNs).sort();
        const maxCount = Math.max(1, ...namespaces.flatMap((ns) => Object.values(byNs[ns])));
        const handleHeatmapCellClick = async (ns: string, severity: string) => {
          setPceHeatmapFilter({ namespace: ns, severity });
          setPceNamespace(ns);
          setPceSeverityFilter(severity);
          setPceListPage(1);
          const merged =
            (pceClusterId || '').trim() ||
            effectiveClusterId ||
            selectedClusterId ||
            undefined;
          await loadPceDetails({
            clusterId: merged,
            namespace: ns,
            severity,
            limit: pceListPageSize,
            offset: 0,
          });
        };
        return (
          <div className="bg-surface border border-border p-4 rounded-lg">
            <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
              <h2 className="text-body font-semibold text-muted uppercase tracking-wider">Exposure by namespace</h2>
              {namespaces.length > 30 && (
                <Button variant="secondary" size="sm" type="button" onClick={() => setHeatmapShowAll((v) => !v)}>
                  {heatmapShowAll ? 'Show first 30 only' : `Show all (${namespaces.length})`}
                </Button>
              )}
            </div>
            <p className="text-caption text-muted mb-3">Click a cell with count &gt; 0 to filter the table below by that namespace and severity.</p>
            <div className="overflow-x-auto max-h-[min(70dvh,520px)] overflow-y-auto overscroll-contain rounded-lg border border-border">
              <table className={UI_TABLE}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH_COMPACT}>Namespace</th>
                    <th className={`${UI_TH_COMPACT} text-right text-red-400`}>Critical</th>
                    <th className={`${UI_TH_COMPACT} text-right text-orange-400`}>High</th>
                    <th className={`${UI_TH_COMPACT} text-right text-yellow-500`}>Medium</th>
                    <th className={`${UI_TH_COMPACT} text-right text-green-400`}>Low</th>
                  </tr>
                </thead>
                <tbody>
                  {(heatmapShowAll ? namespaces : namespaces.slice(0, 30)).map((ns) => {
                    const row = byNs[ns];
                    const td = (n: number, sev: string, rgba: string) => (
                      <td
                        key={sev}
                        role={n > 0 ? 'button' : undefined}
                        tabIndex={n > 0 ? 0 : undefined}
                        className={`text-right py-1.5 px-2 rounded ${n > 0 ? 'cursor-pointer hover:ring-2 hover:ring-inset hover:ring-white/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-inset' : 'cursor-default opacity-50'}`}
                        style={{ backgroundColor: `rgba(${rgba},${0.15 + (n / maxCount) * 0.75})` }}
                        onClick={() => n > 0 && handleHeatmapCellClick(ns, sev)}
                        onKeyDown={(event) => {
                          if (n > 0 && (event.key === 'Enter' || event.key === ' ')) {
                            event.preventDefault();
                            handleHeatmapCellClick(ns, sev);
                          }
                        }}
                        title={n > 0 ? `Filter table by ${ns}, ${sev}` : 'No exposures in this cell'}
                      >
                        {n}
                      </td>
                    );
                    return (
                      <tr key={ns} className={UI_TR}>
                        <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-text`}>{ns}</td>
                        {td(row.critical, 'critical', '220,38,38')}
                        {td(row.high, 'high', '234,88,12')}
                        {td(row.medium, 'medium', '202,138,4')}
                        {td(row.low, 'low', '22,163,74')}
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              {!heatmapShowAll && namespaces.length > 30 && (
                <p className="text-caption text-muted mt-2">Showing first 30 of {namespaces.length} namespaces. Use &quot;Show all&quot; to expand.</p>
              )}
            </div>
          </div>
        );
      })()}

      {/* PCE Drill-down */}
      <div className="bg-surface border border-border p-4 rounded-lg">
        <h2 className="text-body font-semibold text-muted uppercase tracking-wider mb-2">
          Capability drill-down by pod
        </h2>
        <p className="text-caption text-muted mb-2">
          Explore pod-level capabilities that may explain exposure behind findings. Click a heatmap cell above to filter by namespace and severity, or use the filters below.
        </p>
        {pceHeatmapFilter && (
          <div className="mb-3 flex items-center gap-2 flex-wrap">
            <span className="text-caption text-muted">Filtered by namespace <strong className="text-text">{pceHeatmapFilter.namespace}</strong>, severity <strong className="text-text">{pceHeatmapFilter.severity}</strong></span>
            <Button
              variant="secondary"
              size="sm"
              onClick={async () => {
                setPceHeatmapFilter(null);
                setPceNamespace('');
                setPceSeverityFilter('');
                setPceListPage(1);
                const merged =
                  (pceClusterId || '').trim() ||
                  effectiveClusterId ||
                  selectedClusterId ||
                  undefined;
                await loadPceDetails({
                  clusterId: merged,
                  limit: pceListPageSize,
                  offset: 0,
                });
              }}
            >
              Clear filter
            </Button>
          </div>
        )}
        <div className="grid grid-cols-1 md:grid-cols-6 gap-3 mb-4">
          <div>
            <label className="block text-caption text-muted mb-1">Cluster</label>
            <select
              value={pceClusterId}
              onChange={(e) => setPceClusterId(e.target.value)}
              className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
            >
              <option value="">All Clusters</option>
              {clusters.map((c) => (
                <option key={c.id} value={c.id}>{c.name || c.id}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-caption text-muted mb-1">Namespace</label>
            <input
              value={pceNamespace}
              onChange={(e) => setPceNamespace(e.target.value)}
              placeholder="e.g. default"
              className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
            />
          </div>
          <div>
            <label className="block text-caption text-muted mb-1">Severity</label>
            <select
              value={pceSeverityFilter}
              onChange={(e) => setPceSeverityFilter(e.target.value)}
              className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
            >
              <option value="">All</option>
              <option value="critical">Critical</option>
              <option value="high">High</option>
              <option value="medium">Medium</option>
              <option value="low">Low</option>
            </select>
          </div>
          <div>
            <label className="block text-caption text-muted mb-1">Pod name</label>
            <input
              value={pcePodName}
              onChange={(e) => setPcePodName(e.target.value)}
              placeholder="e.g. my-pod"
              className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
            />
          </div>
          <div>
            <label className="block text-caption text-muted mb-1">Capability ID</label>
            <input
              value={pceCapabilityId}
              onChange={(e) => setPceCapabilityId(e.target.value)}
              placeholder="e.g. ESC_PRIV_POD"
              className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
            />
          </div>
          <div className="flex items-end gap-2">
            <Button
              size="sm"
              onClick={async () => {
                setPceListPage(1);
                const merged =
                  (pceClusterId || '').trim() ||
                  effectiveClusterId ||
                  selectedClusterId ||
                  undefined;
                await loadPceDetails({
                  clusterId: merged,
                  namespace: pceNamespace.trim() || undefined,
                  severity: pceSeverityFilter || undefined,
                  podName: pcePodName.trim() || undefined,
                  capabilityId: pceCapabilityId.trim() || undefined,
                  limit: pceListPageSize,
                  offset: 0,
                });
              }}
            >
              Apply &amp; search
            </Button>
            <ResetFiltersButton
              onReset={resetPceFilters}
              active={hasPceFilters}
              title="Clear cluster, namespace, severity, pod, capability and heatmap filters and sort"
            />
          </div>
        </div>
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <label className="text-caption text-muted uppercase tracking-wider">Sort:</label>
          <select
            value={pceSort}
            onChange={(e) => setPceSort(e.target.value as typeof pceSort)}
            className="bg-base border border-border rounded px-3 py-1.5 text-body text-text focus:outline-none focus:border-brand"
          >
            <option value="severity_desc">Severity high to low</option>
            <option value="severity_asc">Severity low to high</option>
            <option value="capability_asc">Capability A-Z</option>
            <option value="pod_asc">Pod name A-Z</option>
            <option value="namespace_asc">Namespace A-Z</option>
          </select>
          <span className="text-caption text-muted">Sort applies to the current page of results.</span>
        </div>
        <div className="ui-table-scroll rounded-lg border border-border bg-surface">
          <table className={UI_TABLE}>
            <thead className={UI_THEAD_STICKY}>
              <tr>
                <th className={UI_TH_COMPACT}>Pod Name</th>
                <th className={`${UI_TH_COMPACT} hidden lg:table-cell`}>Pod UID</th>
                <th className={UI_TH_COMPACT}>Namespace</th>
                <th className={UI_TH_COMPACT}>Capability</th>
                <th className={UI_TH_COMPACT}>Severity</th>
                <th className={`${UI_TH_COMPACT} max-w-[140px]`}>Evidence</th>
                <th className={`${UI_TH_COMPACT} whitespace-nowrap`}>Last Seen</th>
              </tr>
            </thead>
            <tbody>
              {pceDetails.length === 0 && !pceIssue ? (
                <tr>
                  <td colSpan={7} className={`${UI_TD_COMPACT_TIGHT} py-4 text-center text-muted`}>No matching capability records. Adjust filters and run again.</td>
                </tr>
              ) : pceDetails.length > 0 ? (
                sortedPceDetails.map((row) => {
                  const evSum = summarizePceEvidence(row.evidence);
                  const lastSeen = row.lastSeenAt ? (() => {
                    try {
                      const d = new Date(row.lastSeenAt);
                      const now = Date.now();
                      const diffMs = now - d.getTime();
                      if (diffMs < 60000) return 'Just now';
                      if (diffMs < 3600000) return `${Math.floor(diffMs / 60000)}m ago`;
                      if (diffMs < 86400000) return `${Math.floor(diffMs / 3600000)}h ago`;
                      if (diffMs < 604800000) return `${Math.floor(diffMs / 86400000)}d ago`;
                      return d.toLocaleDateString();
                    } catch {
                      return row.lastSeenAt;
                    }
                  })() : (row.updatedAt ? (() => {
                    try {
                      const d = new Date(row.updatedAt);
                      return d.toLocaleDateString();
                    } catch {
                      return '—';
                    }
                  })() : '—');
                  return (
                    <tr key={`${row.podUid}-${row.capabilityId}`} className={UI_TR}>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-text font-medium`} title={row.podUid}>{row.podName ?? '—'}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted font-mono text-caption truncate max-w-[120px] hidden lg:table-cell`} title={row.podUid}>{row.podUid}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-text`}>{row.namespace}</td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-text`}>{row.capabilityId}</td>
                      <td className={UI_TD_COMPACT_TIGHT}>
                        <span className={`px-2 py-0.5 rounded text-caption font-bold uppercase border ${getSeverityBadgeClass(row.severity)}`}>
                          {row.severity}
                        </span>
                      </td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted text-caption max-w-[200px] leading-snug`} title={evSum.title || undefined}>
                        {evSum.label ? <span className="line-clamp-2">{evSum.label}</span> : '—'}
                      </td>
                      <td className={`${UI_TD_COMPACT_TIGHT} text-muted text-caption whitespace-nowrap`} title={row.lastSeenAt || row.updatedAt || undefined}>{lastSeen}</td>
                    </tr>
                  );
                })
              ) : null}
            </tbody>
          </table>
        </div>
        {(pceListTotal > 0 || pceDetails.length > 0) && (
          <Pagination
            page={pceListPage}
            pageSize={pceListPageSize}
            total={pceListTotal}
            onPageChange={setPceListPage}
            onPageSizeChange={(size) => {
              setPceListPageSize(size);
              setPceListPage(1);
            }}
            pageSizeOptions={[10, 25, 50, 100]}
            itemLabel="rows"
            className="rounded-b-lg border border-t-0 border-border"
          />
        )}
      </div>
    </div>
    </PageLayout>
  );
};
