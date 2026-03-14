import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, UserCog, Key } from 'lucide-react';

export const IdentityDetail: React.FC = () => {
  const { id, uid } = useParams<{ id?: string; uid?: string }>();
  const navigate = useNavigate();
  const [sa, setSa] = useState<Record<string, unknown> | null>(null);
  const [permissions, setPermissions] = useState<Array<Record<string, unknown>>>([]);
  const [loading, setLoading] = useState(true);

  const idOrUid = uid ?? id;

  const fetchData = useCallback(async () => {
    if (!idOrUid) return;
    setLoading(true);
    const saData = uid
      ? await api.getServiceAccountByUid(idOrUid)
      : await api.getServiceAccount(idOrUid);
    setSa(saData ?? null);
    if (saData) {
      const uidForPermissions = (saData as any).uid ?? (saData as any).id;
      if (uidForPermissions != null) {
        const permData = await api.getServiceAccountPermissions(String(uidForPermissions));
        setPermissions(permData.permissions ?? []);
      } else {
        setPermissions([]);
      }
    } else {
      setPermissions([]);
    }
    setLoading(false);
  }, [idOrUid, uid]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  if (loading || !idOrUid) {
    return (
      <div className="flex flex-col justify-center items-center h-[40vh]">
        <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-slate-500 mt-4">Loading identity...</span>
      </div>
    );
  }

  if (!sa) {
    return (
      <PageLayout title="Identity not found" description="The service account may have been removed.">
        <Button variant="secondary" onClick={() => navigate('/resources?tab=ServiceAccount')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Identities
        </Button>
      </PageLayout>
    );
  }

  const name = String(sa.name ?? sa.id ?? idOrUid);
  const namespace = sa.namespace != null ? String(sa.namespace) : '—';

  return (
    <PageLayout
      title={name}
      description={`Service Account · ${namespace}`}
      actions={
        <Button variant="secondary" onClick={() => navigate('/resources?tab=ServiceAccount')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Identities
        </Button>
      }
    >
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <UserCog className="w-5 h-5 text-pink-500" /> Overview
          </h3>
          <dl className="grid grid-cols-1 gap-3 text-sm">
            <div>
              <dt className="text-slate-500">Namespace</dt>
              <dd className="text-white font-mono">{namespace}</dd>
            </div>
            <div>
              <dt className="text-slate-500">Cluster</dt>
              <dd className="text-slate-300 font-mono">{sa.clusterId != null ? String(sa.clusterId) : '—'}</dd>
            </div>
          </dl>
        </Card>
      </div>
      <Card className="p-6 mt-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <Key className="w-5 h-5 text-pink-500" /> Permissions
        </h3>
        {permissions.length > 0 ? (
          <div className="overflow-x-auto max-h-[60vh] overflow-y-auto rounded-lg border border-border">
            <table className="w-full text-sm">
              <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
                <tr>
                  <th className="text-left py-2">Resource</th>
                  <th className="text-left py-2">Verbs / Details</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {permissions.slice(0, 100).map((p, i) => (
                  <tr key={i} className="hover:bg-muted/30">
                    <td className="py-2 font-mono text-text">{String(p.resource ?? p.apiGroups ?? '—')}</td>
                    <td className="py-2 text-muted font-mono text-xs">{JSON.stringify(p.verbs ?? p.resources ?? p)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-slate-500 text-sm">No permissions data.</p>
        )}
      </Card>
    </PageLayout>
  );
};
