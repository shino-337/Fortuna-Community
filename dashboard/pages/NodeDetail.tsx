import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { NodeDetailResponse } from '../types';
import { PageLayout } from '../components/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, Server, Box } from 'lucide-react';

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
    return (
      <div className="flex flex-col justify-center items-center h-[40vh]">
        <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-slate-500 mt-4">Loading node...</span>
      </div>
    );
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
          </div>
        </Card>
      )}
      <Card className="p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Box className="w-5 h-5 text-pink-500" /> Workloads on this node
        </h3>
        {pods.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="text-slate-400 border-b border-slate-800">
                <tr>
                  <th className="text-left py-2">Pod</th>
                  <th className="text-left py-2">Namespace</th>
                  <th className="text-left py-2">Risk Count</th>
                  <th className="text-right py-2">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {pods.map((pod) => (
                  <tr key={pod.uid} className="hover:bg-slate-800/50 cursor-pointer" onClick={() => navigate(`/resources/pods/${pod.id}`)}>
                    <td className="py-2 font-medium text-white">{pod.name}</td>
                    <td className="py-2 text-slate-400">{pod.namespace}</td>
                    <td className="py-2">
                      <span className={pod.riskCount > 0 ? 'text-amber-400 font-medium' : 'text-slate-500'}>{pod.riskCount}</span>
                    </td>
                    <td className="py-2 text-right" onClick={(e) => e.stopPropagation()}>
                      <Button size="sm" variant="secondary" onClick={() => navigate(`/resources/pods/${pod.id}`)}>
                        View
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-slate-500 text-sm">No pods on this node.</p>
        )}
      </Card>
    </PageLayout>
  );
};
