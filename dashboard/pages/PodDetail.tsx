import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { PodWithRisk, PodSbom, Insight } from '../types';
import { PageLayout } from '../components/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, Box, Package, ShieldAlert } from 'lucide-react';
import clsx from 'clsx';
import { getSeverityBadgeClass } from '../lib/severity';

type TabId = 'overview' | 'sbom' | 'risks';

export const PodDetail: React.FC = () => {
  const { id, uid } = useParams<{ id?: string; uid?: string }>();
  const navigate = useNavigate();
  const [pod, setPod] = useState<PodWithRisk | null>(null);
  const [sbom, setSbom] = useState<PodSbom | null>(null);
  const [relatedRisks, setRelatedRisks] = useState<Insight[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabId>('overview');
  const [tabLoading, setTabLoading] = useState(false);

  const idOrUid = uid ?? id;

  const fetchPod = useCallback(async () => {
    if (!idOrUid) return;
    setLoading(true);
    const data = uid ? await api.getPodByUid(idOrUid) : await api.getPod(idOrUid!);
    setPod(data);
    setLoading(false);
  }, [idOrUid, uid]);

  const fetchTabData = useCallback(
    async (tab: TabId) => {
      if (!pod) return;
      setTabLoading(true);
      try {
        if (tab === 'sbom') {
          const data = await api.getPodSbom(pod.uid);
          setSbom(data ?? null);
        } else if (tab === 'risks') {
          const { insights } = await api.getPodRiskReport(pod.uid);
          setRelatedRisks(insights);
        }
      } finally {
        setTabLoading(false);
      }
    },
    [pod]
  );

  useEffect(() => {
    fetchPod();
  }, [fetchPod]);

  useEffect(() => {
    if (pod && activeTab !== 'overview') fetchTabData(activeTab);
  }, [pod, activeTab, fetchTabData]);

  if (loading || !idOrUid) {
    return (
      <div className="flex flex-col justify-center items-center h-[40vh]">
        <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-slate-500 mt-4">Loading pod...</span>
      </div>
    );
  }

  if (!pod) {
    return (
      <PageLayout title="Pod not found" description="The pod may have been removed or you lack access.">
        <Button variant="secondary" onClick={() => navigate('/resources')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Resources
        </Button>
      </PageLayout>
    );
  }

  const tabs: { id: TabId; label: string; icon: React.ReactNode }[] = [
    { id: 'overview', label: 'Overview', icon: <Box className="w-4 h-4" /> },
    { id: 'sbom', label: 'SBOM', icon: <Package className="w-4 h-4" /> },
    { id: 'risks', label: 'Related Risks', icon: <ShieldAlert className="w-4 h-4" /> },
  ];

  return (
    <PageLayout
      title={pod.name}
      description={`${pod.namespace} · ${pod.nodeName ?? '—'}`}
      actions={
        <Button variant="secondary" onClick={() => navigate('/resources')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Resources
        </Button>
      }
    >
      <div className="flex gap-1 p-1 bg-slate-900/80 rounded-lg border border-slate-800 w-fit mb-6">
        {tabs.map(({ id: tabId, label, icon }) => (
          <button
            key={tabId}
            onClick={() => setActiveTab(tabId)}
            className={clsx(
              'flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors',
              activeTab === tabId ? 'bg-pink-600 text-white shadow' : 'text-slate-400 hover:bg-slate-800 hover:text-white'
            )}
          >
            {icon}
            {label}
          </button>
        ))}
      </div>

      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <Card className="p-6">
            <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
              <Box className="w-5 h-5 text-pink-500" /> Overview
            </h3>
            <dl className="grid grid-cols-1 gap-3 text-sm">
              <div>
                <dt className="text-slate-500">Namespace</dt>
                <dd className="text-white font-mono">{pod.namespace}</dd>
              </div>
              <div>
                <dt className="text-slate-500">Node</dt>
                <dd className="text-slate-300 font-mono">{pod.nodeName ?? '—'}</dd>
              </div>
              <div>
                <dt className="text-slate-500">Service Account</dt>
                <dd className="text-slate-300 font-mono">{pod.serviceAccount ?? '—'}</dd>
              </div>
              <div>
                <dt className="text-slate-500">UID</dt>
                <dd className="text-slate-400 font-mono text-xs break-all">{pod.uid}</dd>
              </div>
              <div>
                <dt className="text-slate-500">Risk Count</dt>
                <dd className={pod.riskCount > 0 ? 'text-amber-400 font-medium' : 'text-slate-400'}>{pod.riskCount}</dd>
              </div>
            </dl>
          </Card>
        </div>
      )}

      {activeTab === 'sbom' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4">SBOM</h3>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : sbom ? (
            <div className="space-y-4">
              <p className="text-slate-400 text-sm">
                {sbom.podName} · {sbom.namespace} · {sbom.components?.length ?? 0} packages
              </p>
              <div className="overflow-x-auto max-h-[400px] overflow-y-auto">
                <table className="w-full text-sm">
                  <thead className="text-slate-400 border-b border-slate-800 sticky top-0 bg-slate-900">
                    <tr>
                      <th className="text-left py-2">Package</th>
                      <th className="text-left py-2">Version</th>
                      <th className="text-left py-2">Type</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {(sbom.components || []).slice(0, 100).map((c: any, i: number) => (
                      <tr key={i}>
                        <td className="py-2 font-mono text-white">{c.name}</td>
                        <td className="py-2 text-slate-400 font-mono">{c.version ?? '—'}</td>
                        <td className="py-2 text-slate-500">{c.type ?? '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {(sbom.components?.length ?? 0) > 100 && (
                <p className="text-slate-500 text-xs">Showing first 100 of {sbom.components?.length} packages.</p>
              )}
            </div>
          ) : (
            <p className="text-slate-500 text-sm">No SBOM data for this pod.</p>
          )}
        </Card>
      )}

      {activeTab === 'risks' && (
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4">Related Risks</h3>
          {tabLoading ? (
            <p className="text-slate-500 text-sm">Loading...</p>
          ) : relatedRisks.length > 0 ? (
            <div className="space-y-2">
              {relatedRisks.map((risk) => (
                <div
                  key={risk.id}
                  className="p-3 rounded-lg border border-slate-800 bg-slate-900/50 hover:border-pink-500/30 cursor-pointer"
                  onClick={() => navigate(`/risks/${risk.id}`)}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-white">{risk.title}</span>
                    <span className={clsx('px-2 py-0.5 rounded text-xs font-medium', getSeverityBadgeClass(risk.severity))}>
                      {risk.severity}
                    </span>
                  </div>
                  {risk.description && <p className="text-slate-500 text-sm mt-1 line-clamp-2">{risk.description}</p>}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-slate-500 text-sm">No related risks for this pod.</p>
          )}
        </Card>
      )}

      <Card className="p-6 mt-6">
        <h3 className="text-lg font-semibold text-white mb-2">Related</h3>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => navigate('/risks')}>
            View all risks
          </Button>
          <Button variant="secondary" size="sm" onClick={() => navigate('/capabilities')}>
            Capabilities
          </Button>
        </div>
      </Card>
    </PageLayout>
  );
};
