import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { NodeDetailResponse } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { PageLoading, PageEmpty, PageError } from '../design-system/components/PageStatus';
import { ArrowLeft, Server, Box } from 'lucide-react';
import { formatDateTime } from '../lib/display';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH_COMPACT, UI_TR, UI_TD_COMPACT_TIGHT } from '../lib/tableChrome';
import { DataFreshness } from '../components/DataFreshness';

export const NodeDetail: React.FC = () => {
  const { clusterId, nodeName } = useParams<{ clusterId: string; nodeName: string }>();
  const navigate = useNavigate();
  const [node, setNode] = useState<NodeDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);

  const fetchNode = useCallback(async () => {
    if (!clusterId || !nodeName) return;
    setLoading(true);
    try {
      const data = await api.getClusterNode(clusterId, decodeURIComponent(nodeName), { pods: true });
      setNode(data ?? null);
      setError(null);
      setUpdatedAt(new Date());
    } catch {
      setError('Node detail could not be refreshed.');
    } finally {
      setLoading(false);
    }
  }, [clusterId, nodeName]);

  useEffect(() => {
    fetchNode();
  }, [fetchNode]);

  if (loading || !clusterId || !nodeName) {
    return <PageLoading message="Loading node detail..." className="min-h-[40dvh]" />;
  }

  const displayName = decodeURIComponent(nodeName);
  const pods = node?.pods ?? [];
  const podCount = node?.podCount ?? 0;

  return (
    <PageLayout
      title={displayName}
      description={`Node in cluster · ${podCount} pod(s)`}
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <DataFreshness updatedAt={updatedAt} loading={loading} error={error} />
          <Button variant="secondary" onClick={fetchNode} isLoading={loading}>Refresh</Button>
          <Button variant="secondary" onClick={() => navigate(`/clusters/${clusterId}`)}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Cluster
          </Button>
        </div>
      }
    >
      {error && !node ? (
        <PageError
          title="Could not load node"
          description="The node detail request failed. Retry or return to the cluster inventory."
          action={<Button variant="secondary" onClick={fetchNode} isLoading={loading}>Retry node</Button>}
        />
      ) : null}
      {!error && !node ? (
        <PageEmpty title="Node not found" description="The node may no longer be reported by the cluster." className="py-10" />
      ) : null}
      {node ? (
      <>
      {node && (node.role || node.os || node.runtime || node.ip || node.kubeletVersion) && (
        <Card className="p-6 mb-6">
          <h3 className="text-section-title text-text mb-4 flex items-center gap-2">
            <Server className="w-5 h-5 text-brand" /> Node metadata
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 text-body">
            {node.role && (
              <div>
                <span className="text-muted">Role</span>
                <p className="text-text font-medium">{node.role}</p>
              </div>
            )}
            {node.os && (
              <div>
                <span className="text-muted">OS</span>
                <p className="text-text font-medium">{node.os}</p>
              </div>
            )}
            {node.runtime && (
              <div>
                <span className="text-muted">Runtime</span>
                <p className="text-text font-medium">{node.runtime}</p>
              </div>
            )}
            {node.ip && (
              <div>
                <span className="text-muted">IP</span>
                <p className="text-text font-medium">{node.ip}</p>
              </div>
            )}
            {node.kubeletVersion && (
              <div>
                <span className="text-muted">Kubelet</span>
                <p className="text-text font-medium">{node.kubeletVersion}</p>
              </div>
            )}
            {node.lastSeen && (
              <div>
                <span className="text-muted">Last Seen</span>
                <p className="text-text font-medium">{formatDateTime(node.lastSeen)}</p>
              </div>
            )}
          </div>
        </Card>
      )}
      <Card className="p-6">
        <h3 className="text-section-title text-text mb-4 flex items-center gap-2">
          <Box className="w-5 h-5 text-brand" /> Workloads on this node
        </h3>
        {pods.length > 0 ? (
          <div className="ui-table-scroll rounded-lg border border-border">
            <table className={UI_TABLE}>
              <thead className={UI_THEAD_STICKY}>
                <tr>
                  <th className={UI_TH_COMPACT}>Pod</th>
                  <th className={UI_TH_COMPACT}>Namespace</th>
                  <th className={UI_TH_COMPACT}>Risk Count</th>
                  <th className={`${UI_TH_COMPACT} text-right`}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {pods.map((pod) => (
                  <tr
                    key={pod.uid}
                    role="link"
                    tabIndex={0}
                    className={`${UI_TR} cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-inset`}
                    onClick={() => navigate(`/resources/pods/uid/${encodeURIComponent(pod.uid)}`)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        navigate(`/resources/pods/uid/${encodeURIComponent(pod.uid)}`);
                      }
                    }}
                  >
                    <td className={`${UI_TD_COMPACT_TIGHT} font-medium text-text`}>{pod.name}</td>
                    <td className={`${UI_TD_COMPACT_TIGHT} text-muted`}>{pod.namespace}</td>
                    <td className={UI_TD_COMPACT_TIGHT}>
                      <span className={pod.riskCount > 0 ? 'text-amber-400 font-medium' : 'text-muted'}>{pod.riskCount}</span>
                    </td>
                    <td className={`${UI_TD_COMPACT_TIGHT} text-right`} onClick={(e) => e.stopPropagation()}>
                      <Button size="sm" variant="secondary" onClick={() => navigate(`/resources/pods/uid/${encodeURIComponent(pod.uid)}`)}>
                        View
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : <PageEmpty title="No workloads on this node" description="No pods are currently associated with this node." className="py-8" />}
      </Card>
      </>
      ) : null}
    </PageLayout>
  );
};
