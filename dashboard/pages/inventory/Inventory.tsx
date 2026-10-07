import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { RefreshCw } from 'lucide-react';
import { PageLayout } from '../../design-system/layouts/PageLayout';
import { Button } from '../../components/ui/Button';
import { api } from '../../lib/api';
import { PAGE_TITLES } from '../../lib/pageTitles';
import type { InventoryView } from '../../lib/entityLinks';
import { useClusterStore } from '../../store/clusterStore';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';
import type { K8sResource, PodWithRisk } from '../../types';
import { WorkloadsView, type WorkloadFilters } from './WorkloadsView';
import { IdentitiesView, type IdentityFilters } from './IdentitiesView';
import { ClustersView } from './ClustersView';
import { PodPanel } from './PodPanel';
import { IdentityPanel } from './IdentityPanel';
import { LEVELS } from './shared';

const VIEWS: { id: InventoryView; label: string }[] = [
  { id: 'workloads', label: 'Workloads' },
  { id: 'identities', label: 'Identities' },
  { id: 'clusters', label: 'Clusters' },
];

/** Old `?tab=` values from the seven-tab page, mapped to a view and kind filter. */
const LEGACY_TABS: Record<string, { view: InventoryView; kind?: string }> = {
  Pod: { view: 'workloads' },
  Inventory: { view: 'identities' },
  RBACOverview: { view: 'identities' },
  ServiceAccount: { view: 'identities', kind: 'ServiceAccount' },
  Role: { view: 'identities', kind: 'Role' },
  ClusterRole: { view: 'identities', kind: 'ClusterRole' },
  RoleBinding: { view: 'identities', kind: 'Binding' },
  ClusterRoleBinding: { view: 'identities', kind: 'Binding' },
  Bindings: { view: 'identities', kind: 'Binding' },
};

function intParam(v: string | null, fallback: number): number {
  const n = Number(v);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : fallback;
}

/** Inventory: what is running, how risky it is, and where to go next. */
export const Inventory: React.FC = () => {
  const [params, setParams] = useSearchParams();
  const { allowedRoutes } = useOperationalMaterialization();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const setSelectedClusterId = useClusterStore((s) => s.setSelectedClusterId);
  const [refreshKey, setRefreshKey] = useState(0);
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [selectedPod, setSelectedPod] = useState<PodWithRisk | null>(null);
  const [selectedIdentity, setSelectedIdentity] = useState<K8sResource | null>(null);

  // Old links: ?tab= from the seven-tab page becomes ?view= (+ kind).
  useEffect(() => {
    const legacy = params.get('tab');
    if (!legacy) return;
    const next = new URLSearchParams(params);
    next.delete('tab');
    const legacySearch = next.get('search');
    if (legacySearch) {
      next.delete('search');
      next.set('q', legacySearch);
    }
    const mapped = LEGACY_TABS[legacy];
    if (mapped && mapped.view !== 'workloads') next.set('view', mapped.view);
    if (mapped?.kind) next.set('kind', mapped.kind);
    setParams(next, { replace: true });
  }, [params, setParams]);

  const clusterId = selectedClusterId ?? null;
  const prevCluster = useRef(clusterId);

  // A deep-linked ?clusterId= selects that cluster in the header, then leaves the URL.
  // Its namespace and node filters belong to that cluster, so they are kept.
  const clusterParam = params.get('clusterId');
  useEffect(() => {
    if (!clusterParam) return;
    prevCluster.current = clusterParam;
    setSelectedClusterId(clusterParam);
    const next = new URLSearchParams(params);
    next.delete('clusterId');
    setParams(next, { replace: true });
  }, [clusterParam, params, setParams, setSelectedClusterId]);

  const rawView = params.get('view');
  const view: InventoryView = rawView === 'identities' || rawView === 'clusters' ? rawView : 'workloads';

  // Namespaces and nodes belong to one cluster; a cluster change clears those filters.
  useEffect(() => {
    if (prevCluster.current === clusterId) return;
    prevCluster.current = clusterId;
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        ['namespace', 'node', 'page'].forEach((k) => next.delete(k));
        return next;
      },
      { replace: true },
    );
  }, [clusterId, setParams]);

  useEffect(() => {
    if (!clusterId) {
      setNamespaces([]);
      return;
    }
    let cancelled = false;
    api
      .getClusterInventoryStrict(clusterId)
      .then((inv) => !cancelled && setNamespaces([...inv.namespaces].sort()))
      .catch(() => !cancelled && setNamespaces([]));
    return () => {
      cancelled = true;
    };
  }, [clusterId, refreshKey]);

  const patchParams = useCallback(
    (patch: Record<string, string | number>) => {
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          for (const [k, v] of Object.entries(patch)) {
            const s = String(v);
            if (!s || (k === 'page' && s === '1')) next.delete(k);
            else next.set(k, s);
          }
          return next;
        },
        { replace: true },
      );
    },
    [setParams],
  );

  const rawLevel = params.get('level') ?? '';
  const workloadFilters: WorkloadFilters = {
    search: params.get('q') ?? '',
    namespace: params.get('namespace') ?? '',
    node: params.get('node') ?? '',
    level: (LEVELS as readonly string[]).includes(rawLevel) || rawLevel === 'unscored' ? (rawLevel as WorkloadFilters['level']) : '',
    page: intParam(params.get('page'), 1),
    pageSize: intParam(params.get('size'), 50),
  };
  const identityFilters: IdentityFilters = {
    search: params.get('q') ?? '',
    namespace: params.get('namespace') ?? '',
    kind: params.get('kind') ?? '',
    page: intParam(params.get('page'), 1),
    pageSize: intParam(params.get('size'), 50),
  };
  const setFilters = useCallback(
    (patch: Partial<WorkloadFilters & IdentityFilters>) => {
      const out: Record<string, string | number> = {};
      if (patch.search !== undefined) out.q = patch.search;
      if (patch.namespace !== undefined) out.namespace = patch.namespace;
      if (patch.node !== undefined) out.node = patch.node;
      if (patch.level !== undefined) out.level = patch.level;
      if (patch.kind !== undefined) out.kind = patch.kind;
      if (patch.page !== undefined) out.page = patch.page;
      if (patch.pageSize !== undefined) out.size = patch.pageSize === 50 ? '' : patch.pageSize;
      patchParams(out);
    },
    [patchParams],
  );

  const switchView = (id: InventoryView) => {
    setSelectedPod(null);
    setSelectedIdentity(null);
    setParams(id === 'workloads' ? new URLSearchParams() : new URLSearchParams({ view: id }));
  };

  return (
    <PageLayout
      title={PAGE_TITLES.resources}
      description="What is running, how risky it is, and where to look next."
      actions={
        <Button size="sm" variant="secondary" onClick={() => setRefreshKey((k) => k + 1)} aria-label="Refresh">
          <RefreshCw className="h-4 w-4" aria-hidden />
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        <nav aria-label="Inventory views" className="flex w-fit flex-wrap gap-1 rounded-lg border border-border bg-base/60 p-1">
          {VIEWS.map((v) => (
            <button
              key={v.id}
              type="button"
              aria-current={view === v.id ? 'page' : undefined}
              onClick={() => switchView(v.id)}
              className={`min-h-9 rounded-md px-3 text-caption font-semibold transition-colors ${
                view === v.id ? 'bg-surface-2 text-text shadow-sm' : 'text-muted hover:text-text'
              }`}
            >
              {v.label}
            </button>
          ))}
        </nav>

        {view === 'workloads' ? (
          <WorkloadsView
            clusterId={clusterId}
            filters={workloadFilters}
            setFilters={setFilters}
            namespaces={namespaces}
            selectedUid={selectedPod?.uid ?? null}
            onSelect={setSelectedPod}
            refreshKey={refreshKey}
            allowedRoutes={allowedRoutes}
          />
        ) : view === 'identities' ? (
          <IdentitiesView
            clusterId={clusterId}
            filters={identityFilters}
            setFilters={setFilters}
            namespaces={namespaces}
            selectedUid={selectedIdentity?.id ?? null}
            onSelect={setSelectedIdentity}
            refreshKey={refreshKey}
          />
        ) : (
          <ClustersView
            selectedClusterId={clusterId}
            refreshKey={refreshKey}
            canFindings={allowedRoutes.includes('/risks')}
            onOpenCluster={(id) => {
              setSelectedClusterId(id);
              switchView('workloads');
            }}
          />
        )}
      </div>

      <PodPanel pod={selectedPod} onClose={() => setSelectedPod(null)} allowedRoutes={allowedRoutes} />
      <IdentityPanel resource={selectedIdentity} onClose={() => setSelectedIdentity(null)} />
    </PageLayout>
  );
};
