import React, { useState, useEffect, useLayoutEffect, useMemo, useCallback, useRef } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { Box, UserCog, Scroll, Key, RefreshCw, Link2, LayoutGrid, Shield, Target, AlertTriangle, Zap, PanelRight, Boxes } from 'lucide-react';
import { Card } from '../design-system/components/Card';
import { Tabs } from '../design-system/components/Tabs';
import { FilterBar } from '../design-system/components/FilterBar';
import { Table, type TableColumn } from '../design-system/components/Table';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageError } from '../design-system/components/PageStatus';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { Pagination } from '../components/Pagination';
import { Button } from '../components/ui/Button';
import { DataFreshness } from '../components/DataFreshness';
import { StatCard } from '../components/StatCard';
import { api, isApiError } from '../lib/api';
import { useClusterStore } from '../store/clusterStore';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { AttackPath, AttackPathSummary, Cluster, K8sRbacResourceDetail, K8sResource, PodWithRisk, ServiceAccountK8sPermissions } from '../types';
import { deriveUnifiedRiskLevelFromScore, getSeverityBadgeClass, getPodStatusBadgeClass } from '../lib/severity';
import { getClusterDisplayName } from '../lib/clusterDisplay';
import { PAGE_TITLES } from '../lib/pageTitles';
import type { SemanticVisibilityState } from '../lib/visibilityEngine';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
const TAB_IDS = ['Pod', 'Inventory', 'ServiceAccount', 'Role', 'RoleBinding', 'ClusterRole', 'Bindings'] as const;
type TabId = (typeof TAB_IDS)[number];

type ResourceSelection =
  | { kind: 'Pod'; pod: PodWithRisk }
  | { kind: 'Resource'; resource: K8sResource };

type AttackPathUiIssue = {
  state?: SemanticVisibilityState;
  title: string;
  description: string;
  detail?: string;
};

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

function objectName(value: Record<string, unknown> | undefined): string {
  return String(value?.name ?? '—');
}

function stringValue(value: unknown): string {
  return value == null || value === '' ? '—' : String(value);
}

function ruleSummary(rule: { verbs?: string[]; resources?: string[]; apiGroups?: string[]; nonResourceURLs?: string[] }): string {
  const verbs = rule.verbs?.length ? rule.verbs.join(', ') : '—';
  const resources = rule.resources?.length
    ? rule.resources.join(', ')
    : rule.nonResourceURLs?.length
      ? `nonResourceURLs ${rule.nonResourceURLs.join(', ')}`
      : '—';
  const apiGroups = rule.apiGroups?.length ? ` · apiGroups ${rule.apiGroups.join(', ')}` : '';
  return `${verbs} on ${resources}${apiGroups}`;
}

function classifyAttackPathIssue(error: unknown, fallbackTitle = 'Attack paths unavailable'): AttackPathUiIssue {
  if (isApiError(error)) {
    if (error.status === 401) {
      return {
        state: 'no_permission',
        title: 'Session is not active',
        description: 'Sign in again to load attack paths.',
        detail: error.body?.code ? `Core returned ${error.body.code}.` : undefined,
      };
    }
    if (error.status === 403 && error.body?.reason === 'cluster_scope') {
      const cluster = error.body.required_cluster_id;
      return {
        state: 'no_scope',
        title: 'Cluster outside your scope',
        description: cluster
          ? `This account is not scoped for ${cluster}.`
          : 'This account is not scoped for the selected cluster.',
      };
    }
    if (error.status === 403) {
      const required = error.body?.required_permission ?? error.body?.required_permissions?.join(', ');
      return {
        state: 'no_permission',
        title: 'Attack paths hidden',
        description: required ? `Attack paths require ${required}.` : 'Attack paths are not available for this role.',
      };
    }
    return {
      title: fallbackTitle,
      description: error.status >= 500 ? 'Core could not build attack-path data.' : error.message,
      detail: `HTTP ${error.status}`,
    };
  }
  return {
    title: fallbackTitle,
    description: error instanceof Error ? error.message : 'Could not load attack paths.',
  };
}

function AttackPathIssueSurface({
  issue,
  compact = false,
}: {
  issue: AttackPathUiIssue;
  compact?: boolean;
}) {
  const reason = `${issue.description}${issue.detail ? ` ${issue.detail}` : ''}`;
  if (issue.state) {
    return (
      <SemanticEmptyState
        state={issue.state}
        title={issue.title}
        reason={reason}
        compact={compact}
      />
    );
  }
  return <PageError title={issue.title} description={reason} className={compact ? 'py-4' : 'py-6'} />;
}

/** Legacy URLs merged into Resources → Inventory */
const LEGACY_TAB_ALIASES: Record<string, TabId> = { RBACOverview: 'Inventory' };

export const Resources: React.FC = () => {
  const navigate = useNavigate();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const [searchParams, setSearchParams] = useSearchParams();
  const tabParam = searchParams.get('tab');
  const namespaceParam = searchParams.get('namespace') ?? '';
  const initialTab: TabId =
    tabParam && TAB_IDS.includes(tabParam as TabId)
      ? (tabParam as TabId)
      : tabParam && LEGACY_TAB_ALIASES[tabParam]
        ? LEGACY_TAB_ALIASES[tabParam]
        : 'Pod';
  const [activeTab, setActiveTab] = useState<TabId>(initialTab);
  const [namespaceFilter, setNamespaceFilter] = useState(namespaceParam);
  const [resources, setResources] = useState<K8sResource[]>([]);
  const [pods, setPods] = useState<PodWithRisk[]>([]);
  const [podsTotal, setPodsTotal] = useState(0);
  const [inventoryClusters, setInventoryClusters] = useState<Cluster[]>([]);
  const [loading, setLoading] = useState(false);
  const [dataUpdatedAt, setDataUpdatedAt] = useState<Date | null>(null);
  const [dataError, setDataError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [searchTerm, setSearchTerm] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [sortBy, setSortBy] = useState<
    'name_asc' | 'namespace_asc' | 'risk_desc' | 'created_desc'
  >('risk_desc');
  const [clusterNamespaces, setClusterNamespaces] = useState<string[]>([]);
  const [attackSummary, setAttackSummary] = useState<AttackPathSummary | null>(null);
  const [attackSummaryLoading, setAttackSummaryLoading] = useState(false);
  const [attackSummaryError, setAttackSummaryError] = useState<AttackPathUiIssue | null>(null);
  const [selectedResource, setSelectedResource] = useState<ResourceSelection | null>(null);
  const [selectedAttackPaths, setSelectedAttackPaths] = useState<AttackPath[]>([]);
  const [selectedAttackPathsLoading, setSelectedAttackPathsLoading] = useState(false);
  const [selectedAttackPathsError, setSelectedAttackPathsError] = useState<AttackPathUiIssue | null>(null);
  const [selectedServiceAccountPermissions, setSelectedServiceAccountPermissions] = useState<ServiceAccountK8sPermissions | null>(null);
  const [selectedServiceAccountDetail, setSelectedServiceAccountDetail] = useState<Record<string, unknown> | null>(null);
  const [selectedServiceAccountPods, setSelectedServiceAccountPods] = useState<PodWithRisk[]>([]);
  const [selectedServiceAccountLoading, setSelectedServiceAccountLoading] = useState(false);
  const [selectedRbacResourceDetail, setSelectedRbacResourceDetail] = useState<K8sRbacResourceDetail | null>(null);
  const [selectedRbacResourceLoading, setSelectedRbacResourceLoading] = useState(false);

  const namespaceParamRef = useRef<string | null>(null);
  const podsFetchGenRef = useRef(0);

  /** Canonicalize legacy hash URLs (e.g. #/resources?tab=RBACOverview) → ?tab=Inventory before paint. */
  useLayoutEffect(() => {
    const t = searchParams.get('tab');
    if (t !== 'RBACOverview') return;
    const next = new URLSearchParams(searchParams);
    next.set('tab', 'Inventory');
    const qs = next.toString();
    navigate({ pathname: '/resources', search: qs ? `?${qs}` : '' }, { replace: true });
  }, [searchParams, navigate]);

  useEffect(() => {
    const t = searchParams.get('tab');
    if (t && TAB_IDS.includes(t as TabId)) setActiveTab(t as TabId);
    const nextNs = searchParams.get('namespace') ?? '';
    if (namespaceParamRef.current !== null && namespaceParamRef.current !== nextNs) {
      setPage(1);
    }
    namespaceParamRef.current = nextNs;
    setNamespaceFilter(nextNs);
  }, [searchParams]);

  useEffect(() => {
    const id = window.setTimeout(() => setDebouncedSearch(searchTerm.trim()), 300);
    return () => window.clearTimeout(id);
  }, [searchTerm]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);

  const fetchResources = useCallback(async () => {
    const gen = ++podsFetchGenRef.current;
    setLoading(true);
    setDataError(null);
    try {
      if (activeTab === 'Inventory') {
        const data = await api.getClustersStats();
        if (gen !== podsFetchGenRef.current) return;
        setInventoryClusters(data);
        setDataUpdatedAt(new Date());
        return;
      }
      if (activeTab === 'Pod') {
        const data = await api.getPods({
          page,
          pageSize,
          cluster: selectedClusterId ?? undefined,
          namespace: namespaceFilter || undefined,
          search: debouncedSearch || undefined,
          sortBy,
        });
        if (gen !== podsFetchGenRef.current) return;
        setPods(data.pods);
        setPodsTotal(data.total);
        setDataUpdatedAt(new Date());
      } else if (activeTab === 'Bindings') {
        const [rb, crb] = await Promise.all([
          api.getResources('RoleBinding', {
            cluster: selectedClusterId ?? undefined,
            namespace: namespaceFilter || undefined,
          }),
          api.getResources('ClusterRoleBinding', { cluster: selectedClusterId ?? undefined }),
        ]);
        if (gen !== podsFetchGenRef.current) return;
        setResources([...rb, ...crb]);
        setDataUpdatedAt(new Date());
      } else if (activeTab === 'ClusterRole') {
        const data = await api.getResources('ClusterRole', { cluster: selectedClusterId ?? undefined });
        if (gen !== podsFetchGenRef.current) return;
        setResources(data);
        setDataUpdatedAt(new Date());
      } else {
        const data = await api.getResources(activeTab, {
          cluster: selectedClusterId ?? undefined,
          namespace: namespaceFilter || undefined,
        });
        if (gen !== podsFetchGenRef.current) return;
        setResources(data);
        setDataUpdatedAt(new Date());
      }
    } catch (err) {
      if (gen === podsFetchGenRef.current) {
        setDataError(err instanceof Error ? err.message : 'Could not refresh resources');
      }
    } finally {
      if (gen === podsFetchGenRef.current) {
        setLoading(false);
      }
    }
  }, [activeTab, page, pageSize, selectedClusterId, namespaceFilter, debouncedSearch, sortBy]);

  useEffect(() => {
    setPage(1);
  }, [activeTab]);

  useEffect(() => {
    setPage(1);
  }, [selectedClusterId]);

  useEffect(() => {
    if (activeTab !== 'Pod' && sortBy === 'created_desc') {
      setSortBy('name_asc');
    }
  }, [activeTab, sortBy]);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch]);

  useEffect(() => {
    setPage(1);
  }, [sortBy]);

  useEffect(() => {
    fetchResources();
  }, [fetchResources]);

  usePolling(fetchResources, intervalMs, { refreshTrigger });

  const reloadClusterNamespaces = useCallback(async () => {
    if (!selectedClusterId) {
      setClusterNamespaces([]);
      return;
    }
    try {
      const inv = await api.getClusterInventory(selectedClusterId);
      const raw = inv?.namespaces ?? [];
      setClusterNamespaces([...raw].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' })));
    } catch {
      setClusterNamespaces([]);
    }
  }, [selectedClusterId]);
  usePolling(reloadClusterNamespaces, intervalMs, { refreshTrigger });

  const fetchAttackSummary = useCallback(async () => {
    setAttackSummaryLoading(true);
    setAttackSummaryError(null);
    try {
      const data = await api.getAttackPathsSummaryStrict(selectedClusterId ?? undefined);
      setAttackSummary(data);
    } catch (err) {
      setAttackSummary(null);
      setAttackSummaryError(classifyAttackPathIssue(err, 'Attack-path counts unavailable'));
    } finally {
      setAttackSummaryLoading(false);
    }
  }, [selectedClusterId]);

  useEffect(() => {
    void fetchAttackSummary();
  }, [fetchAttackSummary]);

  usePolling(fetchAttackSummary, intervalMs, { refreshTrigger });

  useEffect(() => {
    setSelectedResource(null);
    setSelectedAttackPaths([]);
    setSelectedAttackPathsError(null);
    setSelectedServiceAccountPermissions(null);
    setSelectedServiceAccountDetail(null);
    setSelectedServiceAccountPods([]);
    setSelectedRbacResourceDetail(null);
  }, [activeTab, selectedClusterId]);

  useEffect(() => {
    if (!selectedResource) {
      setSelectedAttackPaths([]);
      setSelectedAttackPathsError(null);
      setSelectedAttackPathsLoading(false);
      setSelectedServiceAccountPermissions(null);
      setSelectedServiceAccountDetail(null);
      setSelectedServiceAccountPods([]);
      setSelectedServiceAccountLoading(false);
      setSelectedRbacResourceDetail(null);
      setSelectedRbacResourceLoading(false);
      return;
    }

    if (selectedResource.kind === 'Pod') {
      let cancelled = false;
      setSelectedAttackPathsLoading(true);
      setSelectedAttackPathsError(null);
      setSelectedServiceAccountPermissions(null);
      setSelectedServiceAccountDetail(null);
      setSelectedServiceAccountPods([]);
      setSelectedServiceAccountLoading(false);
      setSelectedRbacResourceDetail(null);
      setSelectedRbacResourceLoading(false);
      api.getAttackPathsForPodStrict(selectedResource.pod.uid).then((paths) => {
        if (!cancelled) setSelectedAttackPaths(paths);
      }).catch((err) => {
        if (!cancelled) {
          setSelectedAttackPaths([]);
          setSelectedAttackPathsError(classifyAttackPathIssue(err));
        }
      }).finally(() => {
        if (!cancelled) setSelectedAttackPathsLoading(false);
      });
      return () => {
        cancelled = true;
      };
    }

    if (selectedResource.resource.kind === 'ServiceAccount') {
      let cancelled = false;
      setSelectedServiceAccountLoading(true);
      setSelectedAttackPaths([]);
      setSelectedAttackPathsError(null);
      setSelectedAttackPathsLoading(false);
      setSelectedRbacResourceDetail(null);
      setSelectedRbacResourceLoading(false);
      Promise.all([
        api.getServiceAccountByUid(selectedResource.resource.id),
        api.getServiceAccountPermissions(selectedResource.resource.id),
      ]).then(async ([detail, permissions]) => {
        if (cancelled) return;
        setSelectedServiceAccountDetail(detail);
        setSelectedServiceAccountPermissions(permissions);
        const linkedUids = new Set(parseStringArray(detail?.linkedPods));
        const clusterId = String(detail?.clusterId ?? selectedResource.resource.clusterId ?? '');
        const namespace = String(detail?.namespace ?? selectedResource.resource.namespace ?? '');
        if (linkedUids.size > 0 && clusterId && namespace) {
          const pods = await api.getPods({ cluster: clusterId, namespace, pageSize: 1000 });
          if (!cancelled) setSelectedServiceAccountPods(pods.pods.filter((pod) => linkedUids.has(pod.uid)));
        } else {
          setSelectedServiceAccountPods([]);
        }
      }).finally(() => {
        if (!cancelled) setSelectedServiceAccountLoading(false);
      });
      return () => {
        cancelled = true;
      };
    }

    if (['Role', 'RoleBinding', 'ClusterRole', 'ClusterRoleBinding'].includes(selectedResource.resource.kind)) {
      let cancelled = false;
      setSelectedAttackPaths([]);
      setSelectedAttackPathsError(null);
      setSelectedAttackPathsLoading(false);
      setSelectedServiceAccountPermissions(null);
      setSelectedServiceAccountDetail(null);
      setSelectedServiceAccountPods([]);
      setSelectedServiceAccountLoading(false);
      setSelectedRbacResourceLoading(true);
      api.getResourceDetail(selectedResource.resource.kind, selectedResource.resource.id).then((detail) => {
        if (!cancelled) setSelectedRbacResourceDetail(detail);
      }).finally(() => {
        if (!cancelled) setSelectedRbacResourceLoading(false);
      });
      return () => {
        cancelled = true;
      };
    }

    setSelectedAttackPaths([]);
    setSelectedAttackPathsError(null);
    setSelectedAttackPathsLoading(false);
    setSelectedServiceAccountPermissions(null);
    setSelectedServiceAccountDetail(null);
    setSelectedServiceAccountPods([]);
    setSelectedServiceAccountLoading(false);
    setSelectedRbacResourceDetail(null);
    setSelectedRbacResourceLoading(false);
    return undefined;
  }, [selectedResource]);

  const namespaceSelectOptions = useMemo(() => {
    const next = new Set(clusterNamespaces);
    const cur = namespaceFilter.trim();
    if (cur) next.add(cur);
    return [...next].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }));
  }, [clusterNamespaces, namespaceFilter]);

  const clusterScope = selectedClusterId?.trim() || undefined;

  const scrollToInspector = useCallback(() => {
    window.setTimeout(() => {
      document.getElementById('resource-inspector')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }, 0);
  }, []);

  const selectPod = useCallback((pod: PodWithRisk) => {
    setSelectedResource({ kind: 'Pod', pod });
    scrollToInspector();
  }, [scrollToInspector]);

  const selectResource = useCallback((resource: K8sResource) => {
    setSelectedResource({ kind: 'Resource', resource });
    scrollToInspector();
  }, [scrollToInspector]);

  const clearSelection = useCallback(() => {
    setSelectedResource(null);
    setSelectedAttackPaths([]);
    setSelectedAttackPathsError(null);
    setSelectedAttackPathsLoading(false);
    setSelectedServiceAccountPermissions(null);
    setSelectedServiceAccountLoading(false);
  }, []);

  const visibleAttackPathCount = selectedResource?.kind === 'Pod' ? selectedAttackPaths.length : 0;
  const totalAttackPathCount = attackSummary?.totalPaths ?? 0;
  const selectedPodChainPathCount =
    selectedResource?.kind === 'Pod' && selectedResource.pod.riskSignals?.pathCount != null
      ? selectedResource.pod.riskSignals.pathCount
      : null;
  const attackPathScopeValue =
    selectedResource?.kind === 'Pod'
      ? `${visibleAttackPathCount.toLocaleString('en-US')} / ${totalAttackPathCount.toLocaleString('en-US')}`
      : totalAttackPathCount.toLocaleString('en-US');

  const inventoryScoped = useMemo(() => {
    if (!clusterScope) return inventoryClusters;
    return inventoryClusters.filter((c) => c.id === clusterScope);
  }, [inventoryClusters, clusterScope]);

  const inventoryTotals = useMemo(
    () => ({
      sa: inventoryScoped.reduce((a, c) => a + (Number(c.serviceAccountCount) || 0), 0),
      role: inventoryScoped.reduce((a, c) => a + (Number(c.roleCount) || 0), 0),
      clusterRole: inventoryScoped.reduce((a, c) => a + (Number(c.clusterRoleCount) || 0), 0),
      rb: inventoryScoped.reduce((a, c) => a + (Number(c.roleBindingCount) || 0), 0),
      crb: inventoryScoped.reduce((a, c) => a + (Number(c.clusterRoleBindingCount) || 0), 0),
    }),
    [inventoryScoped],
  );

  const tabs = [
    { id: 'Pod' as const, label: 'Pods', icon: <Box size={16} /> },
    { id: 'Inventory' as const, label: 'Inventory', icon: <LayoutGrid size={16} /> },
    { id: 'ServiceAccount', label: 'Service Accounts', icon: <UserCog size={16} /> },
    { id: 'Role', label: 'Roles', icon: <Scroll size={16} /> },
    { id: 'RoleBinding', label: 'Role Bindings', icon: <Key size={16} /> },
    { id: 'ClusterRole', label: 'ClusterRoles', icon: <Scroll size={16} /> },
    { id: 'Bindings', label: 'All bindings', icon: <Link2 size={16} /> },
  ];

  const filteredResources = useMemo(() => {
    if (activeTab === 'Pod' || activeTab === 'Inventory') return [];
    if (!searchTerm.trim()) return resources;
    const q = searchTerm.trim().toLowerCase();
    return resources.filter(
      (r) =>
        r.name.toLowerCase().includes(q) ||
        (r.namespace || '').toLowerCase().includes(q) ||
        (r.kind || '').toLowerCase().includes(q),
    );
  }, [activeTab, resources, searchTerm]);

  const sortedResources = useMemo(() => {
    const out = [...filteredResources];
    out.sort((a, b) => {
      switch (sortBy) {
        case 'namespace_asc':
          return (a.namespace || '').localeCompare(b.namespace || '');
        case 'risk_desc':
          return (b.kind || '').localeCompare(a.kind || '');
        case 'name_asc':
        default:
          return a.name.localeCompare(b.name);
      }
    });
    return out;
  }, [filteredResources, sortBy]);

  const paginatedResources = useMemo(() => {
    if (activeTab === 'Pod' || activeTab === 'Inventory') return [];
    const start = (page - 1) * pageSize;
    return sortedResources.slice(start, start + pageSize);
  }, [activeTab, sortedResources, page, pageSize]);

  const totalForPagination =
    activeTab === 'Pod' ? podsTotal : activeTab === 'Inventory' ? 0 : sortedResources.length;

  const handleResourceView = useCallback(
    (resource: K8sResource) => {
      if (resource.kind === 'ServiceAccount') {
        navigate(`/identities/uid/${encodeURIComponent(resource.id)}`);
      } else if (
        (resource.kind === 'Role' ||
          resource.kind === 'RoleBinding' ||
          resource.kind === 'ClusterRole' ||
          resource.kind === 'ClusterRoleBinding') &&
        resource.clusterId
      ) {
        navigate(`/clusters/${resource.clusterId}`);
      }
    },
    [navigate],
  );

  const openPodDetail = useCallback(
    (pod: PodWithRisk) => {
      navigate(`/resources/pods/uid/${encodeURIComponent(pod.uid)}`);
    },
    [navigate],
  );

  const podColumns: TableColumn<PodWithRisk>[] = useMemo(
    () => [
      {
        key: 'name',
        header: 'Name',
        cell: (pod) => (
          <div>
            <button
              type="button"
              className="text-left font-medium text-text hover:text-brand"
              onClick={() => openPodDetail(pod)}
              aria-label={`Open pod detail for ${pod.name} in ${pod.namespace}`}
            >
              {pod.name}
            </button>
            <div className="text-caption text-muted-2">{pod.namespace}</div>
          </div>
        ),
      },
      {
        key: 'node',
        header: 'Node',
        cell: (pod) => <span className="font-mono text-caption text-muted">{pod.nodeName ?? '—'}</span>,
      },
      {
        key: 'status',
        header: 'Status',
        cell: (pod) => (
          <span
            className={`inline-flex items-center rounded border px-2 py-0.5 text-caption font-medium ${getPodStatusBadgeClass(pod.status)}`}
          >
            {pod.status ?? '—'}
          </span>
        ),
      },
      {
        key: 'risk',
        header: 'Risk',
        cell: (pod) => {
          const rawScore = pod.unifiedScore ?? pod.totalScore;
          const scoreNum = rawScore != null ? Number(rawScore) : NaN;
          const fromV3 =
            Number.isFinite(scoreNum) &&
            pod.finalLevel &&
            ['critical', 'high', 'medium', 'low'].includes(String(pod.finalLevel).toLowerCase());
          const level = (fromV3
            ? String(pod.finalLevel).toLowerCase()
            : deriveUnifiedRiskLevelFromScore(Number.isFinite(scoreNum) ? scoreNum : undefined) ?? 'low') as
            | 'critical'
            | 'high'
            | 'medium'
            | 'low';
          return (
            <span
              className={`inline-flex items-center rounded border px-2 py-0.5 text-caption font-semibold uppercase ${getSeverityBadgeClass(level)}`}
            >
              {level}
              {Number.isFinite(scoreNum) ? (
                <span className="ml-1.5 font-mono normal-case opacity-95">{Math.round(scoreNum)}/100</span>
              ) : null}
            </span>
          );
        },
      },
      {
        key: 'actions',
        header: <span className="block text-right">Actions</span>,
        headerClassName: 'text-right',
        className: 'text-right whitespace-nowrap',
        cell: (pod) => (
          <div className="flex flex-col items-end gap-1">
            <Button
              variant="primary"
              size="sm"
              className="!px-2.5 !py-1 !text-caption"
              onClick={() => openPodDetail(pod)}
            >
              Pod detail
            </Button>
            <button
              type="button"
              className="text-caption font-medium text-muted hover:text-brand"
              onClick={() => selectPod(pod)}
            >
              Show attack paths
            </button>
          </div>
        ),
      },
    ],
    [openPodDetail, selectPod],
  );

  const resourceColumns: TableColumn<K8sResource>[] = useMemo(
    () => [
      {
        key: 'name',
        header: 'Name',
        cell: (resource) => (
          <button
            type="button"
            className="text-left font-medium text-text hover:text-brand"
            onClick={() => selectResource(resource)}
            aria-label={`Inspect ${resource.kind.toLowerCase()} ${resource.name}`}
          >
            {resource.name}
          </button>
        ),
      },
      { key: 'ns', header: 'Namespace', cell: (r) => <span className="text-muted">{r.namespace}</span> },
      { key: 'kind', header: 'Kind', cell: (r) => <span className="text-muted">{r.kind}</span> },
      {
        key: 'status',
        header: 'Status',
        cell: (resource) => (
          <span
            className={`inline-flex items-center rounded px-2 py-0.5 text-caption font-medium border ${
              resource.status === 'Active' || resource.status === 'Running'
                ? 'border-success/20 bg-success/10 text-success'
                : 'border-border bg-surface-2 text-muted'
            }`}
          >
            {resource.status ?? 'Active'}
          </span>
        ),
      },
      {
        key: 'actions',
        header: <span className="block text-right">Actions</span>,
        headerClassName: 'text-right',
        className: 'text-right',
        cell: (resource) => {
          const canView =
            resource.kind === 'ServiceAccount' ||
            (resource.clusterId &&
              ['Role', 'RoleBinding', 'ClusterRole', 'ClusterRoleBinding'].includes(resource.kind));
          return (
            <div className="flex items-center justify-end gap-2">
              <Button
                variant="secondary"
                size="sm"
                className="!px-2.5 !py-1 !text-caption"
                onClick={() => selectResource(resource)}
              >
                Inspect details
              </Button>
              {canView ? (
                <button
                  type="button"
                  className="text-caption font-medium text-brand hover:underline"
                  onClick={() => handleResourceView(resource)}
                >
                  {resource.kind === 'ServiceAccount' ? 'Open identity' : 'Open cluster'}
                </button>
              ) : (
                <span className="text-caption text-muted">—</span>
              )}
            </div>
          );
        },
      },
    ],
    [handleResourceView, selectResource],
  );

  const sortOptionsPod = useMemo(
    () => [
      { value: 'name_asc', label: 'Sort: Name A-Z' },
      { value: 'namespace_asc', label: 'Sort: Namespace A-Z' },
      { value: 'risk_desc', label: 'Sort: Risk score' },
      { value: 'created_desc', label: 'Sort: Newest first' },
    ],
    [],
  );

  const sortOptionsOther = useMemo(
    () => [
      { value: 'name_asc', label: 'Sort: Name A-Z' },
      { value: 'namespace_asc', label: 'Sort: Namespace A-Z' },
      { value: 'risk_desc', label: 'Sort: Kind Z-A' },
    ],
    [],
  );

  const onNsChange = (v: string) => {
    setPage(1);
    setNamespaceFilter(v);
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      if (v) next.set('namespace', v);
      else next.delete('namespace');
      return next;
    });
  };

  const selectResourcesTab = useCallback(
    (id: TabId) => {
      setActiveTab(id);
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('tab', id);
        return next;
      });
      requestAnimationFrame(() => {
        document.getElementById(`resources-tab-${id}`)?.focus();
      });
    },
    [setSearchParams],
  );

  const namespaceDisabled =
    (!selectedClusterId && activeTab === 'Pod') ||
    activeTab === 'ClusterRole' ||
    activeTab === 'Inventory';

  const refreshClicked = () => {
    void fetchResources();
    void fetchAttackSummary();
    if (selectedResource) {
      setSelectedResource({ ...selectedResource });
    }
  };

  const showPagination =
    activeTab !== 'Inventory' && (activeTab === 'Pod' ? podsTotal > 0 : resources.length > 0);

  return (
    <PageLayout
      title={PAGE_TITLES.resources}
      description="Kubernetes inventory with inline attack-path and permission context. Counts are shown as visible / total so scope is explicit. Fortuna login roles are under Settings → Users."
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <DataFreshness
            updatedAt={dataUpdatedAt}
            loading={loading}
            error={dataError}
            staleAfterMs={intervalMs * 2}
          />
          <Button variant="secondary" onClick={refreshClicked} isLoading={loading}>
            <RefreshCw className="mr-2 h-4 w-4" /> Refresh
          </Button>
        </div>
      }
      toolbar={
        activeTab === 'Inventory' ? undefined : (
          <FilterBar
          embedded
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder:
              activeTab === 'Pod'
                ? 'Search pods (name, namespace, UID, node)'
                : activeTab === 'Bindings'
                  ? 'Filter bindings (name, namespace, kind)…'
                  : 'Search resources…',
          }}
          namespace={{
            value: namespaceFilter,
            onChange: onNsChange,
            options: namespaceSelectOptions,
            disabled: namespaceDisabled,
            title: selectedClusterId
              ? 'Namespaces discovered from synced pods in this cluster'
              : 'Select a cluster to list namespaces',
            emptyLabel: 'All namespaces',
          }}
          sort={{
            value: sortBy,
            onChange: (v) => {
              setPage(1);
              setSortBy(v as typeof sortBy);
            },
            options: activeTab === 'Pod' ? sortOptionsPod : sortOptionsOther,
          }}
          trailing={
            activeTab === 'Bindings' ? (
              <span className="text-caption text-muted max-w-md">
                Namespace filter applies to RoleBindings; ClusterRoleBindings are cluster-scoped.
              </span>
            ) : undefined
          }
        />
        )
      }
    >
      <div className="flex flex-col gap-4">
        <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-5">
          <StatCard
            title="Pods in scope"
            value={attackSummaryLoading || loading ? '…' : `${pods.length.toLocaleString('en-US')} / ${podsTotal.toLocaleString('en-US')}`}
            icon={<Box className="w-5 h-5" />}
            tone="info"
            subtitle="Visible / total pods"
          />
          <StatCard
            title="Attack paths"
            value={attackSummaryLoading ? '…' : attackPathScopeValue}
            icon={<Target className="w-5 h-5" />}
            tone="warning"
            subtitle={selectedResource?.kind === 'Pod' ? 'Source paths / cluster total' : 'Cluster total, matches Attack Analysis'}
          />
          <StatCard
            title="Critical paths"
            value={attackSummaryLoading ? '…' : (attackSummary?.criticalPaths ?? 0).toLocaleString('en-US')}
            icon={<AlertTriangle className="w-5 h-5" />}
            tone="critical"
            subtitle="Cluster total"
          />
          <StatCard
            title="High paths"
            value={attackSummaryLoading ? '…' : (attackSummary?.highPaths ?? 0).toLocaleString('en-US')}
            icon={<Zap className="w-5 h-5" />}
            tone="high"
            subtitle="Cluster total"
          />
          <StatCard
            title="Medium paths"
            value={attackSummaryLoading ? '…' : (attackSummary?.mediumPaths ?? 0).toLocaleString('en-US')}
            icon={<Shield className="w-5 h-5" />}
            tone="medium"
            subtitle="Cluster total"
          />
        </div>
        {attackSummaryError ? (
          <div className="rounded-lg border border-border/70 bg-surface/40 px-3 py-2">
            <AttackPathIssueSurface issue={attackSummaryError} compact />
          </div>
        ) : null}

        <Card variant="primary" className="overflow-hidden p-0 shadow-none" contentClassName="p-0">
          <div className="border-b border-border bg-surface/50">
            <Tabs
              variant="underline"
              className="mb-0"
              ariaLabel="Resource type"
              items={tabs}
              value={activeTab}
              onChange={(id) => selectResourcesTab(id as TabId)}
              getPanelId={(id) => `resources-panel-${id}`}
            />
          </div>

          {activeTab === 'Pod' ? (
            <div role="tabpanel" id="resources-panel-Pod" aria-labelledby="resources-tab-Pod">
              <Table<PodWithRisk>
                columns={podColumns}
                data={pods}
                loading={loading}
                emptyTitle="No pods found"
                emptyDescription="Ensure Core and agents are syncing for the selected scope."
                rowKey={(p) => p.uid}
                scrollClassName="ui-table-scroll"
              />
            </div>
          ) : activeTab === 'Inventory' ? (
            <div role="tabpanel" id="resources-panel-Inventory" aria-labelledby="resources-tab-Inventory">
              <div className="p-4 sm:p-6 space-y-6">
                <div className="text-body text-muted space-y-2">
                  <p>
                    Counts reflect <strong className="text-text">ServiceAccounts, Roles, ClusterRoles, RoleBindings,</strong> and{' '}
                    <strong className="text-text">ClusterRoleBindings</strong> synced from the cluster API into Fortuna. This is not the
                    same as <strong className="text-text">Fortuna dashboard users</strong> (Settings → Users).
                  </p>
                  <p className="text-caption">
                    Scope follows the header cluster selector:{' '}
                    <span className="font-mono text-text">{clusterScope ?? 'All clusters'}</span>
                    {inventoryScoped.length === 1 && clusterScope ? (
                      <>
                        {' '}
                        · <span className="font-medium text-text">{getClusterDisplayName(inventoryScoped[0])}</span>
                      </>
                    ) : null}
                  </p>
                </div>
                <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-5">
                  <StatCard
                    title="Service accounts"
                    value={loading ? '…' : inventoryTotals.sa}
                    icon={<UserCog className="w-5 h-5" />}
                    tone="info"
                    subtitle="Subjects for RoleBindings"
                  />
                  <StatCard
                    title="Roles"
                    value={loading ? '…' : inventoryTotals.role}
                    icon={<Scroll className="w-5 h-5" />}
                    tone="warning"
                    subtitle="Namespace-scoped rules"
                  />
                  <StatCard
                    title="ClusterRoles"
                    value={loading ? '…' : inventoryTotals.clusterRole}
                    icon={<Scroll className="w-5 h-5" />}
                    tone="high"
                    subtitle="Cluster-wide rules"
                  />
                  <StatCard
                    title="RoleBindings"
                    value={loading ? '…' : inventoryTotals.rb}
                    icon={<Key className="w-5 h-5" />}
                    tone="brand"
                    subtitle="Namespace bindings"
                  />
                  <StatCard
                    title="ClusterRoleBindings"
                    value={loading ? '…' : inventoryTotals.crb}
                    icon={<Link2 className="w-5 h-5" />}
                    tone="default"
                    subtitle="Cluster-wide bindings"
                  />
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button type="button" size="sm" variant="secondary" onClick={() => selectResourcesTab('Pod')}>
                    Pods
                  </Button>
                  <Button type="button" size="sm" variant="secondary" onClick={() => selectResourcesTab('ServiceAccount')}>
                    Service accounts
                  </Button>
                  <Button type="button" size="sm" variant="secondary" onClick={() => selectResourcesTab('Role')}>
                    Roles
                  </Button>
                  <Button type="button" size="sm" variant="secondary" onClick={() => selectResourcesTab('ClusterRole')}>
                    ClusterRoles
                  </Button>
                  <Button type="button" size="sm" variant="secondary" onClick={() => selectResourcesTab('Bindings')}>
                    All bindings
                  </Button>
                  <Button type="button" size="sm" variant="secondary" onClick={() => selectResourcesTab('RoleBinding')}>
                    Role bindings only
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <div
              role="tabpanel"
              id={`resources-panel-${activeTab}`}
              aria-labelledby={`resources-tab-${activeTab}`}
            >
              <Table<K8sResource>
                columns={resourceColumns}
                data={paginatedResources as K8sResource[]}
                loading={loading}
                emptyTitle="No resources found"
                emptyDescription="Try adjusting tab, namespace, or search filters."
                rowKey={(r) => `${r.id}-${r.kind}`}
                scrollClassName="ui-table-scroll"
              />
            </div>
          )}

          {showPagination ? (
            <div className="border-t border-border/80 bg-surface/30 px-3 py-3">
              <p className="mb-2 text-caption text-muted">
                {activeTab === 'Pod'
                  ? `${pods.length.toLocaleString('en-US')} visible / ${podsTotal.toLocaleString('en-US')} total pods`
                  : `${paginatedResources.length.toLocaleString('en-US')} visible / ${sortedResources.length.toLocaleString('en-US')} total resources`}
              </p>
              <Pagination
                page={page}
                pageSize={pageSize}
                total={totalForPagination}
                onPageChange={setPage}
                onPageSizeChange={(size) => {
                  setPageSize(size);
                  setPage(1);
                }}
                pageSizeOptions={PAGE_SIZE_OPTIONS}
                itemLabel={activeTab === 'Pod' ? 'pods' : 'resources'}
              />
            </div>
          ) : null}
        </Card>

        {activeTab === 'Pod' && searchTerm.trim() !== debouncedSearch ? (
          <p className="text-caption text-muted">Updating search across inventory (300ms debounce)…</p>
        ) : null}

        <Card variant="secondary" className="overflow-hidden border-border/70 bg-surface/50 shadow-none" contentClassName="p-0">
          <div id="resource-inspector" className="border-b border-border bg-surface/30 px-4 py-3 sm:px-5">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="text-caption font-semibold uppercase tracking-wide text-muted-2">Selected resource</p>
                <h3 className="mt-1 text-body font-semibold text-text">
                  {selectedResource ? (selectedResource.kind === 'Pod' ? selectedResource.pod.name : selectedResource.resource.name) : 'No resource selected'}
                </h3>
                <p className="text-caption text-muted">
                  {selectedResource ? (selectedResource.kind === 'Pod' ? `${selectedResource.pod.namespace} · pod` : `${selectedResource.resource.namespace || 'Cluster scope'} · ${selectedResource.resource.kind}`) : 'Choose a row to inspect attack paths, permissions, and follow-up actions inline.'}
                </p>
              </div>
              {selectedResource ? (
                <Button variant="secondary" size="sm" onClick={clearSelection}>
                  Clear selection
                </Button>
              ) : null}
            </div>
          </div>
          <div className="p-4 sm:p-5">
            {selectedResource ? (
              selectedResource.kind === 'Pod' ? (
                <div className="space-y-4">
                  <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 xl:grid-cols-4">
                    <StatCard
                      title="Source paths"
                      value={selectedAttackPathsLoading ? '…' : `${selectedAttackPaths.length.toLocaleString('en-US')} / ${totalAttackPathCount.toLocaleString('en-US')}`}
                      icon={<Target className="w-5 h-5" />}
                      tone="warning"
                      subtitle="Matches Attack Analysis pod scope"
                    />
                    <StatCard
                      title="Risk score"
                      value={selectedResource.pod.unifiedScore != null || selectedResource.pod.totalScore != null ? Math.round(Number(selectedResource.pod.unifiedScore ?? selectedResource.pod.totalScore ?? 0)).toString() : '—'}
                      icon={<Shield className="w-5 h-5" />}
                      tone="critical"
                      subtitle="Pod risk score"
                    />
                    <StatCard
                      title="Status"
                      value={selectedResource.pod.status ?? '—'}
                      icon={<Box className="w-5 h-5" />}
                      tone="default"
                      subtitle={selectedResource.pod.namespace}
                    />
                    <StatCard
                      title="Chain involvement"
                      value={selectedPodChainPathCount != null ? selectedPodChainPathCount.toLocaleString('en-US') : '—'}
                      icon={<PanelRight className="w-5 h-5" />}
                      tone="brand"
                      subtitle={selectedResource.pod.riskSignals?.summary ?? 'Paths where this pod participates'}
                    />
                  </div>

                  <div className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_18rem]">
                    <div className="space-y-4 rounded-xl border border-border/70 bg-base/30 p-4">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="rounded border border-border bg-surface-2 px-2 py-0.5 text-caption font-medium text-text">
                          {selectedResource.pod.namespace}
                        </span>
                        <span className="rounded border border-border bg-surface-2 px-2 py-0.5 text-caption font-medium text-text">
                          {selectedResource.pod.nodeName ?? 'No node'}
                        </span>
                        <span className={`rounded border px-2 py-0.5 text-caption font-medium ${getPodStatusBadgeClass(selectedResource.pod.status)}`}>
                          {selectedResource.pod.status ?? '—'}
                        </span>
                      </div>
                      <div className="space-y-3">
                        <div>
                          <p className="text-caption text-muted-2">Risk factors</p>
                          <div className="mt-2 flex flex-wrap gap-2">
                            <span className="rounded-full border border-border bg-surface px-2.5 py-1 text-caption text-text">
                              Attack-path impact: {selectedResource.pod.riskSignals?.maxImpact ?? '—'}
                            </span>
                            <span className="rounded-full border border-border bg-surface px-2.5 py-1 text-caption text-text">
                              Entry point: {selectedResource.pod.riskSignals?.isEntryPoint ? 'Yes' : 'No'}
                            </span>
                            <span className="rounded-full border border-border bg-surface px-2.5 py-1 text-caption text-text">
                              Pivot: {selectedResource.pod.riskSignals?.isPivot ? 'Yes' : 'No'}
                            </span>
                          </div>
                          {selectedResource.pod.riskSignals?.summary ? (
                            <p className="mt-3 text-body text-muted leading-relaxed">{selectedResource.pod.riskSignals.summary}</p>
                          ) : null}
                        </div>

                        <div>
                          <div className="flex flex-wrap items-center justify-between gap-2">
                            <p className="text-caption text-muted-2">Source attack paths</p>
                            <button
                              type="button"
                              className="text-caption font-medium text-brand hover:underline"
                              onClick={() => navigate(`/attack-paths?podUid=${encodeURIComponent(selectedResource.pod.uid)}`)}
                            >
                              Open same pod scope in Attack Analysis
                            </button>
                          </div>
                          <div className="mt-2 space-y-2">
                            {selectedAttackPathsLoading ? (
                              <div className="rounded-lg border border-border bg-surface/60 px-3 py-4 text-caption text-muted">Loading attack paths…</div>
                            ) : selectedAttackPathsError ? (
                              <div className="rounded-lg border border-border/70 bg-surface/40 px-3 py-2">
                                <AttackPathIssueSurface issue={selectedAttackPathsError} compact />
                              </div>
                            ) : selectedAttackPaths.length > 0 ? (
                              selectedAttackPaths.slice(0, 8).map((path) => {
                                const pathLevel = deriveUnifiedRiskLevelFromScore(path.total_risk * 10) ?? 'low';
                                return (
                                  <article key={path.path_id ?? path.description} className="rounded-lg border border-border bg-surface/60 px-3 py-3">
                                    <div className="flex flex-wrap items-start justify-between gap-2">
                                      <div className="min-w-0">
                                        <p className="font-mono text-caption text-muted-2">{path.path_id ?? 'path'}</p>
                                        <p className="mt-1 text-body text-text">{path.description}</p>
                                      </div>
                                      <span className={`shrink-0 rounded border px-2 py-0.5 text-caption font-semibold uppercase ${getSeverityBadgeClass(pathLevel)}`}>
                                        {pathLevel}
                                        <span className="ml-1.5 font-mono normal-case opacity-95">{Math.round(path.total_risk * 10)}/100</span>
                                      </span>
                                    </div>
                                    <div className="mt-2 flex flex-wrap gap-2 text-caption text-muted">
                                      <span>Difficulty {Math.round(path.difficulty * 100)}%</span>
                                      <span>Impact {Math.round(path.impact * 100)}%</span>
                                      <span>Length {path.length}</span>
                                    </div>
                                  </article>
                                );
                              })
                            ) : (
                              <div className="rounded-lg border border-dashed border-border bg-surface/40 px-3 py-4 text-caption text-muted">No attack paths returned for this pod.</div>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>

                    <div className="space-y-4 rounded-xl border border-border/70 bg-surface/50 p-4">
                      <div>
                        <p className="text-caption text-muted-2">Recommendations</p>
                        <ul className="mt-2 space-y-2 text-body text-muted">
                          <li>Open the full attack-path workspace when you need graph detail or chain ranking.</li>
                          <li>Review the pod&apos;s service account and bound RBAC before changing access.</li>
                          <li>Use the visible / total counts above to keep selection scope explicit.</li>
                        </ul>
                      </div>

                      <div className="rounded-lg border border-border bg-base/40 p-3">
                        <p className="text-caption text-muted-2">Source detail</p>
                        <dl className="mt-2 space-y-2 text-caption">
                          <div className="flex items-center justify-between gap-3">
                            <dt className="text-muted">UID</dt>
                            <dd className="font-mono text-text break-all text-right">{selectedResource.pod.uid}</dd>
                          </div>
                          <div className="flex items-center justify-between gap-3">
                            <dt className="text-muted">Node</dt>
                            <dd className="font-mono text-text text-right">{selectedResource.pod.nodeName ?? '—'}</dd>
                          </div>
                        </dl>
                      </div>
                    </div>
                  </div>
                </div>
              ) : selectedResource.resource.kind === 'ServiceAccount' ? (
                <div className="space-y-4">
                  <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 xl:grid-cols-4">
                    <StatCard
                      title="Effective rules"
                      value={selectedServiceAccountLoading ? '…' : (selectedServiceAccountPermissions?.effectiveRules?.length ?? 0).toLocaleString('en-US')}
                      icon={<Shield className="w-5 h-5" />}
                      tone="success"
                      subtitle="Permissions granted in scope"
                    />
                    <StatCard
                      title="Role bindings"
                      value={selectedServiceAccountLoading ? '…' : (selectedServiceAccountPermissions?.roleBindings?.length ?? 0).toLocaleString('en-US')}
                      icon={<Link2 className="w-5 h-5" />}
                      tone="brand"
                      subtitle="Namespace bindings"
                    />
                    <StatCard
                      title="Cluster role bindings"
                      value={selectedServiceAccountLoading ? '…' : (selectedServiceAccountPermissions?.clusterRoleBindings?.length ?? 0).toLocaleString('en-US')}
                      icon={<Link2 className="w-5 h-5" />}
                      tone="default"
                      subtitle="Cluster-wide bindings"
                    />
                      <StatCard
                        title="Attack paths"
                        value={attackSummaryLoading ? '…' : attackPathScopeValue}
                        icon={<Target className="w-5 h-5" />}
                        tone="warning"
                        subtitle="Select a pod to compare path load"
                      />
                  </div>

                  <div className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_18rem]">
                    <div className="space-y-4 rounded-xl border border-border/70 bg-base/30 p-4">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="rounded border border-border bg-surface-2 px-2 py-0.5 text-caption font-medium text-text">
                          {selectedResource.resource.namespace || 'Cluster scope'}
                        </span>
                        <span className="rounded border border-border bg-surface-2 px-2 py-0.5 text-caption font-medium text-text">
                          {selectedResource.resource.kind}
                        </span>
                        <span className="rounded border border-border bg-surface-2 px-2 py-0.5 text-caption font-medium text-text">
                          {selectedResource.resource.status ?? 'Active'}
                        </span>
                      </div>
                      <div>
                        <p className="text-caption text-muted-2">Permissions</p>
                        {selectedServiceAccountLoading ? (
                          <div className="mt-2 rounded-lg border border-border bg-surface/60 px-3 py-4 text-caption text-muted">Loading permissions…</div>
                        ) : selectedServiceAccountPermissions?.effectiveRules?.length ? (
                          <div className="mt-2 space-y-2">
                            {selectedServiceAccountPermissions.effectiveRules.slice(0, 6).map((rule, index) => (
                              <div key={index} className="rounded-lg border border-border bg-surface/60 px-3 py-3">
                                <p className="text-caption font-medium text-text">Rule {index + 1}</p>
                                <p className="mt-1 text-caption text-muted break-words">
                                  {(rule.verbs ?? ['*']).join(', ')} on {(rule.resources ?? ['*']).join(', ')}
                                </p>
                              </div>
                            ))}
                          </div>
                        ) : (
                          <div className="mt-2 rounded-lg border border-dashed border-border bg-surface/40 px-3 py-4 text-caption text-muted">No effective rules returned for this service account.</div>
                        )}
                      </div>
                      <div>
                        <p className="text-caption text-muted-2">Binding sources</p>
                        {selectedServiceAccountLoading ? (
                          <div className="mt-2 rounded-lg border border-border bg-surface/60 px-3 py-4 text-caption text-muted">Loading bindings…</div>
                        ) : (selectedServiceAccountPermissions?.roleBindings?.length ?? 0) + (selectedServiceAccountPermissions?.clusterRoleBindings?.length ?? 0) > 0 ? (
                          <div className="mt-2 grid gap-2 md:grid-cols-2">
                            {(selectedServiceAccountPermissions?.roleBindings ?? []).map((row, index) => (
                              <div key={`rb-${index}`} className="rounded-lg border border-border bg-surface/60 px-3 py-2">
                                <p className="text-caption font-semibold text-text">RoleBinding: {objectName(row.roleBinding)}</p>
                                <p className="mt-1 text-caption text-muted">Role: <span className="font-mono text-text">{objectName(row.role)}</span></p>
                              </div>
                            ))}
                            {(selectedServiceAccountPermissions?.clusterRoleBindings ?? []).map((row, index) => (
                              <div key={`crb-${index}`} className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2">
                                <p className="text-caption font-semibold text-amber-100">ClusterRoleBinding: {objectName(row.clusterRoleBinding)}</p>
                                <p className="mt-1 text-caption text-muted">ClusterRole: <span className="font-mono text-text">{objectName(row.clusterRole)}</span></p>
                              </div>
                            ))}
                          </div>
                        ) : (
                          <div className="mt-2 rounded-lg border border-dashed border-border bg-surface/40 px-3 py-4 text-caption text-muted">No RoleBinding or ClusterRoleBinding references this service account.</div>
                        )}
                      </div>
                    </div>

                    <div className="space-y-4 rounded-xl border border-border/70 bg-surface/50 p-4">
                      <div>
                        <p className="text-caption text-muted-2 flex items-center gap-1.5">
                          <Boxes className="h-3.5 w-3.5" aria-hidden /> Workloads using this identity
                        </p>
                        {selectedServiceAccountLoading ? (
                          <div className="mt-2 rounded-lg border border-border bg-base/40 px-3 py-3 text-caption text-muted">Loading linked pods…</div>
                        ) : selectedServiceAccountPods.length > 0 ? (
                          <div className="mt-2 space-y-2">
                            {selectedServiceAccountPods.map((pod) => (
                              <button
                                key={pod.uid}
                                type="button"
                                onClick={() => openPodDetail(pod)}
                                className="block w-full rounded-lg border border-border bg-base/40 px-3 py-2 text-left hover:border-brand/40"
                              >
                                <span className="block font-mono text-caption text-text">{pod.namespace}/{pod.name}</span>
                                <span className="mt-0.5 block break-all font-mono text-meta text-muted-2">{pod.uid}</span>
                              </button>
                            ))}
                          </div>
                        ) : (
                          <div className="mt-2 rounded-lg border border-dashed border-border bg-base/30 px-3 py-3 text-caption text-muted">No linked pods synced for this service account.</div>
                        )}
                      </div>

                      <div>
                        <p className="text-caption text-muted-2">Recommendations</p>
                        <ul className="mt-2 space-y-2 text-body text-muted">
                          <li>Check the listed bindings before changing access or rotating credentials.</li>
                          <li>Compare the service account against pods with inline attack paths to see where access becomes abuse.</li>
                          <li>Keep the visible / total counts explicit when you switch filters or namespaces.</li>
                        </ul>
                      </div>

                      <div className="rounded-lg border border-border bg-base/40 p-3">
                        <p className="text-caption text-muted-2">Source detail</p>
                        <dl className="mt-2 space-y-2 text-caption">
                          <div className="flex items-center justify-between gap-3">
                            <dt className="text-muted">UID</dt>
                            <dd className="font-mono text-text break-all text-right">{selectedResource.resource.id}</dd>
                          </div>
                          <div className="flex items-center justify-between gap-3">
                            <dt className="text-muted">Name</dt>
                            <dd className="font-mono text-text text-right">{selectedResource.resource.name}</dd>
                          </div>
                          <div className="flex items-center justify-between gap-3">
                            <dt className="text-muted">Cluster</dt>
                            <dd className="font-mono text-text text-right">{String(selectedServiceAccountDetail?.clusterId ?? selectedResource.resource.clusterId ?? '—')}</dd>
                          </div>
                        </dl>
                      </div>
                    </div>
                  </div>
                </div>
              ) : ['Role', 'RoleBinding', 'ClusterRole', 'ClusterRoleBinding'].includes(selectedResource.resource.kind) ? (
                <div className="space-y-4">
                  <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 xl:grid-cols-4">
                    <StatCard
                      title="Rules"
                      value={selectedRbacResourceLoading ? '…' : (selectedRbacResourceDetail?.rules?.length ?? 0).toLocaleString('en-US')}
                      icon={<Shield className="w-5 h-5" />}
                      tone="success"
                      subtitle="Resolved Kubernetes RBAC rules"
                    />
                    <StatCard
                      title="Subjects"
                      value={selectedRbacResourceLoading ? '…' : (selectedRbacResourceDetail?.subjects?.length ?? 0).toLocaleString('en-US')}
                      icon={<UserCog className="w-5 h-5" />}
                      tone="brand"
                      subtitle="Binding subjects"
                    />
                    <StatCard
                      title="Role refs"
                      value={selectedRbacResourceLoading ? '…' : (selectedRbacResourceDetail?.roleRef ? 1 : 0).toLocaleString('en-US')}
                      icon={<Key className="w-5 h-5" />}
                      tone="default"
                      subtitle="Binding target role"
                    />
                    <StatCard
                      title="Referencing bindings"
                      value={selectedRbacResourceLoading
                        ? '…'
                        : (
                          (selectedRbacResourceDetail?.referencedBy?.length ?? 0) +
                          (selectedRbacResourceDetail?.roleBindings?.length ?? 0) +
                          (selectedRbacResourceDetail?.clusterRoleBindings?.length ?? 0)
                        ).toLocaleString('en-US')}
                      icon={<Link2 className="w-5 h-5" />}
                      tone="warning"
                      subtitle="Bindings pointing here"
                    />
                  </div>

                  <div className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_20rem]">
                    <div className="space-y-4 rounded-xl border border-border/70 bg-base/30 p-4">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="rounded border border-border bg-surface-2 px-2 py-0.5 text-caption font-medium text-text">
                          {selectedResource.resource.namespace || 'Cluster scope'}
                        </span>
                        <span className="rounded border border-border bg-surface-2 px-2 py-0.5 text-caption font-medium text-text">
                          {selectedResource.resource.kind}
                        </span>
                      </div>

                      {(selectedRbacResourceDetail?.roleRef || selectedRbacResourceDetail?.resolvedRole) && (
                        <div>
                          <p className="text-caption text-muted-2">Role reference</p>
                          <div className="mt-2 rounded-lg border border-border bg-surface/60 px-3 py-3">
                            <p className="text-body font-semibold text-text">
                              {stringValue(selectedRbacResourceDetail.roleRef?.kind)}: {stringValue(selectedRbacResourceDetail.roleRef?.name)}
                            </p>
                            <p className="mt-1 text-caption text-muted">
                              Resolved role: <span className="font-mono text-text">{stringValue(selectedRbacResourceDetail.resolvedRole?.name)}</span>
                            </p>
                          </div>
                        </div>
                      )}

                      {selectedRbacResourceDetail?.subjects?.length ? (
                        <div>
                          <p className="text-caption text-muted-2">Subjects</p>
                          <div className="mt-2 grid gap-2 md:grid-cols-2">
                            {selectedRbacResourceDetail.subjects.map((subject, index) => (
                              <div key={index} className="rounded-lg border border-border bg-surface/60 px-3 py-2">
                                <p className="text-caption font-semibold text-text">{stringValue(subject.kind)}: {stringValue(subject.name)}</p>
                                <p className="mt-1 text-caption text-muted">Namespace: <span className="font-mono text-text">{stringValue(subject.namespace)}</span></p>
                              </div>
                            ))}
                          </div>
                        </div>
                      ) : null}

                      <div>
                        <p className="text-caption text-muted-2">Rules</p>
                        {selectedRbacResourceLoading ? (
                          <div className="mt-2 rounded-lg border border-border bg-surface/60 px-3 py-4 text-caption text-muted">Loading RBAC detail…</div>
                        ) : selectedRbacResourceDetail?.rules?.length ? (
                          <div className="mt-2 space-y-2">
                            {selectedRbacResourceDetail.rules.slice(0, 10).map((rule, index) => (
                              <div key={index} className="rounded-lg border border-border bg-surface/60 px-3 py-3">
                                <p className="text-caption font-medium text-text">Rule {index + 1}</p>
                                <p className="mt-1 text-caption text-muted break-words">{ruleSummary(rule)}</p>
                              </div>
                            ))}
                            {selectedRbacResourceDetail.rules.length > 10 ? (
                              <p className="text-caption text-muted">+{selectedRbacResourceDetail.rules.length - 10} more rules</p>
                            ) : null}
                          </div>
                        ) : (
                          <div className="mt-2 rounded-lg border border-dashed border-border bg-surface/40 px-3 py-4 text-caption text-muted">No rules resolved for this resource.</div>
                        )}
                      </div>
                    </div>

                    <div className="space-y-4 rounded-xl border border-border/70 bg-surface/50 p-4">
                      <div>
                        <p className="text-caption text-muted-2">Referencing bindings</p>
                        {selectedRbacResourceLoading ? (
                          <div className="mt-2 rounded-lg border border-border bg-base/40 px-3 py-3 text-caption text-muted">Loading references…</div>
                        ) : (
                          <div className="mt-2 space-y-2">
                            {[
                              ...(selectedRbacResourceDetail?.referencedBy ?? []),
                              ...(selectedRbacResourceDetail?.roleBindings ?? []),
                              ...(selectedRbacResourceDetail?.clusterRoleBindings ?? []),
                            ].slice(0, 8).map((binding, index) => (
                              <div key={index} className="rounded-lg border border-border bg-base/40 px-3 py-2">
                                <p className="font-mono text-caption text-text">{stringValue(binding.name)}</p>
                                <p className="mt-0.5 text-meta text-muted-2">{stringValue(binding.namespace || 'Cluster scope')}</p>
                              </div>
                            ))}
                            {[
                              ...(selectedRbacResourceDetail?.referencedBy ?? []),
                              ...(selectedRbacResourceDetail?.roleBindings ?? []),
                              ...(selectedRbacResourceDetail?.clusterRoleBindings ?? []),
                            ].length === 0 ? (
                              <div className="rounded-lg border border-dashed border-border bg-base/30 px-3 py-3 text-caption text-muted">No synced bindings reference this role.</div>
                            ) : null}
                          </div>
                        )}
                      </div>

                      <div className="rounded-lg border border-border bg-base/40 p-3">
                        <p className="text-caption text-muted-2">Source detail</p>
                        <dl className="mt-2 space-y-2 text-caption">
                          <div className="flex items-center justify-between gap-3">
                            <dt className="text-muted">UID</dt>
                            <dd className="font-mono text-text break-all text-right">{selectedResource.resource.id}</dd>
                          </div>
                          <div className="flex items-center justify-between gap-3">
                            <dt className="text-muted">Name</dt>
                            <dd className="font-mono text-text text-right">{selectedResource.resource.name}</dd>
                          </div>
                          <div className="flex items-center justify-between gap-3">
                            <dt className="text-muted">Cluster</dt>
                            <dd className="font-mono text-text text-right">{stringValue(selectedResource.resource.clusterId)}</dd>
                          </div>
                        </dl>
                      </div>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 xl:grid-cols-4">
                    <StatCard
                      title="Visible resources"
                      value={`${paginatedResources.length.toLocaleString('en-US')} / ${sortedResources.length.toLocaleString('en-US')}`}
                      icon={<LayoutGrid className="w-5 h-5" />}
                      tone="info"
                      subtitle="Current page / filtered total"
                    />
                      <StatCard
                        title="Attack paths"
                        value={attackSummaryLoading ? '…' : attackPathScopeValue}
                        icon={<Target className="w-5 h-5" />}
                        tone="warning"
                        subtitle="Select a pod to inspect source paths"
                      />
                    <StatCard
                      title="Critical paths"
                      value={attackSummaryLoading ? '…' : (attackSummary?.criticalPaths ?? 0).toLocaleString('en-US')}
                      icon={<AlertTriangle className="w-5 h-5" />}
                      tone="critical"
                      subtitle="Cluster total"
                    />
                    <StatCard
                      title="High paths"
                      value={attackSummaryLoading ? '…' : (attackSummary?.highPaths ?? 0).toLocaleString('en-US')}
                      icon={<Zap className="w-5 h-5" />}
                      tone="high"
                      subtitle="Cluster total"
                    />
                  </div>

                  <div className="rounded-lg border border-dashed border-border bg-base/40 p-4 text-caption text-muted">
                    Select a row to inspect attack paths, permissions, and recommendations inline.
                  </div>
                </div>
              )
            ) : (
              <div className="rounded-lg border border-dashed border-border bg-base/40 p-4 text-caption text-muted">
                Select a row to inspect attack paths, permissions, and recommendations inline.
              </div>
            )}
          </div>
        </Card>
      </div>
    </PageLayout>
  );
};
