
import {
  Cluster,
  Insight,
  User,
  K8sResource,
  SecurityRule,
  Certificate,
  Agent,
  QueueMetric,
  Notification,
  RotationEvent,
  AuditLog,
  Report,
  ErrorLog,
  SyncStatus,
  PodSbom,
  PodSbomSummary,
  DashboardStats,
  ThreatVelocityPoint,
  PodCapabilitySummaryCluster,
  PodCapabilitySummaryCapability,
  PodCapabilitySummaryNamespace,
  PodCapabilitySummarySeverity,
  PodCapabilityTrendPoint,
  PodCapabilityDetail,
  CapabilityMetadata,
  PodAttackStep,
  AttackStepSummary,
  PromotionRule,
  RuntimeSignal,
} from '../types';
import { useAuthStore } from '../store/authStore';

const CORE_API_URL = (import.meta as any).env?.VITE_CORE_API_URL || '';
const API_BASE = CORE_API_URL
  ? `${CORE_API_URL.replace(/\/$/, '')}/api/v1`
  : '/api/v1';

const buildUrl = (path: string) => {
  if (path.startsWith('http')) {
    return path;
  }
  return `${API_BASE}${path}`;
};

const getToken = () => useAuthStore.getState().token;

const request = async <T>(path: string, options: RequestInit = {}): Promise<T> => {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> | undefined),
  };
  const token = getToken();
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const res = await fetch(buildUrl(path), {
    ...options,
    headers,
  });

  if (res.status === 401) {
    let message = 'Session expired. Please log in again.';
    try {
      const body = await res.json().catch(() => ({}));
      const err = (body as { error?: string })?.error;
      if (err) {
        message = err === 'Authorization header required' ? 'Please log in.' : err;
      }
    } catch {
      // ignore
    }
    useAuthStore.getState().logout();
    throw new Error(message);
  }

  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json();
};

// REMOVED: All MOCK data constants - replaced with real API calls
// - MOCK_SBOM: getSbomList() uses real API
// - MOCK_CLUSTERS: getClusters() uses real API
// - MOCK_INSIGHTS: getRisks() uses real API
// - MOCK_RESOURCES: getResources() uses real API
// - MOCK_RULES: getRules() uses real API
// - MOCK_CERTS: getCertificates() uses real API
// - MOCK_AGENTS: getAgents() uses real API
// - MOCK_QUEUE_METRICS: getQueueMetrics() uses real API
// - MOCK_NOTIFICATIONS: getNotifications() uses real API
// - MOCK_AUDIT_LOGS: getAuditLogs() uses real API
// - MOCK_REPORTS: getReports() uses real API
// - MOCK_SYNC_STATUS: getSyncStatus() uses real API
// - MOCK_USERS: getUsers() - TODO: implement API endpoint
// - MOCK_ROTATION_HISTORY: getRotationHistory() - TODO: implement API endpoint

export const api = {
  login: async (username: string, password: string): Promise<{ user: User; token: string }> => {
    const resp = await request<{ token: string; user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    return { user: resp.user, token: resp.token };
  },

  getStats: async () => {
    const stats = await request<DashboardStats>('/dashboard/stats');
    const num = (v: unknown) => (typeof v === 'number' ? v : Number(v) || 0);
    return {
      clusters: num(stats.totalClusters),
      insights: num(stats.totalRisks),
      critical: num(stats.criticalRisks),
      pods: num(stats.runningPods),
      agents: num(stats.activeAgents),
      resolved24h: num(stats.resolved24h ?? 0),
    };
  },

  /** GET /api/v1/clusters – active clusters only (cutoff), SSOT from DB */
  getClusters: async (): Promise<Cluster[]> => {
    try {
      const data = await request<{ clusters: Array<Record<string, unknown>> }>('/clusters');
      const list = data.clusters || [];
      return list.map((c: Record<string, unknown>) => {
        const id = String(c.id ?? '');
        const name = String(c.name ?? c.id ?? '').trim() || id;
        const k8sVersion = c.k8sVersion != null ? String(c.k8sVersion) : c.version != null ? String(c.version) : undefined;
        return {
          id,
          name,
          region: c.region != null ? String(c.region) : undefined,
          endpoint: c.endpoint != null ? String(c.endpoint) : undefined,
          status: c.status != null ? String(c.status) : 'unknown',
          lastSync: c.lastSync != null ? String(c.lastSync) : undefined,
          version: k8sVersion,
          k8sVersion,
          source: c.source != null ? String(c.source) : undefined,
          distribution: c.distribution != null ? String(c.distribution) : undefined,
          nodes: typeof c.nodes === 'number' ? c.nodes : undefined,
          pods: typeof c.pods === 'number' ? c.pods : undefined,
          healthScore: (c.status === 'active' ? 90 : c.status === 'inactive' ? 50 : 40) as number,
        } as Cluster;
      });
    } catch (err) {
      return [];
    }
  },

  /** GET /api/v1/clusters/stats – same list + podCount, deploymentCount, connectionStatus (same cutoff as API) */
  getClustersStats: async (): Promise<Cluster[]> => {
    try {
      const data = await request<{ clusters: Array<Record<string, unknown>>; total?: number }>('/clusters/stats');
      const list = data.clusters || [];
      return list.map((c: Record<string, unknown>) => {
        const id = String(c.id ?? '');
        const name = String(c.name ?? c.id ?? '').trim() || id;
        const k8sVersion = c.k8sVersion != null ? String(c.k8sVersion) : c.version != null ? String(c.version) : undefined;
        const connectionStatus = c.connectionStatus != null ? String(c.connectionStatus) : undefined;
        return {
          id,
          name,
          region: c.region != null ? String(c.region) : undefined,
          endpoint: c.endpoint != null ? String(c.endpoint) : undefined,
          status: c.status != null ? String(c.status) : 'unknown',
          lastSync: c.lastSync != null ? String(c.lastSync) : undefined,
          version: k8sVersion,
          k8sVersion,
          source: c.source != null ? String(c.source) : undefined,
          distribution: c.distribution != null ? String(c.distribution) : undefined,
          podCount: typeof c.podCount === 'number' ? c.podCount : undefined,
          deploymentCount: typeof c.deploymentCount === 'number' ? c.deploymentCount : undefined,
          connectionStatus,
          healthScore: connectionStatus === 'connected' ? 90 : connectionStatus === 'degraded' ? 60 : 40,
        } as Cluster;
      });
    } catch (err) {
      return [];
    }
  },

  /** GET /api/v1/risks – default type=vulnerability so count matches dashboard/stats (totalRisks). Optional pagination, severity, search. */
  getRisks: async (params?: { page?: number; pageSize?: number; type?: string; severity?: string; status?: string; search?: string }): Promise<{ insights: Insight[]; total: number; page: number; pageSize: number }> => {
    try {
      const query = new URLSearchParams();
      query.set('type', params?.type ?? 'vulnerability');
      if (params?.page != null) query.set('page', String(params.page));
      if (params?.pageSize != null) query.set('pageSize', String(params.pageSize));
      if (params?.severity) query.set('severity', params.severity);
      if (params?.status) query.set('status', params.status);
      if (params?.search?.trim()) query.set('search', params.search.trim());
      const qs = query.toString();
      const url = qs ? `/risks?${qs}` : '/risks';
      const data = await request<{ total: number; page?: number; pageSize?: number; insights: any[] }>(url);
      const insights = (data.insights || []).map((insight) => {
        const severity = (insight.severity || 'medium').toLowerCase();
        const severityScore: Record<string, number> = {
          critical: 9,
          high: 7,
          medium: 5,
          low: 3,
        };
        const cvss = insight.cvss || severityScore[severity] || 5;
        return {
          id: insight.cveId || String(insight.id),
          title: insight.title,
          description: insight.description,
          severity,
          score: Math.round(cvss * 10),
          category: insight.insightType === 'vulnerability' ? 'sbom' : 'security',
          status: insight.status === 'active' ? 'new' : insight.status === 'resolved' ? 'resolved' : 'acknowledged',
          timestamp: insight.detectedAt || insight.createdAt,
          clusterId: (insight.clusterId ?? insight.resourceNamespace) ?? '',
          clusterName: (insight.clusterName ?? insight.resourceNamespace) ?? '',
          affectedResources: [
            {
              id: insight.resourceUid || String(insight.id),
              name: insight.resourceName,
              kind: insight.resourceType,
              namespace: insight.resourceNamespace,
            },
          ],
          impact: insight.recommendation,
        } as Insight;
      });
      return {
        insights,
        total: Number(data.total) ?? 0,
        page: Number(data.page) ?? 1,
        pageSize: Number(data.pageSize) ?? 20,
      };
    } catch (err) {
      return { insights: [], total: 0, page: 1, pageSize: 20 };
    }
  },

  getResources: async (type?: string): Promise<K8sResource[]> => {
    try {
      const kind = type ? `?kind=${encodeURIComponent(type)}` : '';
      const data = await request<{ resources: Array<{ kind: string; name: string; namespace?: string; uid: string; clusterId: string }> }>(`/resources${kind}`);
      return (data.resources || []).map((res) => ({
        id: res.uid,
        name: res.name,
        namespace: res.namespace || '-',
        kind: res.kind as K8sResource['kind'],
        age: '-',
        status: 'Active',
      }));
    } catch (err) {
      return [];
    }
  },

  getRules: async (): Promise<SecurityRule[]> => {
    try {
      const data = await request<{ rules: SecurityRule[] }>('/rules');
      return data.rules || [];
    } catch (err) {
      return [];
    }
  },

  getCertificates: async (): Promise<Certificate[]> => {
    try {
      const data = await request<{ certificates?: Certificate[] }>('/certificates/info');
      if (Array.isArray(data)) {
        return data as unknown as Certificate[];
      }
      return data.certificates || [];
    } catch (err) {
      return [];
    }
  },

  getAgents: async (): Promise<Agent[]> => {
    try {
      const data = await request<{ agents: Agent[] }>('/agents/status');
      return data.agents || [];
    } catch (err) {
      return [];
    }
  },

  getQueueMetrics: async (): Promise<QueueMetric[]> => {
    try {
      const data = await request<{ _dataSource?: string; normalizer?: number; correlator?: number; risk?: number; timestamp?: string }>('/metrics/queue');
      if (data._dataSource === 'unsupported') return [];
      return [{
        timestamp: data.timestamp ?? '',
        normalizer: data.normalizer ?? 0,
        correlator: data.correlator ?? 0,
        risk: data.risk ?? 0,
      }];
    } catch (err) {
      return [];
    }
  },

  getUsers: async (): Promise<User[]> => {
    // TODO: Implement API endpoint /users when available
    // For now, return empty array - no mock data
    try {
      const data = await request<{ users: User[] }>('/users');
      return data.users || [];
    } catch (err) {
      // API endpoint not available yet, return empty array
      return [];
    }
  },

  getNotifications: async (): Promise<Notification[]> => {
    try {
      const data = await request<{ notifications: Notification[] }>('/notifications');
      return data.notifications || [];
    } catch (err) {
      return [];
    }
  },
  
  getRotationHistory: async (): Promise<RotationEvent[]> => {
    // TODO: Implement API endpoint /certificates/rotation/history when available
    // For now, return empty array - no mock data
    try {
      const data = await request<{ history: RotationEvent[] }>('/certificates/rotation/history');
      return data.history || [];
    } catch (err) {
      // API endpoint not available yet, return empty array
      return [];
    }
  },
  
  getAuditLogs: async (params?: { page?: number; pageSize?: number }): Promise<{ logs: AuditLog[]; total: number; page: number; pageSize: number }> => {
    try {
      const query = new URLSearchParams();
      if (params?.page != null) query.set('page', String(params.page));
      if (params?.pageSize != null) query.set('pageSize', String(params.pageSize));
      const qs = query.toString();
      const url = qs ? `/audit?${qs}` : '/audit';
      const data = await request<{ logs: AuditLog[]; total?: number; page?: number; pageSize?: number }>(url);
      return {
        logs: data.logs || [],
        total: Number(data.total) ?? 0,
        page: Number(data.page) ?? 1,
        pageSize: Number(data.pageSize) ?? 50,
      };
    } catch (err) {
      return { logs: [], total: 0, page: 1, pageSize: 50 };
    }
  },
  
  getReports: async (): Promise<Report[]> => {
    try {
      const data = await request<{ reports: Report[] }>('/reports');
      return data.reports || [];
    } catch (err) {
      return [];
    }
  },

  getErrorLogs: async (): Promise<ErrorLog[]> => {
    try {
      const data = await request<{ _dataSource?: string; logs?: ErrorLog[] }>('/error-logs');
      if (data._dataSource === 'unsupported') return [];
      return data.logs ?? [];
    } catch (err) {
      return [];
    }
  },

  getSyncStatus: async (): Promise<SyncStatus> => {
    try {
      const data = await request<{ health: { status: string }, sync: { lastFullScan: string, nextScan: string }, resources: { pods: number, serviceAccounts: number, roles?: number, bindings?: number } }>('/metrics/system');
      return {
        lastScan: data.sync.lastFullScan,
        nextScan: data.sync.nextScan,
        drift: false,
        resources: {
          pods: data.resources.pods,
          sas: data.resources.serviceAccounts,
          roles: data.resources.roles || 0,
          bindings: data.resources.bindings || 0,
        }
      };
    } catch (err) {
      return {
        lastScan: '',
        nextScan: '',
        drift: false,
        resources: { pods: 0, sas: 0, roles: 0, bindings: 0 },
      };
    }
  },

  getSbomList: async (params?: { podName?: string; namespace?: string }): Promise<PodSbomSummary[]> => {
    try {
      const query = new URLSearchParams();
      query.set('limit', '100');
      if (params?.podName?.trim()) query.set('podName', params.podName.trim());
      if (params?.namespace?.trim()) query.set('namespace', params.namespace.trim());
      const qs = query.toString();
      const url = `/sbom?${qs}`;
      const data = await request<{ sboms: PodSbomSummary[] }>(url);
      const list = data.sboms || [];
      return list.map((s: Record<string, unknown>) => ({
        ...s,
        lastScan: s.lastScan != null ? String(s.lastScan) : '',
        podCreatedAt: s.podCreatedAt != null ? String(s.podCreatedAt) : undefined,
        podStatus: s.podStatus != null ? String(s.podStatus) : undefined,
      })) as PodSbomSummary[];
    } catch (err) {
      return [];
    }
  },

  getPodSbom: async (podId: string): Promise<PodSbom | undefined> => {
    try {
      return await request<PodSbom>(`/sbom/${podId}`);
    } catch (err) {
      return undefined;
    }
  },

  getThreatVelocity: async (days = 7): Promise<ThreatVelocityPoint[]> => {
    try {
      const data = await request<{ trend: ThreatVelocityPoint[] }>(`/dashboard/metrics/threat-velocity?days=${days}`);
      return data.trend;
    } catch (err) {
      return [];
    }
  },

  getPceSummaryByCluster: async (): Promise<PodCapabilitySummaryCluster[]> => {
    try {
      const data = await request<{ summary: PodCapabilitySummaryCluster[] }>('/pod-capabilities/summary/cluster');
      return data.summary || [];
    } catch (err) {
      return [];
    }
  },

  getPceSummaryByCapability: async (): Promise<PodCapabilitySummaryCapability[]> => {
    try {
      const data = await request<{ summary: PodCapabilitySummaryCapability[] }>('/pod-capabilities/summary/capability');
      return data.summary || [];
    } catch (err) {
      return [];
    }
  },

  getPceSummaryByNamespace: async (params?: { namespace?: string; severity?: string; capabilityId?: string }): Promise<PodCapabilitySummaryNamespace[]> => {
    try {
      const query = new URLSearchParams();
      if (params?.namespace) query.set('namespace', params.namespace);
      if (params?.severity) query.set('severity', params.severity);
      if (params?.capabilityId) query.set('capabilityId', params.capabilityId);
      const suffix = query.toString() ? `?${query.toString()}` : '';
      const data = await request<{ summary: PodCapabilitySummaryNamespace[] }>(`/pod-capabilities/summary/namespace${suffix}`);
      return data.summary || [];
    } catch (err) {
      return [];
    }
  },

  getPceSummaryBySeverity: async (params?: { severity?: string; capabilityId?: string }): Promise<PodCapabilitySummarySeverity[]> => {
    try {
      const query = new URLSearchParams();
      if (params?.severity) query.set('severity', params.severity);
      if (params?.capabilityId) query.set('capabilityId', params.capabilityId);
      const suffix = query.toString() ? `?${query.toString()}` : '';
      const data = await request<{ summary: PodCapabilitySummarySeverity[] }>(`/pod-capabilities/summary/severity${suffix}`);
      return data.summary || [];
    } catch (err) {
      return [];
    }
  },

  getPceTrend: async (days = 7, params?: { namespace?: string; capabilityId?: string; podUid?: string; clusterId?: string }): Promise<PodCapabilityTrendPoint[]> => {
    try {
      const query = new URLSearchParams({ days: String(days) });
      if (params?.namespace) query.set('namespace', params.namespace);
      if (params?.capabilityId) query.set('capabilityId', params.capabilityId);
      if (params?.podUid) query.set('podUid', params.podUid);
      if (params?.clusterId) query.set('clusterId', params.clusterId);
      const data = await request<{ points: PodCapabilityTrendPoint[] }>(`/pod-capabilities/trends?${query.toString()}`);
      return data.points || [];
    } catch (err) {
      return [];
    }
  },

  getPceCapabilities: async (params?: { clusterId?: string; namespace?: string; capabilityId?: string; severity?: string; podName?: string; limit?: number; offset?: number }): Promise<PodCapabilityDetail[]> => {
    try {
      const query = new URLSearchParams();
      if (params?.clusterId) query.set('clusterId', params.clusterId);
      if (params?.namespace) query.set('namespace', params.namespace);
      if (params?.capabilityId) query.set('capabilityId', params.capabilityId);
      if (params?.severity) query.set('severity', params.severity);
      if (params?.podName) query.set('podName', params.podName);
      if (params?.limit) query.set('limit', String(params.limit));
      if (params?.offset) query.set('offset', String(params.offset));
      const suffix = query.toString() ? `?${query.toString()}` : '';
      const data = await request<{ capabilities: Array<PodCapabilityDetail & { pod_name?: string }> }>(`/pod-capabilities${suffix}`);
      const raw = data.capabilities || [];
      return raw.map((c) => ({
        ...c,
        podName: c.podName ?? (c as any).pod_name ?? undefined,
      })) as PodCapabilityDetail[];
    } catch (err) {
      return [];
    }
  },
  getPodCapabilities: async (podUid: string): Promise<PodCapabilityDetail[]> => {
    if (!podUid) return [];
    try {
      const data = await request<{ capabilities: PodCapabilityDetail[] }>(`/pods/${podUid}/capabilities`);
      return data.capabilities || [];
    } catch (err) {
      return [];
    }
  },

  // Phase 2.2: Capability Metadata
  getCapabilityMetadata: async (): Promise<CapabilityMetadata[]> => {
    try {
      const data = await request<{ metadata: CapabilityMetadata[]; count: number }>('/capability-metadata');
      return data.metadata || [];
    } catch (err) {
      return [];
    }
  },

  getCapabilityMetadataById: async (capabilityId: string): Promise<CapabilityMetadata | null> => {
    try {
      const data = await request<CapabilityMetadata>(`/capability-metadata/${capabilityId}`);
      return data;
    } catch (err) {
      return null;
    }
  },

  // Phase 2.2: Attack Steps
  getPodAttackSteps: async (podUid: string): Promise<PodAttackStep[]> => {
    if (!podUid) return [];
    try {
      const data = await request<{ podUid: string; steps: PodAttackStep[]; count: number }>(`/attack-steps/pods/${podUid}`);
      return data.steps || [];
    } catch (err) {
      return [];
    }
  },

  getAttackStepsSummary: async (): Promise<AttackStepSummary[]> => {
    try {
      const data = await request<{ summary: AttackStepSummary[]; count: number }>('/attack-steps/summary');
      return data.summary || [];
    } catch (err) {
      return [];
    }
  },

  // Phase 2.3: Promotion Rules
  getPromotionRules: async (): Promise<PromotionRule[]> => {
    try {
      const data = await request<{ rules: PromotionRule[]; count: number }>('/promotion-rules');
      return data.rules || [];
    } catch (err) {
      return [];
    }
  },

  getPromotionRulesByCapability: async (capabilityId: string): Promise<PromotionRule[]> => {
    try {
      const data = await request<{ capabilityId: string; rules: PromotionRule[]; count: number }>(`/promotion-rules/capability/${capabilityId}`);
      return data.rules || [];
    } catch (err) {
      return [];
    }
  },

  getPromotionRulesBySignalType: async (signalType: string): Promise<PromotionRule[]> => {
    try {
      const data = await request<{ signalType: string; rules: PromotionRule[]; count: number }>(`/promotion-rules/signal/${signalType}`);
      return data.rules || [];
    } catch (err) {
      return [];
    }
  },

  // Phase 2.3: Runtime Signals
  getRuntimeSignals: async (params?: { podUid?: string; signalType?: string; category?: string; startDate?: string; endDate?: string; limit?: number; offset?: number }): Promise<{ signals: RuntimeSignal[]; count: number; total: number }> => {
    try {
      const queryParams = new URLSearchParams();
      if (params?.podUid) queryParams.append('podUid', params.podUid);
      if (params?.signalType) queryParams.append('signalType', params.signalType);
      if (params?.category) queryParams.append('category', params.category);
      if (params?.startDate) queryParams.append('startDate', params.startDate);
      if (params?.endDate) queryParams.append('endDate', params.endDate);
      if (params?.limit) queryParams.append('limit', params.limit.toString());
      if (params?.offset) queryParams.append('offset', params.offset.toString());
      
      const query = queryParams.toString();
      const url = query ? `/runtime-signals?${query}` : '/runtime-signals';
      const data = await request<{ signals: RuntimeSignal[]; count: number; total: number; limit: number; offset: number }>(url);
      return { signals: data.signals || [], count: data.count || 0, total: data.total || 0 };
    } catch (err) {
      return { signals: [], count: 0, total: 0 };
    }
  },

  getRuntimeSignalsByPod: async (podUid: string, params?: { signalType?: string; category?: string; limit?: number }): Promise<RuntimeSignal[]> => {
    try {
      const queryParams = new URLSearchParams();
      if (params?.signalType) queryParams.append('signalType', params.signalType);
      if (params?.category) queryParams.append('category', params.category);
      if (params?.limit) queryParams.append('limit', params.limit.toString());
      
      const query = queryParams.toString();
      const url = query ? `/runtime-signals/pods/${podUid}?${query}` : `/runtime-signals/pods/${podUid}`;
      const data = await request<{ podUid: string; signals: RuntimeSignal[]; count: number }>(url);
      return data.signals || [];
    } catch (err) {
      return [];
    }
  },

  // Attack Paths Graph
  getAttackPathsGraph: async (): Promise<{ nodes: Array<{ id: string; label: string; type: string; risk?: string }>; links: Array<{ source: string; target: string; value: number }> }> => {
    try {
      const data = await request<{ data: { nodes: any[]; links: any[] } }>('/attack-paths/graph');
      // Transform API response to match component format
      const nodes = (data.data?.nodes || []).map((node: any) => ({
        id: node.id || node.uid || String(node.name),
        label: node.name || node.label || node.id,
        type: node.kind?.toLowerCase() || node.type || 'pod',
        risk: node.riskLevel || node.risk || 'low',
      }));
      const links = (data.data?.links || []).map((link: any) => ({
        source: link.source?.id || link.source || String(link.from),
        target: link.target?.id || link.target || String(link.to),
        value: link.weight || link.value || 1,
      }));
      return { nodes, links };
    } catch (err) {
      // If API fails, return empty graph (no mock data)
      return { nodes: [], links: [] };
    }
  },
};
