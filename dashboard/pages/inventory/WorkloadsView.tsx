import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Search } from 'lucide-react';
import { Table, type TableColumn } from '../../design-system/components/Table';
import { Pagination } from '../../components/Pagination';
import { AvailabilityNotice } from '../../components/AvailabilityNotice';
import { api, getAvailabilityIssue, type AvailabilityIssue, type PodLevelCounts } from '../../lib/api';
import { attackPathsForPodPath, findingsForResourcePath, identityDetailPath, podDetailPath } from '../../lib/entityLinks';
import { getPodStatusBadgeClass } from '../../lib/severity';
import type { PodWithRisk } from '../../types';
import { CountChip, LEVEL_LABEL, LEVEL_TONE, LEVELS, RiskBadge, type Level } from './shared';

export interface WorkloadFilters {
  search: string;
  namespace: string;
  node: string;
  level: Level | 'unscored' | '';
  page: number;
  pageSize: number;
}

/** Workloads ranked by risk, each row leading to its findings and attack paths. */
export const WorkloadsView: React.FC<{
  clusterId: string | null;
  filters: WorkloadFilters;
  setFilters: (patch: Partial<WorkloadFilters>) => void;
  namespaces: string[];
  selectedUid: string | null;
  onSelect: (pod: PodWithRisk) => void;
  refreshKey: number;
  allowedRoutes: string[];
}> = ({ clusterId, filters, setFilters, namespaces, selectedUid, onSelect, refreshKey, allowedRoutes }) => {
  const [rows, setRows] = useState<PodWithRisk[]>([]);
  const [total, setTotal] = useState(0);
  const [levelCounts, setLevelCounts] = useState<PodLevelCounts | null>(null);
  const [loading, setLoading] = useState(true);
  const [issue, setIssue] = useState<AvailabilityIssue | null>(null);
  const [searchDraft, setSearchDraft] = useState(filters.search);

  useEffect(() => setSearchDraft(filters.search), [filters.search]);
  useEffect(() => {
    if (searchDraft === filters.search) return;
    const t = window.setTimeout(() => setFilters({ search: searchDraft, page: 1 }), 300);
    return () => window.clearTimeout(t);
  }, [searchDraft, filters.search, setFilters]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .getPods({
        cluster: clusterId ?? undefined,
        namespace: filters.namespace || undefined,
        node: filters.node || undefined,
        search: filters.search || undefined,
        level: filters.level || undefined,
        sortBy: 'risk_desc',
        page: filters.page,
        pageSize: filters.pageSize,
        withLevelCounts: true,
      })
      .then((r) => {
        if (cancelled) return;
        setRows(r.pods);
        setTotal(r.total);
        setLevelCounts(r.levelCounts ?? null);
        setIssue(null);
      })
      .catch((e) => {
        if (cancelled) return;
        setIssue(getAvailabilityIssue(e, 'Workloads'));
      })
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [clusterId, filters.namespace, filters.node, filters.search, filters.level, filters.page, filters.pageSize, refreshKey]);

  const canFindings = allowedRoutes.includes('/risks');
  const canPaths = allowedRoutes.includes('/attack-paths');

  const columns = useMemo<TableColumn<PodWithRisk>[]>(() => {
    const cols: TableColumn<PodWithRisk>[] = [
      {
        key: 'name',
        header: 'Workload',
        cell: (p) => (
          <div className="min-w-0">
            <Link to={podDetailPath(p.uid, p.clusterId)} className="font-medium text-text hover:text-brand hover:underline">
              {p.name}
            </Link>
            <div className="text-caption text-muted">{p.namespace}</div>
          </div>
        ),
      },
    ];
    if (!clusterId) cols.push({ key: 'cluster', header: 'Cluster', cell: (p) => <span className="text-caption text-muted">{p.clusterId}</span> });
    cols.push(
      { key: 'risk', header: 'Risk', cell: (p) => <RiskBadge level={p.finalLevel} score={p.unifiedScore ?? p.totalScore} /> },
      {
        key: 'findings',
        header: 'Findings',
        cell: (p) =>
          p.riskCount > 0 && canFindings ? (
            <Link to={findingsForResourcePath({ uid: p.uid, clusterId: p.clusterId })} className="tabular-nums text-brand hover:underline">
              {p.riskCount.toLocaleString()}
            </Link>
          ) : (
            <span className="tabular-nums text-muted">{p.riskCount.toLocaleString()}</span>
          ),
      },
      {
        key: 'paths',
        header: 'Attack paths',
        cell: (p) => {
          const n = p.riskSignals?.pathCount ?? 0;
          const role = p.riskSignals?.isEntryPoint ? 'entry' : p.riskSignals?.isPivot ? 'pivot' : '';
          if (!p.riskSignals) return <span className="text-muted">—</span>;
          return (
            <span className="inline-flex items-center gap-1.5">
              {n > 0 && canPaths ? (
                <Link to={attackPathsForPodPath({ uid: p.uid, clusterId: p.clusterId })} className="tabular-nums text-brand hover:underline">
                  {n.toLocaleString()}
                </Link>
              ) : (
                <span className="tabular-nums text-muted">{n.toLocaleString()}</span>
              )}
              {role ? <span className="rounded border border-border px-1 text-meta text-muted">{role}</span> : null}
            </span>
          );
        },
      },
      {
        key: 'identity',
        header: 'Runs as',
        className: 'hidden lg:table-cell',
        headerClassName: 'hidden lg:table-cell',
        cell: (p) =>
          p.serviceAccount ? (
            p.serviceAccountUid ? (
              <Link to={identityDetailPath({ uid: p.serviceAccountUid, clusterId: p.clusterId })} className="text-caption text-text hover:text-brand hover:underline">
                {p.serviceAccount}
              </Link>
            ) : (
              <span className="text-caption text-muted">{p.serviceAccount}</span>
            )
          ) : (
            <span className="text-muted">—</span>
          ),
      },
      {
        key: 'status',
        header: 'Status',
        className: 'hidden md:table-cell',
        headerClassName: 'hidden md:table-cell',
        cell: (p) => <span className={`rounded px-1.5 py-0.5 text-meta ${getPodStatusBadgeClass(p.phase || p.status)}`}>{p.phase || p.status || '—'}</span>,
      },
    );
    return cols;
  }, [clusterId, canFindings, canPaths]);

  const filtered = Boolean(filters.search || filters.namespace || filters.node || filters.level);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[14rem] flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden />
          <input
            value={searchDraft}
            onChange={(e) => setSearchDraft(e.target.value)}
            placeholder="Search name, namespace, node or UID"
            aria-label="Search workloads"
            className="min-h-9 w-full rounded-lg border border-border bg-base pl-9 pr-3 text-body text-text outline-none focus:border-brand"
          />
        </div>
        <select
          value={filters.namespace}
          onChange={(e) => setFilters({ namespace: e.target.value, page: 1 })}
          disabled={!clusterId}
          title={clusterId ? undefined : 'Pick a cluster in the header to filter by namespace'}
          aria-label="Namespace"
          className="min-h-9 rounded-lg border border-border bg-base px-3 text-body text-text disabled:opacity-50"
        >
          <option value="">All namespaces</option>
          {namespaces.map((ns) => (
            <option key={ns} value={ns}>
              {ns}
            </option>
          ))}
        </select>
        {filters.node ? (
          <CountChip label={`Node ${filters.node} ✕`} active onClick={() => setFilters({ node: '', page: 1 })} />
        ) : null}
      </div>

      <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Risk level">
        {[...LEVELS, 'unscored' as const].map((lv) => (
          <CountChip
            key={lv}
            label={LEVEL_LABEL[lv]}
            tone={LEVEL_TONE[lv]}
            count={levelCounts ? levelCounts[lv] : undefined}
            active={filters.level === lv}
            onClick={() => setFilters({ level: filters.level === lv ? '' : lv, page: 1 })}
          />
        ))}
        {filtered ? (
          <button
            type="button"
            className="ml-auto text-caption font-semibold text-muted hover:text-text"
            onClick={() => {
              setSearchDraft('');
              setFilters({ search: '', namespace: '', node: '', level: '', page: 1 });
            }}
          >
            Reset filters
          </button>
        ) : null}
      </div>

      {issue ? <AvailabilityNotice issue={issue} /> : null}

      <Table
        columns={columns}
        data={rows}
        loading={loading && rows.length === 0}
        rowKey={(p) => `${p.clusterId}/${p.uid}`}
        onRowClick={onSelect}
        isRowSelected={(p) => p.uid === selectedUid}
        scrollClassName="overflow-x-auto"
        emptyTitle={filtered ? 'No workloads match these filters' : 'No workloads yet'}
        emptyDescription={filtered ? 'Clear a filter to see more.' : 'Workloads appear after the agent finishes its first scan.'}
      />
      {total > 0 ? (
        <Pagination
          page={filters.page}
          pageSize={filters.pageSize}
          total={total}
          onPageChange={(page) => setFilters({ page })}
          onPageSizeChange={(pageSize) => setFilters({ pageSize, page: 1 })}
          pageSizeOptions={[20, 50, 100]}
          itemLabel="workloads"
        />
      ) : null}
    </div>
  );
};
