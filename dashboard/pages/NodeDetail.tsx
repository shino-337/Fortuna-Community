import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { NodeDetailResponse } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { PageLoading } from '../components/PageLoading';
import { PageEmpty } from '../components/PageEmpty';
import { ArrowLeft, Server, Box } from 'lucide-react';
import { formatDateTime } from '../lib/display';

export const NodeDetail: React.FC = () => {
  const { clusterId, nodeName } = useParams<{ clusterId: string; nodeName: string }>();
  const navigate = useNavigate();
  const [node, setNode] = useState<NodeDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchNode = useCallback(async () => {
    if (!clusterId || !nodeName) return;
    setLoading(true);
    const data = await api.getClusterNode(clusterId, decodeURIComponent(nodeName), { pods: true });
    setNode(data ?? null);
    setLoading(false);
  }, [clusterId, nodeName]);

  useEffect(() => {
    fetchNode();
  }, [fetchNode]);

  if (loading || !clusterId || !nodeName) {
    return <PageLoading message="Loading node detail..." className="min-h-[40vh]" />;
  }

  const displayName = decodeURIComponent(nodeName);
  const pods = node?.pods ?? [];
  const podCount = node?.podCount ?? 0;

  return (
    <PageLayout
      title={displayName}
      description={`Node in cluster · ${podCount} pod(s)`}
      actions={
        <Button variant="secondary" onClick={() => navigate(`/clusters/${clusterId}`)}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Cluster
        </Button>
      }
    >
      {node && (node.role || node.os || node.runtime || node.ip || node.kubeletVersion) && (
        <Card className="p-6 mb-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <Server className="w-5 h-5 text-pink-500" /> Node metadata
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 text-sm">
            {node.role && (
              <div>
                <span className="text-slate-500">Role</span>
                <p className="text-white font-medium">{node.role}</p>
              </div>
            )}
            {node.os && (
              <div>
                <span className="text-slate-500">OS</span>
                <p className="text-white font-medium">{node.os}</p>
              </div>
            )}
            {node.runtime && (
              <div>
                <span className="text-slate-500">Runtime</span>
                <p className="text-white font-medium">{node.runtime}</p>
              </div>
            )}
            {node.ip && (
              <div>
                <span className="text-slate-500">IP</span>
                <p className="text-white font-medium">{node.ip}</p>
              </div>
            )}
            {node.kubeletVersion && (
              <div>
                <span className="text-slate-500">Kubelet</span>
                <p className="text-white font-medium">{node.kubeletVersion}</p>
              </div>
            )}
            {node.lastSeen && (
              <div>
                <span className="text-slate-500">Last Seen</span>
                <p className="text-white font-medium">{formatDateTime(node.lastSeen)}</p>
              </div>
            )}
          </div>
        </Card>
      )}
      <Card className="p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Box className="w-5 h-5 text-pink-500" /> Workloads on this node
        </h3>
        {pods.length > 0 ? (
          <div className="overflow-x-auto max-h-[60vh] overflow-y-auto rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
                <tr>
                  <th className="text-left py-2">Pod</th>
                  <th className="text-left py-2">Namespace</th>
                  <th className="text-left py-2">Risk Count</th>
                  <th className="text-right py-2">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {pods.map((pod) => (
                  <tr key={pod.uid} className="hover:bg-muted/30 cursor-pointer" onClick={() => navigate(`/resources/pods/uid/${encodeURIComponent(pod.uid)}`)}>
                    <td className="py-2 font-medium text-text">{pod.name}</td>
                    <td className="py-2 text-muted">{pod.namespace}</td>
                    <td className="py-2">
                      <span className={pod.riskCount > 0 ? 'text-amber-400 font-medium' : 'text-muted'}>{pod.riskCount}</span>
                    </td>
                    <td className="py-2 text-right" onClick={(e) => e.stopPropagation()}>
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
    </PageLayout>
  );
};
