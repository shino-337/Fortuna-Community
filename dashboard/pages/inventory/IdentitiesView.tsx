import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Search } from 'lucide-react';
import { Table, type TableColumn } from '../../design-system/components/Table';
import { Pagination } from '../../components/Pagination';
import { AvailabilityNotice } from '../../components/AvailabilityNotice';
import { api, getAvailabilityIssue, type AvailabilityIssue } from '../../lib/api';
import { identityDetailPath } from '../../lib/entityLinks';
import type { K8sResource } from '../../types';
import { CountChip } from './shared';

export const IDENTITY_KINDS = ['ServiceAccount', 'Role', 'ClusterRole', 'RoleBinding', 'ClusterRoleBinding'] as const;
export type IdentityKind = (typeof IDENTITY_KINDS)[number];

/** Kind filter groups: bindings of both scopes are one group, since they answer the same question. */
export const KIND_GROUPS: { id: string; label: string; kinds: IdentityKind[] }[] = [
  { id: '', label: 'All', kinds: [...IDENTITY_KINDS] },
  { id: 'ServiceAccount', label: 'Service accounts', kinds: ['ServiceAccount'] },
  { id: 'Role', label: 'Roles', kinds: ['Role'] },
  { id: 'ClusterRole', label: 'Cluster roles', kinds: ['ClusterRole'] },
  { id: 'Binding', label: 'Bindings', kinds: ['RoleBinding', 'ClusterRoleBinding'] },
];

export const KIND_LABEL: Record<string, string> = {
  ServiceAccount: 'Service account',
  Role: 'Role',
  ClusterRole: 'Cluster role',
  RoleBinding: 'Role binding',
  ClusterRoleBinding: 'Cluster role binding',
};

export interface IdentityFilters {
  search: string;
  namespace: string;
  kind: string;
  page: number;
  pageSize: number;
}

const clusterWide = (kind: string) => kind === 'ClusterRole' || kind === 'ClusterRoleBinding';

/** Service accounts, roles and bindings in one list; the kind is a filter, not a tab. */
export const IdentitiesView: React.FC<{
  clusterId: string | null;
  filters: IdentityFilters;
  setFilters: (patch: Partial<IdentityFilters>) => void;
  namespaces: string[];
  selectedUid: string | null;
  onSelect: (resource: K8sResource) => void;
  refreshKey: number;
}> = ({ clusterId, filters, setFilters, namespaces, selectedUid, onSelect, refreshKey }) => {
  const [all, setAll] = useState<K8sResource[]>([]);
  const [loading, setLoading] = useState(true);
  const [issue, setIssue] = useState<AvailabilityIssue | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    Promise.all(IDENTITY_KINDS.map((k) => api.getResources(k, { cluster: clusterId ?? undefined, namespace: clusterWide(k) ? undefined : filters.namespace || undefined })))
      .then((lists) => {
        if (cancelled) return;
        setAll(lists.flat());
        setIssue(null);
      })
      .catch((e) => !cancelled && setIssue(getAvailabilityIssue(e, 'Identities')))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [clusterId, filters.namespace, refreshKey]);

  const counts = useMemo(() => {
    const out: Record<string, number> = {};
    for (const g of KIND_GROUPS) out[g.id] = all.filter((r) => g.kinds.includes(r.kind as IdentityKind)).length;
    return out;
  }, [all]);

  const rows = useMemo(() => {
    const kinds = (KIND_GROUPS.find((g) => g.id === filters.kind) ?? KIND_GROUPS[0]).kinds as string[];
    const q = filters.search.trim().toLowerCase();
    return all
      .filter((r) => kinds.includes(r.kind))
      .filter((r) => !q || r.name.toLowerCase().includes(q) || r.namespace.toLowerCase().includes(q))
      .sort((a, b) => IDENTITY_KINDS.indexOf(a.kind as IdentityKind) - IDENTITY_KINDS.indexOf(b.kind as IdentityKind) || a.name.localeCompare(b.name));
  }, [all, filters.kind, filters.search]);

  const pageRows = rows.slice((filters.page - 1) * filters.pageSize, filters.page * filters.pageSize);

  const columns = useMemo<TableColumn<K8sResource>[]>(() => {
    const cols: TableColumn<K8sResource>[] = [
      {
        key: 'name',
        header: 'Name',
        cell: (r) =>
          r.kind === 'ServiceAccount' ? (
            <Link to={identityDetailPath({ uid: r.id, clusterId: r.clusterId })} className="font-medium text-text hover:text-brand hover:underline">
              {r.name}
            </Link>
          ) : (
            <span className="font-medium text-text">{r.name}</span>
          ),
      },
      { key: 'kind', header: 'Kind', cell: (r) => <span className="rounded border border-border px-1.5 py-0.5 text-meta text-muted">{KIND_LABEL[r.kind] ?? r.kind}</span> },
      {
        key: 'namespace',
        header: 'Namespace',
        cell: (r) => <span className="text-caption text-muted">{clusterWide(r.kind) ? 'cluster-wide' : r.namespace}</span>,
      },
    ];
    if (!clusterId) cols.push({ key: 'cluster', header: 'Cluster', cell: (r) => <span className="text-caption text-muted">{r.clusterId}</span> });
    return cols;
  }, [clusterId]);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[14rem] flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden />
          <input
            value={filters.search}
            onChange={(e) => setFilters({ search: e.target.value, page: 1 })}
            placeholder="Search name or namespace"
            aria-label="Search identities"
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
      </div>
      <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Kind">
        {KIND_GROUPS.map((g) => (
          <CountChip key={g.id || 'all'} label={g.label} count={loading ? undefined : counts[g.id]} active={filters.kind === g.id} onClick={() => setFilters({ kind: g.id, page: 1 })} />
        ))}
      </div>

      {issue ? <AvailabilityNotice issue={issue} /> : null}

      <Table
        columns={columns}
        data={pageRows}
        loading={loading && all.length === 0}
        rowKey={(r) => `${r.clusterId}/${r.kind}/${r.id}`}
        onRowClick={onSelect}
        isRowSelected={(r) => r.id === selectedUid}
        scrollClassName="overflow-x-auto"
        emptyTitle={filters.search || filters.kind ? 'Nothing matches these filters' : 'No identities yet'}
        emptyDescription={filters.search || filters.kind ? 'Clear a filter to see more.' : 'Service accounts and RBAC objects appear after the first inventory scan.'}
      />
      {rows.length > 0 ? (
        <Pagination
          page={filters.page}
          pageSize={filters.pageSize}
          total={rows.length}
          onPageChange={(page) => setFilters({ page })}
          onPageSizeChange={(pageSize) => setFilters({ pageSize, page: 1 })}
          pageSizeOptions={[20, 50, 100]}
          itemLabel="identities"
        />
      ) : null}
    </div>
  );
};
