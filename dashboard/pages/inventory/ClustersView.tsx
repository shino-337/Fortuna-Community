import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Table, type TableColumn } from '../../design-system/components/Table';
import { AvailabilityNotice } from '../../components/AvailabilityNotice';
import { api, getAvailabilityIssue, type AvailabilityIssue } from '../../lib/api';
import { getClusterDisplayName } from '../../lib/clusterDisplay';
import type { Cluster } from '../../types';

const CONNECTION_CLASS: Record<string, string> = {
  connected: 'text-emerald-300',
  degraded: 'text-amber-300',
  disconnected: 'text-red-300',
};

/** Every cluster this account can see; choosing one scopes the whole app to it. */
export const ClustersView: React.FC<{
  selectedClusterId: string | null;
  onOpenCluster: (id: string) => void;
  refreshKey: number;
  canFindings: boolean;
}> = ({ selectedClusterId, onOpenCluster, refreshKey, canFindings }) => {
  const [rows, setRows] = useState<Cluster[]>([]);
  const [loading, setLoading] = useState(true);
  const [issue, setIssue] = useState<AvailabilityIssue | null>(null);
  const [retryKey, setRetryKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .getClustersStats()
      .then((r) => {
        if (cancelled) return;
        setRows(r);
        setIssue(null);
      })
      // Rows from the last good load stay on screen; the notice says why they may be stale.
      .catch((e) => !cancelled && setIssue(getAvailabilityIssue(e, 'Cluster inventory')))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [refreshKey, retryKey]);

  const columns = useMemo<TableColumn<Cluster>[]>(
    () => [
      {
        key: 'name',
        header: 'Cluster',
        cell: (c) => (
          <div>
            <div className="font-medium text-text">{getClusterDisplayName(c)}</div>
            {c.distribution || c.k8sVersion || c.version ? (
              <div className="text-caption text-muted">{[c.distribution, c.k8sVersion ?? c.version].filter(Boolean).join(' · ')}</div>
            ) : null}
          </div>
        ),
      },
      {
        key: 'connection',
        header: 'Connection',
        cell: (c) => <span className={`text-caption capitalize ${CONNECTION_CLASS[c.connectionStatus ?? ''] ?? 'text-muted'}`}>{c.connectionStatus ?? 'unknown'}</span>,
      },
      { key: 'agents', header: 'Agents', cell: (c) => <span className="tabular-nums">{(c.agentCount ?? 0).toLocaleString()}</span> },
      { key: 'pods', header: 'Pods', cell: (c) => <span className="tabular-nums">{(c.podCount ?? 0).toLocaleString()}</span> },
      {
        key: 'findings',
        header: 'Open findings',
        cell: (c) =>
          canFindings && (c.riskCount ?? 0) > 0 ? (
            <Link to={`/risks/findings?clusterId=${encodeURIComponent(c.id)}`} className="tabular-nums text-brand hover:underline">
              {(c.riskCount ?? 0).toLocaleString()}
            </Link>
          ) : (
            <span className="tabular-nums text-muted">{(c.riskCount ?? 0).toLocaleString()}</span>
          ),
      },
      {
        key: 'details',
        header: <span className="sr-only">Details</span>,
        cell: (c) => (
          <Link to={`/clusters/${encodeURIComponent(c.id)}`} className="text-caption font-semibold text-brand hover:underline">
            Details
          </Link>
        ),
      },
    ],
    [canFindings],
  );

  return (
    <div className="flex flex-col gap-3">
      <p className="text-caption text-muted">Choose a cluster to see its workloads. The header keeps it selected on every page.</p>
      {issue ? <AvailabilityNotice issue={issue} onRetry={issue.retryable ? () => setRetryKey((k) => k + 1) : undefined} /> : null}
      {issue && rows.length === 0 ? null : (
      <Table
        columns={columns}
        data={rows}
        loading={loading && rows.length === 0}
        rowKey={(c) => c.id}
        onRowClick={(c) => onOpenCluster(c.id)}
        isRowSelected={(c) => c.id === selectedClusterId}
        scrollClassName="overflow-x-auto"
        emptyTitle="No clusters registered"
        emptyDescription="A cluster appears when its agent first reports."
      />
      )}
    </div>
  );
};
