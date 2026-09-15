import React, { useCallback, useEffect, useRef, useState } from 'react';
import { matchPath, useLocation, useNavigate, useParams } from 'react-router-dom';
import { api } from '../lib/api';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, UserCog, Key, Link2, Boxes } from 'lucide-react';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH_COMPACT, UI_TR, UI_TD_COMPACT_TIGHT } from '../lib/tableChrome';
import type { K8sClusterRoleBindingPermission, K8sEffectiveRule, K8sRoleBindingPermission, PodWithRisk } from '../types';
import { PageLoading } from '../design-system/components/PageStatus';

function parseStringArray(value: unknown): string[] {
  if (Array.isArray(value)) return value.map((x) => String(x)).filter(Boolean);
  if (typeof value !== 'string' || !value.trim()) return [];
  try {
    const parsed = JSON.parse(value) as unknown;
    return Array.isArray(parsed) ? parsed.map((x) => String(x)).filter(Boolean) : [];
  } catch {
    return [];
  }
}

function bindingName(binding: Record<string, unknown> | undefined): string {
  return String(binding?.name ?? '—');
}

function roleName(role: Record<string, unknown> | undefined): string {
  return String(role?.name ?? '—');
}

export const IdentityDetail: React.FC = () => {
  const { id, uid } = useParams<{ id?: string; uid?: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const requestSequence = useRef(0);
  const [sa, setSa] = useState<Record<string, unknown> | null>(null);
  const [effectiveRules, setEffectiveRules] = useState<K8sEffectiveRule[]>([]);
  const [roleBindings, setRoleBindings] = useState<K8sRoleBindingPermission[]>([]);
  const [clusterRoleBindings, setClusterRoleBindings] = useState<K8sClusterRoleBindingPermission[]>([]);
  const [linkedPods, setLinkedPods] = useState<PodWithRisk[]>([]);
  const [bindingCounts, setBindingCounts] = useState({ roleBindings: 0, clusterRoleBindings: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const uidFromPath = matchPath({ path: '/identities/uid/:uid', end: true }, location.pathname)?.params.uid;
  const idFromPath = matchPath({ path: '/identities/:id', end: true }, location.pathname)?.params.id;
  const canonicalUid = uid ?? uidFromPath;
  const legacyId = id ?? idFromPath;
  const idOrUid = canonicalUid ?? legacyId;

  const fetchData = useCallback(async () => {
    if (!idOrUid) return;
    const sequence = ++requestSequence.current;
    setLoading(true);
    setError(null);
    try {
      let saData = canonicalUid
        ? await api.getServiceAccountByUid(idOrUid)
        : await api.getServiceAccountByUid(idOrUid);
      if (!saData && !canonicalUid && /^[0-9]+$/.test(idOrUid)) {
        const list = await api.getServiceAccounts({ pageSize: 1000 });
        const legacyMatch = list.serviceAccounts.find((sa) => String(sa.id) === idOrUid);
        saData = legacyMatch ? legacyMatch as unknown as Record<string, unknown> : null;
      }
      if (sequence !== requestSequence.current) return;
      setSa(saData ?? null);
      if (saData) {
        const uidForPermissions = (saData as { uid?: string; id?: string }).uid ?? (saData as { id?: string }).id;
        if (uidForPermissions != null) {
          const permData = await api.getServiceAccountPermissions(String(uidForPermissions));
          if (sequence !== requestSequence.current) return;
          const rb = permData.roleBindings ?? [];
          const crb = permData.clusterRoleBindings ?? [];
          setRoleBindings(rb);
          setClusterRoleBindings(crb);
          setEffectiveRules(permData.effectiveRules ?? []);
          setBindingCounts({
            roleBindings: rb.length,
            clusterRoleBindings: crb.length,
          });
        } else {
          setRoleBindings([]);
          setClusterRoleBindings([]);
          setEffectiveRules([]);
          setBindingCounts({ roleBindings: 0, clusterRoleBindings: 0 });
        }
        const linkedPodUids = new Set(parseStringArray((saData as { linkedPods?: unknown }).linkedPods));
        const clusterId = String((saData as { clusterId?: unknown }).clusterId ?? '');
        const namespace = String((saData as { namespace?: unknown }).namespace ?? '');
        if (linkedPodUids.size > 0 && clusterId && namespace) {
          const podData = await api.getPods({ cluster: clusterId, namespace, pageSize: 1000 });
          if (sequence !== requestSequence.current) return;
          setLinkedPods(podData.pods.filter((pod) => linkedPodUids.has(pod.uid)));
        } else {
          setLinkedPods([]);
        }
      } else {
        setRoleBindings([]);
        setClusterRoleBindings([]);
        setEffectiveRules([]);
        setLinkedPods([]);
        setBindingCounts({ roleBindings: 0, clusterRoleBindings: 0 });
      }
    } catch (err) {
      if (sequence !== requestSequence.current) return;
      setError(err instanceof Error ? err.message : 'Failed to load identity data');
    } finally {
      if (sequence === requestSequence.current) setLoading(false);
    }
  }, [canonicalUid, idOrUid]);

  useEffect(() => {
    void fetchData();
    return () => { requestSequence.current++; };
  }, [fetchData]);

  if (loading || !idOrUid) {
    return <PageLoading message="Loading identity..." className="min-h-[40dvh]" />;
  }

  if (error) {
    return (
      <PageLayout title="Identity unavailable">
        <p role="alert" className="mb-4 text-danger">{error}</p>
        <Button variant="secondary" onClick={() => void fetchData()}>Retry</Button>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onClick={() => navigate('/resources?tab=ServiceAccount')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Service accounts
          </Button>
          <Button variant="secondary" onClick={() => navigate('/resources?tab=Pod')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Pods
          </Button>
        </div>
      </PageLayout>
    );
  }

  if (!sa) {
    return (
      <PageLayout title="Identity not found" description="The service account may have been removed.">
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onClick={() => navigate('/resources?tab=ServiceAccount')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Service accounts
          </Button>
          <Button variant="secondary" onClick={() => navigate('/resources?tab=Pod')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Pods
          </Button>
        </div>
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
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onClick={() => navigate('/resources?tab=ServiceAccount')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Service accounts
          </Button>
          <Button variant="secondary" onClick={() => navigate('/resources?tab=Pod')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Pods
          </Button>
        </div>
      }
    >
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card className="p-6">
          <h3 className="text-section-title text-text mb-4 flex items-center gap-2">
            <UserCog className="w-5 h-5 text-brand" /> Overview
          </h3>
          <dl className="grid grid-cols-1 gap-3 text-body">
            <div>
              <dt className="text-muted">Namespace</dt>
              <dd className="text-text font-mono">{namespace}</dd>
            </div>
            <div>
              <dt className="text-muted">Cluster</dt>
              <dd className="text-text font-mono">{sa.clusterId != null ? String(sa.clusterId) : '—'}</dd>
            </div>
            <div>
              <dt className="text-muted">UID</dt>
              <dd className="text-text font-mono break-all">{sa.uid != null ? String(sa.uid) : '—'}</dd>
            </div>
          </dl>
        </Card>
        <Card className="p-6">
          <h3 className="text-section-title text-text mb-4 flex items-center gap-2">
            <Boxes className="w-5 h-5 text-brand" /> Workloads using this identity
          </h3>
          {linkedPods.length > 0 ? (
            <div className="space-y-2">
              {linkedPods.map((pod) => (
                <button
                  key={pod.uid}
                  type="button"
                  onClick={() => navigate(`/resources/pods/uid/${encodeURIComponent(pod.uid)}`)}
                  className="block w-full rounded-lg border border-border bg-base/40 px-3 py-2 text-left hover:border-brand/40"
                >
                  <span className="block font-mono text-caption text-text">{pod.namespace}/{pod.name}</span>
                  <span className="mt-0.5 block break-all font-mono text-meta text-muted-2">{pod.uid}</span>
                </button>
              ))}
            </div>
          ) : (
            <p className="text-muted text-body">No linked pods synced for this ServiceAccount.</p>
          )}
        </Card>
      </div>
      <Card className="p-6 mt-6">
        <h3 className="text-section-title text-text mb-2 flex items-center gap-2">
          <Link2 className="w-5 h-5 text-brand" /> Binding sources
        </h3>
        <p className="text-caption text-muted mb-4">
          Shows the RoleBinding or ClusterRoleBinding that grants each permission set.
        </p>
        {roleBindings.length + clusterRoleBindings.length > 0 ? (
          <div className="grid gap-2 md:grid-cols-2">
            {roleBindings.map((row, index) => (
              <div key={`rb-${index}`} className="rounded-lg border border-border bg-base/40 px-3 py-2">
                <p className="text-caption font-semibold text-text">RoleBinding: {bindingName(row.roleBinding)}</p>
                <p className="mt-1 text-caption text-muted">{row.clusterRole ? "ClusterRole" : "Role"}: <span className="font-mono text-text">{roleName(row.clusterRole ?? row.role)}</span></p>
              </div>
            ))}
            {clusterRoleBindings.map((row, index) => (
              <div key={`crb-${index}`} className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2">
                <p className="text-caption font-semibold text-amber-100">ClusterRoleBinding: {bindingName(row.clusterRoleBinding)}</p>
                <p className="mt-1 text-caption text-muted">ClusterRole: <span className="font-mono text-text">{roleName(row.clusterRole)}</span></p>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-muted text-body">No RoleBinding or ClusterRoleBinding rows reference this ServiceAccount.</p>
        )}
      </Card>
      <Card className="p-6 mt-6">
        <h3 className="text-section-title text-text mb-2 flex items-center gap-2">
          <Key className="w-5 h-5 text-brand" /> Synchronized RBAC grants
        </h3>
        <p className="text-caption text-muted mb-4">
          Based on synchronized inventory; this is not a live authorization check. RoleBindings: {bindingCounts.roleBindings} · ClusterRoleBindings: {bindingCounts.clusterRoleBindings} · Rules
          shown: {Math.min(effectiveRules.length, 200)} / {effectiveRules.length}
        </p>
        {effectiveRules.length > 0 ? (
          <div className="ui-table-scroll rounded-lg border border-border">
            <table className={UI_TABLE}>
              <thead className={UI_THEAD_STICKY}>
                <tr>
                  <th className={UI_TH_COMPACT}>Scope</th>
                  <th className={UI_TH_COMPACT}>API groups</th>
                  <th className={UI_TH_COMPACT}>Resources</th>
                  <th className={UI_TH_COMPACT}>Verbs</th>
                  <th className={UI_TH_COMPACT}>Resource names</th>
                </tr>
              </thead>
              <tbody>
                {effectiveRules.slice(0, 200).map((p, i) => (
                  <tr key={i} className={UI_TR}>
                    <td className={UI_TD_COMPACT_TIGHT}>{p.scope === "namespace" ? `Namespace: ${p.namespace}` : p.scope === "cluster" ? "Cluster" : "Unknown"}</td>
                    <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-text text-caption`}>
                      {(p.apiGroups ?? []).map(group => group || '(core)').join(', ') || '—'}
                    </td>
                    <td className={`${UI_TD_COMPACT_TIGHT} font-mono text-text text-caption`}>
                      {[...(p.resources ?? []), ...(p.nonResourceURLs ?? [])].join(', ') || '—'}
                    </td>
                    <td className={`${UI_TD_COMPACT_TIGHT} text-muted font-mono text-caption`}>
                      {(p.verbs ?? []).join(', ') || '—'}
                    </td>
                    <td className={`${UI_TD_COMPACT_TIGHT} text-muted font-mono text-caption`}>
                      {(p.resourceNames ?? []).join(', ') || '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-muted text-body">No effective rules (no bindings or roles synced yet).</p>
        )}
      </Card>
    </PageLayout>
  );
};
