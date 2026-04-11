
import {
  Cluster,
  ClusterOverview,
  ClusterInventory,
  ClusterAgent,
  ClusterSecuritySummary,
  NodeDetailResponse,
  PodWithRisk,
  Insight,
  User,
  K8sResource,
  SecurityRule,
  Certificate,
  Agent,
  Notification,
  RotationEvent,
  AuditLog,
  Report,
  ErrorLog,
  SyncStatus,
  PodSbom,
  PodSbomSummary,
  DashboardStats,
  InsightsSummary,
  InsightsSummaryByClusterItem,
  ThreatVelocityPoint,
  RiskHistogramResponse,
  PodCapabilitySummaryCluster,
  PodCapabilitySummaryCapability,
  PodCapabilitySummaryNamespace,
  PodCapabilitySummarySeverity,
  PodCapabilityTrendPoint,
  PodCapabilityDetail,
  CapabilityMetadata,
  PodAttackStep,
  AttackStepSummary,
  AttackPath,
  AttackPathSummary,
  AttackPathGraphData,
  PromotionRule,
  RuntimeSignal,
  RuntimeSignalSuppressionStats,
  PodRuntimeMetric,
  PodProcessItem,
  PodNetworkConnectionItem,
  PodNetworkTopDestinationItem,
  NetworkActivityDestinationRow,
  NetworkActivityTalkerRow,
  NetworkActivityWorkloadRow,
  NetworkActivityConnectionRow,
  PodK8sEventItem,
  PodRuntimeSecurityEvent,
  PodRuntimeBehaviorFact,
  PodRuntimeIncident,
  RiskRuleItem,
  RiskRuleFull,
  PodRiskReportSummary,
  ThreatSummary,
} from '../types';
import { useAuthStore } from '../store/authStore';

const CORE_API_URL = (import.meta as any).env?.VITE_CORE_API_URL || '';
const API_BASE = CORE_API_URL
  ? `${CORE_API_URL.replace(/\/$/, '')}/api/v1`
  : '/api/v1';
const API_V2_BASE = CORE_API_URL
  ? `${CORE_API_URL.replace(/\/$/, '')}/api/v2`
  : '/api/v2';

const buildUrl = (path: string) => {
  if (path.startsWith('http')) {
    return path;
  }
  return `${API_BASE}${path}`;
};
const buildUrlV2 = (path: string) => `${API_V2_BASE}${path}`;

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
    const text = await res.text().catch(() => '');
    throw new Error(getErrorMessage(res.status, text || undefined));
  }
  return res.json();
};

const requestV2 = async <T>(path: string, options: RequestInit = {}): Promise<T> => {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> | undefined),
  };
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(buildUrlV2(path), { ...options, headers });
  if (res.status === 401) {
    useAuthStore.getState().logout();
    throw new Error('Session expired. Please log in again.');
  }
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(getErrorMessage(res.status, text || undefined));
  }
  return res.json();
};

/** Fetch response as text (e.g. for YAML). Uses same auth as request(). */
const requestText = async (path: string): Promise<string> => {
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(buildUrl(path), { headers });
  if (res.status === 401) {
    useAuthStore.getState().logout();
    throw new Error('Session expired. Please log in again.');
  }
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(getErrorMessage(res.status, text || undefined));
  }
  return res.text();
};

/** Safe message for non-OK responses (no raw body to avoid leaking details). */
function getErrorMessage(status: number, bodyText?: string): string {
  if (status === 401) return 'Session expired. Please log in again.';
  if (status === 403) return 'Access denied.';
  if (status >= 500) return 'Something went wrong. Please try again.';
  if (bodyText) {
    try {
      const o = JSON.parse(bodyText) as { error?: string };
      if (typeof o?.error === 'string' && o.error.length < 150) return o.error;
    } catch {
      // ignore
    }
  }
  return `Request failed: ${status}`;
}

/** Map API pod object to PodWithRisk (includes POD_DETAIL_SPEC: podIP, startTime, restartCount, owner*, qosClass). */
function mapApiPodToPodWithRisk(p: Record<string, unknown>): PodWithRisk {
  const phase = (p as any).phase != null && String((p as any).phase).trim() ? String((p as any).phase).trim() : undefined;
  return {
    id: Number(p.id ?? 0),
    clusterId: String(p.clusterId ?? ''),
    name: String(p.name ?? ''),
    namespace: String(p.namespace ?? ''),
    uid: String(p.uid ?? ''),
    nodeName: p.nodeName != null ? String(p.nodeName) : undefined,
    serviceAccount: p.serviceAccount != null ? String(p.serviceAccount) : undefined,
    riskCount: Number((p as any).riskCount ?? 0),
    status: phase,
    phase: phase,
    createdAt: p.createdAt != null ? String(p.createdAt) : undefined,
    podIP: p.podIP != null && p.podIP !== '' ? String(p.podIP) : undefined,
    startTime: p.startTime != null ? String(p.startTime) : undefined,
    restartCount: typeof p.restartCount === 'number' ? p.restartCount : undefined,
    ownerKind: p.ownerKind != null && p.ownerKind !== '' ? String(p.ownerKind) : undefined,
    ownerName: p.ownerName != null && p.ownerName !== '' ? String(p.ownerName) : undefined,
    replicaSetName: p.replicaSetName != null && p.replicaSetName !== '' ? String(p.replicaSetName) : undefined,
    qosClass: p.qosClass != null && p.qosClass !== '' ? String(p.qosClass) : undefined,
  };
}

// REMOVED: All MOCK data constants - replaced with real API calls
// - MOCK_SBOM: getSbomList() uses real API
// - MOCK_CLUSTERS: getClusters() uses real API
// - MOCK_INSIGHTS: getRisks() uses real API
// - MOCK_RESOURCES: getResources() uses real API
// - MOCK_RULES: getRules() uses real API
// - MOCK_CERTS: getCertificates() uses real API
// - MOCK_AGENTS: getAgents() uses real API (agents table)
// - MOCK_NOTIFICATIONS: getNotifications() uses real API (notifications table)
// - MOCK_AUDIT_LOGS: getAuditLogs() uses real API
// - MOCK_REPORTS: getReports() uses real API
// - MOCK_SYNC_STATUS: getSyncStatus() uses real API
// - getUsers(): GET /users (from users table; admin only when auth enabled)
// - getRotationHistory(): GET /certificates/rotation/history (stub [] until rotation_history table)

export const api = {
  login: async (username: string, password: string): Promise<{ user: User; token: string }> => {
    const resp = await request<{ token: string; user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    return { user: resp.user, token: resp.token };
  },

  /** GET /api/v1/dashboard/stats – optional clusterId, sinceMinutes, byType ('all' = all insight types; default 'vulnerability'). */
  getStats: async (clusterId?: string | null, sinceMinutes?: number, byType?: 'all' | 'vulnerability') => {
    const params = new URLSearchParams();
    if (clusterId?.trim()) params.set('clusterId', clusterId.trim());
    if (sinceMinutes != null && sinceMinutes > 0) params.set('sinceMinutes', String(sinceMinutes));
    if (byType === 'all') params.set('byType', 'all');
    const qs = params.toString() ? `?${params.toString()}` : '';
    const stats = await request<DashboardStats>(`/dashboard/stats${qs}`);
    const num = (v: unknown) => (typeof v === 'number' ? v : Number(v) || 0);
    return {
      clusters: num(stats.totalClusters),
      insights: num(stats.totalRisks),
      critical: num(stats.criticalRisks),
      pods: num(stats.runningPods),
      agents: num(stats.activeAgents),
      resolved24h: num(stats.resolved24h ?? 0),
      affectedPodCount: num(stats.affectedPodCount ?? 0),
      clusterName: stats.clusterName != null ? String(stats.clusterName).trim() || undefined : undefined,
    };
  },

  /** GET /api/v1/insights/summary – severity breakdown. Optional clusterId, sinceMinutes (time range). */
  getInsightsSummary: async (clusterId?: string | null, sinceMinutes?: number): Promise<InsightsSummary> => {
    const params = new URLSearchParams();
    if (clusterId?.trim()) params.set('clusterId', clusterId.trim());
    if (sinceMinutes != null && sinceMinutes > 0) params.set('sinceMinutes', String(sinceMinutes));
    const qs = params.toString() ? `?${params.toString()}` : '';
    const data = await request<InsightsSummary & Record<string, unknown>>(`/risk/insights/summary${qs}`);
    const num = (v: unknown) => (typeof v === 'number' ? v : Number(v) || 0);
    return {
      total: num(data.total),
      critical: num(data.critical),
      high: num(data.high),
      medium: num(data.medium),
      low: num(data.low),
      byType: data.byType,
    };
  },

  /** GET /api/v1/insights/summary/by-cluster – summary per cluster (for global view). Optional sinceMinutes. */
  getInsightsSummaryByCluster: async (sinceMinutes?: number): Promise<InsightsSummaryByClusterItem[]> => {
    const params = new URLSearchParams();
    if (sinceMinutes != null && sinceMinutes > 0) params.set('sinceMinutes', String(sinceMinutes));
    const qs = params.toString() ? `?${params.toString()}` : '';
    const data = await request<{ byCluster?: InsightsSummaryByClusterItem[] }>(`/risk/insights/summary/by-cluster${qs}`);
    return Array.isArray(data?.byCluster) ? data.byCluster : [];
  },

  /** GET /api/v1/insights/summary/global – global (all-clusters) summary. Optional sinceMinutes. Same shape as getInsightsSummary. */
  getInsightsSummaryGlobal: async (sinceMinutes?: number): Promise<InsightsSummary> => {
    const params = new URLSearchParams();
    if (sinceMinutes != null && sinceMinutes > 0) params.set('sinceMinutes', String(sinceMinutes));
    const qs = params.toString() ? `?${params.toString()}` : '';
    const data = await request<InsightsSummary & Record<string, unknown>>(`/risk/insights/summary/global${qs}`);
    const num = (v: unknown) => (typeof v === 'number' ? v : Number(v) || 0);
    return {
      total: num(data.total),
      critical: num(data.critical),
      high: num(data.high),
      medium: num(data.medium),
      low: num(data.low),
      byType: data.byType,
    };
  },

  /** GET /api/v1/clusters/:id/overview */
  getClusterOverview: async (id: string): Promise<ClusterOverview | null> => {
    try {
      return await request<ClusterOverview>(`/inventory/clusters/${id}/overview`);
    } catch {
      return null;
    }
  },

  /** GET /api/v1/inventory/clusters/:id/inventory */
  getClusterInventory: async (id: string): Promise<ClusterInventory | null> => {
    try {
      return await request<ClusterInventory>(`/inventory/clusters/${id}/inventory`);
    } catch {
      return null;
    }
  },

  /** GET /api/v1/clusters/:id/agents */
  getClusterAgents: async (id: string): Promise<{ agents: ClusterAgent[]; total: number }> => {
    try {
      const data = await request<{ agents: ClusterAgent[]; total: number }>(`/inventory/clusters/${id}/agents`);
      const agents = (data.agents || []).map((a: Record<string, unknown>) => ({
        agentId: String(a.agentId ?? ''),
        nodeName: a.nodeName != null ? String(a.nodeName) : undefined,
        status: String(a.status ?? ''),
        lastHeartbeat: a.lastHeartbeat != null ? String(a.lastHeartbeat) : '',
        version: a.version != null ? String(a.version) : undefined,
      }));
      return { agents, total: Number(data.total) ?? agents.length };
    } catch {
      return { agents: [], total: 0 };
    }
  },

  /** GET /api/v1/clusters/:id/security-summary */
  getClusterSecuritySummary: async (id: string): Promise<ClusterSecuritySummary | null> => {
    try {
      return await request<ClusterSecuritySummary>(`/inventory/clusters/${id}/security-summary`);
    } catch {
      return null;
    }
  },

  /** GET /api/v1/clusters/:id/nodes/:nodeName – Node Detail (metadata + optional ?pods=true for workloads) */
  getClusterNode: async (clusterId: string, nodeName: string, opts?: { pods?: boolean }): Promise<NodeDetailResponse | null> => {
    try {
      const qs = opts?.pods ? '?pods=true' : '';
      const data = await request<NodeDetailResponse>(`/inventory/clusters/${encodeURIComponent(clusterId)}/nodes/${encodeURIComponent(nodeName)}${qs}`);
      return data;
    } catch {
      return null;
    }
  },

  /** GET /api/v1/clusters/:id – single cluster for Cluster Detail */
  getCluster: async (id: string): Promise<Cluster | null> => {
    try {
      const c = await request<Record<string, unknown>>(`/inventory/clusters/${id}`);
      const name = String(c.name ?? c.id ?? '').trim() || String(id);
      return {
        id: String(c.id ?? id),
        name,
        region: c.region != null ? String(c.region) : undefined,
        endpoint: c.endpoint != null ? String(c.endpoint) : undefined,
        status: c.status != null ? String(c.status) : 'unknown',
        lastSync: c.lastSync != null ? String(c.lastSync) : undefined,
        version: c.k8sVersion != null ? String(c.k8sVersion) : c.version != null ? String(c.version) : undefined,
        k8sVersion: c.k8sVersion != null ? String(c.k8sVersion) : undefined,
        source: c.source != null ? String(c.source) : undefined,
        distribution: c.distribution != null ? String(c.distribution) : undefined,
      } as Cluster;
    } catch {
      return null;
    }
  },

  /** GET /api/v1/clusters – active clusters only (cutoff), SSOT from DB */
  getClusters: async (): Promise<Cluster[]> => {
    try {
      const data = await request<{ clusters: Array<Record<string, unknown>> }>('/inventory/clusters');
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
      const data = await request<{ clusters: Array<Record<string, unknown>>; total?: number }>('/inventory/clusters/stats');
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
          riskCount: typeof c.riskCount === 'number' ? c.riskCount : undefined,
          agentCount: typeof c.agentCount === 'number' ? c.agentCount : undefined,
          connectionStatus,
          healthScore: connectionStatus === 'connected' ? 90 : connectionStatus === 'degraded' ? 60 : 40,
        } as Cluster;
      });
    } catch (err) {
      return [];
    }
  },

  /** GET /api/v1/insights/:id – single insight for Risk Detail */
  getInsight: async (id: string): Promise<Insight | null> => {
    try {
      const insight = await request<Record<string, unknown>>(`/risk/insights/${id}`);
      const severity = (insight.severity || 'medium').toLowerCase();
      const cvss = Number(insight.cvss) || (severity === 'critical' ? 9 : severity === 'high' ? 7 : severity === 'medium' ? 5 : 3);
      const insightTypeRaw = (insight.insightType ?? insight.insight_type ?? '') as string;
      return {
        id: String(insight.id ?? id),
        cveId:
          insight.cveId != null
            ? String(insight.cveId)
            : insight.cve_id != null
              ? String(insight.cve_id)
              : undefined,
        insightType: insightTypeRaw ? String(insightTypeRaw) : undefined,
        affectedComponent:
          insight.affectedComponent != null
            ? String(insight.affectedComponent)
            : insight.affected_component != null
              ? String(insight.affected_component)
              : undefined,
        affectedVersion:
          insight.affectedVersion != null
            ? String(insight.affectedVersion)
            : insight.affected_version != null
              ? String(insight.affected_version)
              : undefined,
        title: String(insight.title ?? ''),
        description: insight.description != null ? String(insight.description) : undefined,
        severity,
        score: Math.round(cvss * 10),
        status: insight.status === 'active' ? 'new' : insight.status === 'resolved' ? 'resolved' : 'acknowledged',
        timestamp: (insight.detectedAt ?? insight.createdAt) != null ? String(insight.detectedAt ?? insight.createdAt) : undefined,
        affectedResources: [
          {
            id: String(insight.resourceUid ?? insight.id),
            name: insight.resourceName != null ? String(insight.resourceName) : undefined,
            kind: insight.resourceType != null ? String(insight.resourceType) : undefined,
            namespace: insight.resourceNamespace != null ? String(insight.resourceNamespace) : undefined,
          },
        ],
        impact: insight.recommendation != null ? String(insight.recommendation) : undefined,
      riskExplanation: insight.riskExplanation != null ? String(insight.riskExplanation) : undefined,
      remediation: insight.remediation ?? undefined,
        resolvedAt: insight.resolvedAt != null ? String(insight.resolvedAt) : undefined,
        updatedAt: insight.updatedAt != null ? String(insight.updatedAt) : undefined,
        evidence: insight.evidence,
        violatedRules: insight.violatedRules,
      } as Insight;
    } catch {
      return null;
    }
  },

  /** GET /api/v1/risk/insights/:id/context – cross-resource context for a single insight (pods, cluster, rules). */
  getInsightContext: async (
    id: string,
  ): Promise<{ insight: Record<string, unknown>; pods: any[]; cluster?: any; rules: any[] } | null> => {
    try {
      return await request<{ insight: Record<string, unknown>; pods: any[]; cluster?: any; rules: any[] }>(
        `/risk/insights/${id}/context`,
      );
    } catch {
      return null;
    }
  },

  /** POST /api/v1/insights/:id/resolve – mark insight as resolved */
  resolveInsight: async (id: string, resolution?: string): Promise<void> => {
    await request(`/risk/insights/${id}/resolve`, {
      method: 'POST',
      body: JSON.stringify({ resolution: resolution ?? '' }),
    });
  },

  /** GET /api/v1/risks – defaults to all insight types. Optional type, clusterId, pagination, severity, status, search, namespace, sinceMinutes, withScores (1 = include totalScore/priorityLevel from risk_scores). */
  /** GET /api/v1/risk/histogram – score distribution for Risk Center histogram chart (cached 30s) */
  getRiskHistogram: async (params?: { clusterId?: string | null; sinceMinutes?: number }): Promise<RiskHistogramResponse> => {
    const query = new URLSearchParams();
    if (params?.clusterId?.trim()) query.set('clusterId', params.clusterId.trim());
    const sinceMinutes = params?.sinceMinutes != null && params.sinceMinutes > 0 ? params.sinceMinutes : 30;
    query.set('sinceMinutes', String(sinceMinutes));
    const qs = query.toString();
    const url = `/risk/histogram?${qs}`;
    return request<RiskHistogramResponse>(url);
  },

  /** POST /api/v1/risk/scores/sync – recalculate risk_scores for all resources with active insights (202 Accepted, runs in background) */
  syncRiskScores: async (): Promise<{ message: string; resources: number }> => {
    return request<{ message: string; resources: number }>('/risk/scores/sync', { method: 'POST' });
  },

  /** GET /api/v1/risks – defaults to all insight types. Optional type, clusterId, pagination, severity, status, search, namespace, sinceMinutes, withScores (1 = include totalScore/priorityLevel from risk_scores). scoreBin: 0|10|...|90 to filter by score range [bin, bin+10). */
  getRisks: async (params?: {
    page?: number;
    pageSize?: number;
    type?: string;
    severity?: string;
    status?: string;
    search?: string;
    clusterId?: string | null;
    namespace?: string;
    sinceMinutes?: number;
    withScores?: number;
    priorityLevel?: string;
    scoreBin?: number;
  }): Promise<{ insights: Insight[]; total: number; page: number; pageSize: number }> => {
    try {
      const query = new URLSearchParams();
      if (params?.type) query.set('type', params.type);
      if (params?.page != null) query.set('page', String(params.page));
      if (params?.pageSize != null) query.set('pageSize', String(params.pageSize));
      if (params?.severity) query.set('severity', params.severity);
      if (params?.status != null) query.set('status', params.status);
      if (params?.search?.trim()) query.set('search', params.search.trim());
      if (params?.clusterId?.trim()) query.set('clusterId', params.clusterId.trim());
      if (params?.namespace?.trim()) query.set('resourceNamespace', params.namespace.trim());
      if (params?.sinceMinutes != null && params.sinceMinutes > 0) query.set('sinceMinutes', String(params.sinceMinutes));
      if (params?.withScores === 1) query.set('withScores', '1');
      if (params?.priorityLevel?.trim()) query.set('priorityLevel', params.priorityLevel.trim());
      if (params?.scoreBin != null && params.scoreBin >= 0 && params.scoreBin <= 90 && params.scoreBin % 10 === 0) query.set('scoreBin', String(params.scoreBin));
      const qs = query.toString();
      const url = qs ? `/risk/insights?${qs}` : '/risk/insights';
      const data = await request<{ total: number; page?: number; pageSize?: number; insights: any[] }>(url);
      const insights = (data.insights || []).map((insight) => {
        const severity = (insight.severity || 'medium').toLowerCase();
        const severityScore: Record<string, number> = {
          critical: 9,
          high: 7,
          medium: 5,
          low: 3,
        };
        // Score 0–100: use backend totalScore when withScores=1, else cvss*10 or severity default
        const totalScoreFromApi = insight.totalScore != null ? Number(insight.totalScore) : null;
        const cvss = insight.cvss ?? severityScore[severity] ?? 5;
        const score = totalScoreFromApi != null ? Math.round(totalScoreFromApi) : Math.round(Number(cvss) * 10);
        const exploitabilityScore =
          insight.exploitabilityScore != null ? Number(insight.exploitabilityScore) : undefined;
        const businessImpactScore =
          insight.businessImpactScore != null ? Number(insight.businessImpactScore) : undefined;
        const timeDecay = insight.timeDecay != null ? Number(insight.timeDecay) : undefined;
        const itype = (insight.insightType ?? insight.insight_type ?? 'vulnerability') as string;
        return {
          id: String(insight.id),
          cveId:
            insight.cveId != null
              ? String(insight.cveId)
              : insight.cve_id != null
                ? String(insight.cve_id)
                : undefined,
          affectedComponent:
            insight.affectedComponent != null
              ? String(insight.affectedComponent)
              : insight.affected_component != null
                ? String(insight.affected_component)
                : undefined,
          affectedVersion:
            insight.affectedVersion != null
              ? String(insight.affectedVersion)
              : insight.affected_version != null
                ? String(insight.affected_version)
                : undefined,
          title: insight.title,
          description: insight.description,
          severity,
          score: Math.min(100, Math.max(0, score)),
          category: itype === 'vulnerability' || itype === 'supply_chain_malware' ? 'sbom' : 'security',
          insightType: itype,
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
          evidence: insight.evidence,
          violatedRules: insight.violatedRules,
          totalScore: totalScoreFromApi ?? undefined,
          priorityLevel: insight.priorityLevel ? String(insight.priorityLevel) : undefined,
          exploitabilityScore,
          businessImpactScore,
          timeDecay,
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

  /** GET /api/v1/risks/export – download risks as CSV (same filters as getRisks). Triggers browser download. */
  exportRisksCSV: async (params?: { clusterId?: string | null; severity?: string; status?: string; search?: string; sinceMinutes?: number }): Promise<void> => {
    const query = new URLSearchParams();
    if (params?.clusterId?.trim()) query.set('clusterId', params.clusterId.trim());
    if (params?.severity) query.set('severity', params.severity);
    if (params?.status != null) query.set('status', params.status);
    if (params?.search?.trim()) query.set('search', params.search.trim());
    if (params?.sinceMinutes != null && params.sinceMinutes > 0) query.set('sinceMinutes', String(params.sinceMinutes));
    const qs = query.toString();
    const path = qs ? `/risk/insights/export?${qs}` : '/risk/insights/export';
    const url = buildUrl(path);
    const headers: Record<string, string> = {};
    const token = getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(url, { headers });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `Export failed: ${res.status}`);
    }
    const blob = await res.blob();
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'risks-export.csv';
    a.click();
    URL.revokeObjectURL(a.href);
  },

  /** GET /api/v1/risks/export?format=pdf – download risks as print-optimized HTML (open and use Print → Save as PDF). */
  exportRisksPDF: async (params?: { clusterId?: string | null; severity?: string; status?: string; search?: string; sinceMinutes?: number }): Promise<void> => {
    const query = new URLSearchParams();
    query.set('format', 'pdf');
    if (params?.clusterId?.trim()) query.set('clusterId', params.clusterId.trim());
    if (params?.severity) query.set('severity', params.severity);
    if (params?.status != null) query.set('status', params.status);
    if (params?.search?.trim()) query.set('search', params.search.trim());
    if (params?.sinceMinutes != null && params.sinceMinutes > 0) query.set('sinceMinutes', String(params.sinceMinutes));
    const qs = query.toString();
    const path = `/risk/insights/export?${qs}`;
    const url = buildUrl(path);
    const headers: Record<string, string> = {};
    const token = getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(url, { headers });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || `Export failed: ${res.status}`);
    }
    const blob = await res.blob();
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'risks-export.html';
    a.click();
    URL.revokeObjectURL(a.href);
  },

  /** GET /api/v1/inventory/pods/:uid – single pod by UID (domain route) */
  getPodByUid: async (uid: string): Promise<PodWithRisk | null> => {
    try {
      const p = await request<Record<string, unknown>>(`/inventory/pods/${encodeURIComponent(uid)}`);
      return mapApiPodToPodWithRisk(p);
    } catch {
      return null;
    }
  },

  /** GET /api/v1/inventory/pods/:uid – single pod (uid only; DB id no longer exposed) */
  getPod: async (id: string | number): Promise<PodWithRisk | null> => {
    try {
      const p = await request<Record<string, unknown>>(`/inventory/pods/${encodeURIComponent(String(id))}`);
      return mapApiPodToPodWithRisk(p);
    } catch {
      return null;
    }
  },

  /** WebSocket URL for pod detail live updates (Phase 5.1). Pass uid; token is appended as query for auth. */
  getPodDetailWsUrl: (uid: string): string => {
    const path = `/api/v1/ws/pod/${encodeURIComponent(uid)}`;
    const base = CORE_API_URL ? CORE_API_URL.replace(/\/$/, '') : window.location.origin;
    const protocol = base.startsWith('https') ? 'wss:' : 'ws:';
    const host = base.startsWith('http') ? new URL(base).host : window.location.host;
    const token = getToken();
    const qs = token ? `?token=${encodeURIComponent(token)}` : '';
    return `${protocol}//${host}${path}${qs}`;
  },

  /** WebSocket URL for Risk Center live updates (Phase 2.3). Server pushes insights_updated when data changes. */
  getRisksWsUrl: (): string => {
    const path = '/api/v1/ws/risks';
    const base = CORE_API_URL ? CORE_API_URL.replace(/\/$/, '') : window.location.origin;
    const protocol = base.startsWith('https') ? 'wss:' : 'ws:';
    const host = base.startsWith('http') ? new URL(base).host : window.location.host;
    const token = getToken();
    const qs = token ? `?token=${encodeURIComponent(token)}` : '';
    return `${protocol}//${host}${path}${qs}`;
  },

  /** Runtime domain: pod-scoped APIs use /api/v1/runtime/pods/:uid/... */
  getPodRuntimeMetrics: async (podUid: string): Promise<PodRuntimeMetric[]> => {
    try {
      const data = await request<{ items?: PodRuntimeMetric[] }>(`/runtime/pods/${encodeURIComponent(podUid)}/metrics`);
      return data.items ?? [];
    } catch {
      return [];
    }
  },
  getPodProcesses: async (podUid: string): Promise<PodProcessItem[]> => {
    try {
      const data = await request<{ items?: PodProcessItem[] }>(`/runtime/pods/${encodeURIComponent(podUid)}/processes`);
      return data.items ?? [];
    } catch {
      return [];
    }
  },
  getPodNetworkConnections: async (podUid: string): Promise<PodNetworkConnectionItem[]> => {
    try {
      const data = await request<{ items?: PodNetworkConnectionItem[] }>(`/runtime/pods/${encodeURIComponent(podUid)}/network`);
      return data.items ?? [];
    } catch {
      return [];
    }
  },

  /** Aggregated dest_ip:port/proto for this pod (same time window semantics as GET .../network). */
  getPodNetworkTopDestinations: async (
    podUid: string,
    params?: { sinceMinutes?: number; limit?: number }
  ): Promise<PodNetworkTopDestinationItem[]> => {
    try {
      const q = new URLSearchParams();
      if (params?.sinceMinutes != null && params.sinceMinutes > 0) {
        q.set('sinceMinutes', String(params.sinceMinutes));
      }
      if (params?.limit != null && params.limit > 0) {
        q.set('limit', String(params.limit));
      }
      const qs = q.toString();
      const path = `/runtime/pods/${encodeURIComponent(podUid)}/network/top-destinations${qs ? `?${qs}` : ''}`;
      const data = await request<{ items?: PodNetworkTopDestinationItem[] }>(path);
      return data.items ?? [];
    } catch {
      return [];
    }
  },

  /**
   * Cluster-wide network activity from pod_network_connections (same source as Pod Detail).
   * @param view pods | connections | destinations | talkers | edges (edges = aggregated pod→dest for topology)
   */
  getNetworkActivity: async (params: {
    cluster: string;
    view?: 'pods' | 'connections' | 'destinations' | 'talkers' | 'edges';
    namespace?: string;
    q?: string;
    /** Lọc đúng theo pod nguồn (AND với namespace/q); drill khi thiếu podName. */
    podUid?: string;
    sinceMinutes?: number;
    page?: number;
    pageSize?: number;
  }): Promise<{
    view: string;
    clusterId: string;
    total: number;
    page: number;
    pageSize: number;
    items:
      | NetworkActivityWorkloadRow[]
      | NetworkActivityConnectionRow[]
      | NetworkActivityDestinationRow[]
      | NetworkActivityTalkerRow[];
  }> => {
    const q = new URLSearchParams();
    q.set('cluster', params.cluster);
    q.set('view', params.view ?? 'connections');
    if (params.namespace) q.set('namespace', params.namespace);
    if (params.q) q.set('q', params.q);
    if (params.podUid?.trim()) q.set('podUid', params.podUid.trim());
    if (params.sinceMinutes != null && params.sinceMinutes > 0) {
      q.set('sinceMinutes', String(params.sinceMinutes));
    }
    if (params.page != null) q.set('page', String(params.page));
    if (params.pageSize != null) q.set('pageSize', String(params.pageSize));
    return request(`/runtime/network-activity?${q.toString()}`);
  },
  getPodEvents: async (podUid: string): Promise<PodK8sEventItem[]> => {
    try {
      const data = await request<{ items?: PodK8sEventItem[] }>(`/runtime/pods/${encodeURIComponent(podUid)}/events`);
      return data.items ?? [];
    } catch {
      return [];
    }
  },

  getPodSpecYaml: async (podUid: string): Promise<string> => {
    return requestText(`/inventory/pods/${encodeURIComponent(podUid)}/spec`);
  },

  getPodSpecYamlBlob: async (podUid: string): Promise<Blob> => {
    return requestBlob(`/inventory/pods/${encodeURIComponent(podUid)}/spec?download=1`);
  },

  /** GET /api/v1/inventory/pods – list pods with riskCount */
  getPods: async (params?: { cluster?: string; namespace?: string; node?: string; page?: number; pageSize?: number }): Promise<{ pods: PodWithRisk[]; total: number; page: number; pageSize: number }> => {
    try {
      const q = new URLSearchParams();
      if (params?.cluster) q.set('cluster', params.cluster);
      if (params?.namespace) q.set('namespace', params.namespace);
      if (params?.node) q.set('node', params.node);
      if (params?.page != null) q.set('page', String(params.page));
      if (params?.pageSize != null) q.set('pageSize', String(params.pageSize));
      const qs = q.toString();
      const data = await request<{ pods: Array<Record<string, unknown>>; total: number; page?: number; pageSize?: number }>(qs ? `/inventory/pods?${qs}` : '/inventory/pods');
      const pods = (data.pods || []).map((p: Record<string, unknown>) => mapApiPodToPodWithRisk(p));
      return {
        pods,
        total: Number(data.total) ?? pods.length,
        page: Number(data.page) ?? 1,
        pageSize: Number(data.pageSize) ?? 50,
      };
    } catch {
      return { pods: [], total: 0, page: 1, pageSize: 50 };
    }
  },

  getResources: async (type?: string, params?: { cluster?: string; namespace?: string }): Promise<K8sResource[]> => {
    try {
      const q = new URLSearchParams();
      if (type) q.set('kind', type);
      if (params?.cluster) q.set('cluster', params.cluster);
      if (params?.namespace) q.set('namespace', params.namespace);
      const qs = q.toString();
      const data = await request<{ resources: Array<{ kind: string; name: string; namespace?: string; uid: string; clusterId: string }> }>(`/resources${qs ? `?${qs}` : ''}`);
      return (data.resources || []).map((res) => ({
        id: res.uid,
        name: res.name,
        namespace: res.namespace || '-',
        kind: res.kind as K8sResource['kind'],
        clusterId: res.clusterId || undefined,
        age: '-',
        status: 'Active',
      }));
    } catch (err) {
      return [];
    }
  },

  getRules: async (): Promise<SecurityRule[]> => {
    try {
      const data = await request<{ rules: Array<Record<string, unknown>>; total?: number; active?: number; disabled?: number }>('/policy/rules');
      const raw = data.rules || [];
      return raw.map((r) => ({
        id: String(r.id ?? ''),
        name: String(r.name ?? r.id ?? ''),
        severity: String(r.severity ?? 'medium').toLowerCase(),
        enabled: Boolean(r.enabled),
        category: r.category != null ? String(r.category) : undefined,
        type: r.type != null ? String(r.type) : undefined,
        description: r.description != null ? String(r.description) : undefined,
        logic: r.logic != null ? String(r.logic) : undefined,
        evalTime: r.evalTime != null ? String(r.evalTime) : undefined,
        lastUpdated: r.lastUpdated != null ? String(r.lastUpdated) : undefined,
        matches: r.matches != null ? Number(r.matches) : undefined,
        lastMatchedAt: r.lastMatchedAt != null ? String(r.lastMatchedAt) : undefined,
        source: r.source != null ? String(r.source) : undefined,
        signature: r.signature != null ? String(r.signature) : undefined,
        overlapGroup: r.overlapGroup != null ? String(r.overlapGroup) : undefined,
        isCanonical: r.isCanonical != null ? Boolean(r.isCanonical) : undefined,
        canonicalRuleId: r.canonicalRuleId != null ? String(r.canonicalRuleId) : undefined,
        impactedFindings24h: r.impactedFindings24h != null ? Number(r.impactedFindings24h) : undefined,
        impactedFindings7d: r.impactedFindings7d != null ? Number(r.impactedFindings7d) : undefined,
        relatedCapabilities: Array.isArray(r.relatedCapabilities) ? r.relatedCapabilities.map((x) => String(x)) : undefined,
      })) as SecurityRule[];
    } catch (err) {
      return [];
    }
  },

  /** POST /api/v1/rules/reload */
  reloadRules: async (): Promise<{ reloaded: boolean; count?: number; duration?: string }> => {
    return await request<{ reloaded: boolean; count?: number; duration?: string }>('/policy/rules/reload', {
      method: 'POST',
      body: JSON.stringify({}),
    });
  },

  /** GET /api/v1/rules/:id/metrics */
  getRuleMetrics: async (id: string): Promise<{ totalMatches: number; recentMatches: Insight[] }> => {
    try {
      const data = await request<{ totalMatches?: number; recentMatches?: unknown[] }>(`/policy/rules/${id}/metrics`);
      const recentMatches = (data.recentMatches || []).map((m: unknown) => {
        const x = m as Record<string, unknown>;
        return {
          id: String(x.id ?? ''),
          title: String(x.title ?? ''),
          severity: String(x.severity ?? 'medium').toLowerCase(),
          status: x.status as string,
          timestamp: (x.detectedAt ?? x.createdAt) as string | undefined,
        } as Insight;
      });
      return { totalMatches: Number(data.totalMatches ?? 0), recentMatches };
    } catch {
      return { totalMatches: 0, recentMatches: [] };
    }
  },

  /** POST /api/v1/rules/:id/test */
  testRule: async (id: string, resource: Record<string, unknown>): Promise<{ match: boolean; details?: Record<string, unknown> }> => {
    return await request<{ match: boolean; details?: Record<string, unknown> }>(`/policy/rules/${id}/test`, {
      method: 'POST',
      body: JSON.stringify({ resource }),
    });
  },

  /** GET /api/v1/risk-rules – list from DB (or files when DB empty) */
  getRiskRules: async (): Promise<{ rules: RiskRuleItem[]; total: number; source: string }> => {
    const data = await request<{ rules: RiskRuleItem[]; total: number; source?: string }>('/risk/rules');
    return {
      rules: data.rules ?? [],
      total: data.total ?? 0,
      source: data.source ?? 'db',
    };
  },

  /** GET /api/v1/risk-rules/:id – single rule (DB) */
  getRiskRule: async (id: string): Promise<RiskRuleFull | null> => {
    try {
      return await request<RiskRuleFull>(`/risk/rules/${encodeURIComponent(id)}`);
    } catch {
      return null;
    }
  },

  /** POST /api/v1/risk-rules – create rule (DB). On 400, throws Error with .errors: string[] if present. */
  createRiskRule: async (rule: RiskRuleFull): Promise<RiskRuleFull> => {
    const res = await fetch(buildUrl('/risk/rules'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {}) },
      body: JSON.stringify(rule),
    });
    const text = await res.text();
    if (!res.ok) {
      const err = new Error(res.status === 400 ? 'Validation failed' : getErrorMessage(res.status, text || undefined)) as Error & { errors?: string[] };
      if (res.status === 400) {
        try {
          const data = JSON.parse(text) as { errors?: string[] };
          if (Array.isArray(data.errors)) err.errors = data.errors;
        } catch {
          // ignore
        }
      }
      throw err;
    }
    return JSON.parse(text) as RiskRuleFull;
  },

  /** PUT /api/v1/risk-rules/:id – update rule (DB). On 400, throws Error with .errors: string[] if present. */
  updateRiskRule: async (id: string, rule: RiskRuleFull): Promise<RiskRuleFull> => {
    const res = await fetch(buildUrl(`/risk/rules/${encodeURIComponent(id)}`), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {}) },
      body: JSON.stringify(rule),
    });
    const text = await res.text();
    if (!res.ok) {
      const err = new Error(res.status === 400 ? 'Validation failed' : getErrorMessage(res.status, text || undefined)) as Error & { errors?: string[] };
      if (res.status === 400) {
        try {
          const data = JSON.parse(text) as { errors?: string[] };
          if (Array.isArray(data.errors)) err.errors = data.errors;
        } catch {
          // ignore
        }
      }
      throw err;
    }
    return JSON.parse(text) as RiskRuleFull;
  },

  /** DELETE /api/v1/risk-rules/:id – delete rule (DB) */
  deleteRiskRule: async (id: string): Promise<void> => {
    await request(`/risk/rules/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },

  /** POST /api/v1/risk/rules/validate – validate rule payload (no save). Accepts JSON or YAML. */
  validateRiskRule: async (rule: RiskRuleFull): Promise<{ valid: boolean; errors: string[] }> => {
    const data = await request<{ valid?: boolean; errors?: string[] }>('/risk/rules/validate', {
      method: 'POST',
      body: JSON.stringify(rule),
    });
    return { valid: data.valid === true, errors: data.errors ?? [] };
  },

  /** POST /api/v1/risk/rules/validate – validate rule from YAML (server parses and validates). */
  validateRiskRuleYaml: async (yaml: string): Promise<{ valid: boolean; errors: string[] }> => {
    const res = await fetch(buildUrl('/risk/rules/validate'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-yaml', ...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {}) },
      body: yaml,
    });
    const text = await res.text();
    if (res.status === 401) {
      useAuthStore.getState().logout();
      throw new Error('Session expired.');
    }
    let data: { valid?: boolean; errors?: string[] };
    try {
      data = JSON.parse(text);
    } catch {
      throw new Error(text || 'Validate failed');
    }
    return { valid: data.valid === true, errors: data.errors ?? [] };
  },

  /** POST /api/v1/risk/rules/import – create rule from YAML. Verify first with validateRiskRuleYaml. */
  importRiskRuleYaml: async (yaml: string): Promise<RiskRuleFull> => {
    const res = await fetch(buildUrl('/risk/rules/import'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-yaml', ...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {}) },
      body: yaml,
    });
    const text = await res.text();
    if (res.status === 401) {
      useAuthStore.getState().logout();
      throw new Error('Session expired.');
    }
    if (!res.ok) {
      const err = new Error(res.status === 400 ? 'Validation failed' : getErrorMessage(res.status, text)) as Error & { errors?: string[] };
      if (res.status === 400) {
        try {
          const data = JSON.parse(text) as { errors?: string[] };
          if (Array.isArray(data.errors)) err.errors = data.errors;
        } catch {
          // ignore
        }
      }
      throw err;
    }
    return JSON.parse(text) as RiskRuleFull;
  },

  /** GET /api/v1/risk/rules/export – returns YAML (all rules or one if ?id=). */
  getRiskRulesExportYaml: async (ruleId?: string): Promise<string> => {
    const url = ruleId ? buildUrl(`/risk/rules/export?id=${encodeURIComponent(ruleId)}`) : buildUrl('/risk/rules/export');
    const res = await fetch(url, {
      headers: getToken() ? { Authorization: `Bearer ${getToken()}` } : {},
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(getErrorMessage(res.status, text));
    }
    return res.text();
  },

  /** GET /api/v1/rules/:id – single rule + matchCount + recentMatches */
  getRule: async (id: string): Promise<{ rule: SecurityRule & { description?: string }; matchCount: number; recentMatches: Insight[] } | null> => {
    try {
      const data = await request<{ rule: unknown; source?: string; signature?: string; overlapGroup?: string; isCanonical?: boolean; canonicalRule?: string; impactedFindings24h?: number; impactedFindings7d?: number; relatedCapabilities?: string[]; matchCount?: number; recentMatches?: unknown[] }>(`/policy/rules/${id}`);
      const rule = {
        ...(data.rule as SecurityRule & { description?: string }),
        source: data.source,
        signature: data.signature,
        overlapGroup: data.overlapGroup,
        isCanonical: data.isCanonical,
        canonicalRuleId: data.canonicalRule,
        impactedFindings24h: data.impactedFindings24h,
        impactedFindings7d: data.impactedFindings7d,
        relatedCapabilities: data.relatedCapabilities,
      };
      const recentMatches = (data.recentMatches || []).map((m: unknown) => {
        const x = m as Record<string, unknown>;
        return {
          id: String(x.id ?? ''),
          title: (x.title ?? '') as string,
          description: (x.description ?? '') as string,
          severity: String(x.severity ?? 'medium').toLowerCase(),
          status: x.status as string,
          impact: (x.resourceName ?? x.resourceUid ?? '') as string,
          evidence: x.evidence,
          violatedRules: x.violatedRules,
          timestamp: (x.detectedAt ?? x.createdAt) as string | undefined,
        } as Insight;
      });
      return {
        rule,
        matchCount: Number(data.matchCount) ?? 0,
        recentMatches,
      };
    } catch {
      return null;
    }
  },

  /** GET /api/v1/inventory/serviceaccounts/:uid (uid = K8s ServiceAccount UID) */
  getServiceAccount: async (uid: string): Promise<Record<string, unknown> | null> => {
    try {
      return await request<Record<string, unknown>>(`/inventory/serviceaccounts/${encodeURIComponent(uid)}`);
    } catch {
      return null;
    }
  },

  /** GET /api/v1/inventory/serviceaccounts/:uid */
  getServiceAccountByUid: async (uid: string): Promise<Record<string, unknown> | null> => {
    try {
      return await request<Record<string, unknown>>(`/inventory/serviceaccounts/${encodeURIComponent(uid)}`);
    } catch {
      return null;
    }
  },

  /** GET /api/v1/inventory/serviceaccounts/:uid/permissions */
  getServiceAccountPermissions: async (uid: string): Promise<{ permissions?: Array<Record<string, unknown>> }> => {
    try {
      return await request<{ permissions?: Array<Record<string, unknown>> }>(`/inventory/serviceaccounts/${encodeURIComponent(uid)}/permissions`);
    } catch {
      return { permissions: [] };
    }
  },

  getCertificates: async (): Promise<Certificate[]> => {
    try {
      const data = await request<Record<string, unknown> & { certificates?: Certificate[] }>('/cluster/certificates/info');
      if (Array.isArray(data?.certificates)) {
        return data.certificates;
      }
      // CertManager API returns a single certificate object
      if (data && (data.subject || data.not_after || data.issuer)) {
        const notAfter = String(data.not_after ?? '');
        const days = Number(data.days_until_expiry ?? 0);
        const expired = Boolean(data.is_expired);
        const status: Certificate['status'] = expired ? 'expired' : days <= 30 ? 'warning' : 'valid';
        return [{
          id: String(data.serial_number ?? 'core-cert'),
          name: 'Fortuna Core Certificate',
          subject: String(data.subject ?? ''),
          issuer: String(data.issuer ?? ''),
          serialNumber: String(data.serial_number ?? ''),
          daysRemaining: Number.isFinite(days) ? days : undefined,
          expiryDate: notAfter,
          status,
          usage: Array.isArray(data.dns_names) ? (data.dns_names as string[]) : [],
        }];
      }
      return [];
    } catch (err) {
      return [];
    }
  },

  getAgents: async (): Promise<Agent[]> => {
    try {
      const data = await request<{
        agents: Array<{
          agentId: string;
          nodeName: string;
          status: string;
          lastHeartbeat: string;
        }>;
      }>('/agents/status');
      const raw = data.agents || [];
      return raw.map((a) => ({
        id: a.agentId,
        node: a.nodeName,
        status: a.status === 'healthy' ? 'up' : a.status === 'slow' ? 'slow' : a.status === 'disconnected' ? 'down' : a.status,
        lastHeartbeat: typeof a.lastHeartbeat === 'string' ? a.lastHeartbeat : a.lastHeartbeat ? new Date(a.lastHeartbeat as unknown as string).toISOString() : '',
      }));
    } catch (err) {
      return [];
    }
  },

  getUsers: async (): Promise<User[]> => {
    try {
      const data = await request<{ users: Array<Record<string, unknown>> }>('/users');
      return (data.users || []).map((u) => ({
        id: String(u.id ?? ''),
        username: String(u.username ?? ''),
        name: String(u.username ?? ''),
        email: u.email ? String(u.email) : undefined,
        role: u.role ? String(u.role) : undefined,
        active: typeof u.active === 'boolean' ? u.active : undefined,
        status: typeof u.active === 'boolean' ? (u.active ? 'active' : 'disabled') : undefined,
      }));
    } catch (err) {
      return [];
    }
  },

  getNotifications: async (): Promise<Notification[]> => {
    try {
      const data = await request<{ notifications: Array<Record<string, unknown>> }>('/notifications');
      return (data.notifications || []).map((n) => {
        const severity = String(n.severity ?? 'info').toLowerCase();
        const readAt = n.readAt ? String(n.readAt) : undefined;
        const type = severity === 'critical' ? 'error' : severity === 'warning' ? 'warning' : severity === 'error' ? 'error' : 'info';
        return {
          id: String(n.id ?? ''),
          title: String(n.title ?? ''),
          message: String(n.message ?? ''),
          severity,
          type,
          source: n.source ? String(n.source) : undefined,
          readAt,
          read: Boolean(readAt),
          timestamp: n.timestamp ? String(n.timestamp) : undefined,
        } as Notification;
      });
    } catch (err) {
      return [];
    }
  },
  
  getRotationHistory: async (): Promise<RotationEvent[]> => {
    try {
      const data = await request<{ history: Array<Record<string, unknown>> }>('/cluster/certificates/rotation/history');
      return (data.history || []).map((h) => ({
        id: String(h.id ?? ''),
        timestamp: String(h.timestamp ?? h.time ?? ''),
        success: Boolean(h.success),
      }));
    } catch (err) {
      return [];
    }
  },

  rotateCertificate: async (): Promise<{ message?: string }> => {
    return await request<{ message?: string }>('/cluster/certificates/rotate', {
      method: 'POST',
      body: JSON.stringify({}),
    });
  },
  
  getAuditLogs: async (params?: {
    page?: number;
    pageSize?: number;
    resource?: string;
    resourceId?: string;
    action?: string;
  }): Promise<{ logs: AuditLog[]; total: number; page: number; pageSize: number }> => {
    try {
      const query = new URLSearchParams();
      if (params?.page != null) query.set('page', String(params.page));
      if (params?.pageSize != null) query.set('pageSize', String(params.pageSize));
      if (params?.resource?.trim()) query.set('resource', params.resource.trim());
      if (params?.resourceId?.trim()) query.set('resource_id', params.resourceId.trim());
      if (params?.action?.trim()) query.set('action', params.action.trim());
      const qs = query.toString();
      const url = qs ? `/audit/logs?${qs}` : '/audit/logs';
      const data = await request<{ logs: Array<Record<string, unknown>>; total?: number; page?: number; pageSize?: number }>(url);
      const logs = (data.logs || []).map((l) => {
        const detailsRaw = l.details;
        let status: AuditLog['status'] = 'success';
        if (typeof detailsRaw === 'string' && detailsRaw.toLowerCase().includes('failed')) status = 'failure';
        return {
          id: String(l.id ?? ''),
          action: String(l.action ?? ''),
          resource: String(l.resource ?? ''),
          resourceId: l.resourceId != null ? String(l.resourceId) : undefined,
          timestamp: String(l.createdAt ?? l.timestamp ?? ''),
          user: l.user ? String(l.user) : undefined,
          actor: String(l.user ?? l.userId ?? 'system'),
          ip: l.ip != null ? String(l.ip) : undefined,
          details: typeof detailsRaw === 'string' ? detailsRaw : detailsRaw ? JSON.stringify(detailsRaw) : '',
          status,
        } as AuditLog;
      });
      return {
        logs,
        total: Number(data.total) ?? 0,
        page: Number(data.page) ?? 1,
        pageSize: Number(data.pageSize) ?? 50,
      };
    } catch (err) {
      return { logs: [], total: 0, page: 1, pageSize: 50 };
    }
  },

  /** POST /api/v1/risk/insights/bulk – bulk acknowledge/resolve/dismiss insights. */
  bulkInsightsAction: async (params: {
    action: 'acknowledge' | 'resolve' | 'dismiss';
    insightIds: string[];
    resolution?: string;
    reason?: string;
  }): Promise<{ success_count: number; failed_count: number; action: string; errors?: Array<{ id: string; error: string }> }> => {
    const body: Record<string, unknown> = {
      action: params.action,
      insight_ids: params.insightIds,
    };
    if (params.action === 'resolve' && params.resolution != null) {
      body.resolution = params.resolution;
    }
    if (params.action === 'dismiss' && params.reason != null) {
      body.reason = params.reason;
    }
    return await request('/risk/insights/bulk', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },
  
  getReports: async (): Promise<Report[]> => {
    try {
      const data = await request<{ reports: Array<Record<string, unknown>> }>('/audit/reports');
      return (data.reports || []).map((r, idx) => ({
        id: `${r.resource ?? 'resource'}-${r.action ?? 'action'}-${idx}`,
        name: `${String(r.resource ?? 'resource')} / ${String(r.action ?? 'action')}`,
        title: `${String(r.resource ?? 'resource')} / ${String(r.action ?? 'action')}`,
        type: String(r.resource ?? ''),
        action: String(r.action ?? ''),
        resource: String(r.resource ?? ''),
        count: Number(r.count ?? 0),
        status: 'ready',
        generatedAt: new Date().toISOString(),
      }));
    } catch (err) {
      return [];
    }
  },

  getErrorLogs: async (params?: { page?: number; pageSize?: number; level?: string; source?: string }): Promise<{ logs: ErrorLog[]; total: number }> => {
    try {
      const query = new URLSearchParams();
      if (params?.page != null) query.set('page', String(params.page));
      if (params?.pageSize != null) query.set('pageSize', String(params.pageSize));
      if (params?.level) query.set('level', params.level);
      if (params?.source) query.set('source', params.source);
      const qs = query.toString();
      const url = qs ? `/error-logs?${qs}` : '/error-logs';
      const data = await request<{ logs?: ErrorLog[]; total?: number }>(url);
      return { logs: data.logs ?? [], total: data.total ?? 0 };
    } catch (err) {
      return { logs: [], total: 0 };
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
      const url = `/inventory/sbom?${qs}`;
      const data = await request<{ sboms: PodSbomSummary[] }>(url);
      const list = data.sboms || [];
      return list.map((s: Record<string, unknown>) => ({
        ...s,
        lastScan: s.lastScan != null ? String(s.lastScan) : '',
        podCreatedAt: s.podCreatedAt != null ? String(s.podCreatedAt) : undefined,
        podStatus: s.podStatus != null ? String(s.podStatus) : undefined,
        sbomSource: s.sbomSource != null ? String(s.sbomSource) : undefined,
        confidence: s.confidence != null ? String(s.confidence) : undefined,
        goVersion: s.goVersion != null ? String(s.goVersion) : undefined,
      })) as PodSbomSummary[];
    } catch (err) {
      return [];
    }
  },

  /** GET /api/v1/inventory/pods/:uid/sbom – SBOM detail by pod UID */
  getPodSbom: async (podUid: string): Promise<PodSbom | undefined> => {
    try {
      return await request<PodSbom>(`/inventory/pods/${encodeURIComponent(podUid)}/sbom`);
    } catch (err) {
      return undefined;
    }
  },

  /** GET /api/v1/malware/threats/:pod_uid – per-pod malware/telemetry matches (requires same auth as SBOM) */
  getPodThreatSummary: async (podUid: string): Promise<ThreatSummary | null> => {
    try {
      const data = await request<ThreatSummary>(`/malware/threats/${encodeURIComponent(podUid)}`);
      if (!data || data.totalThreats <= 0) return null;
      return data;
    } catch {
      return null;
    }
  },

  /** GET /api/v1/risk/pods/:uid/runtime/events — security runtime events (Falco ingest, REP, …) */
  getPodRuntimeSecurityEvents: async (podUid: string, limit = 100): Promise<PodRuntimeSecurityEvent[]> => {
    try {
      const data = await request<{ events?: PodRuntimeSecurityEvent[] }>(
        `/risk/pods/${encodeURIComponent(podUid)}/runtime/events?limit=${limit}`,
      );
      return data.events || [];
    } catch {
      return [];
    }
  },
  getPodRuntimeBehaviorFactsV2: async (podUid: string, limit = 100): Promise<PodRuntimeBehaviorFact[]> => {
    if (!podUid) return [];
    try {
      const data = await requestV2<{ facts?: PodRuntimeBehaviorFact[] }>(`/runtime/pods/${encodeURIComponent(podUid)}/facts?limit=${limit}`);
      return data.facts || [];
    } catch {
      return [];
    }
  },
  getPodRuntimeIncidentsV2: async (podUid: string, limit = 100): Promise<PodRuntimeIncident[]> => {
    if (!podUid) return [];
    try {
      const data = await requestV2<{ incidents?: PodRuntimeIncident[] }>(`/runtime/pods/${encodeURIComponent(podUid)}/incidents?limit=${limit}`);
      return data.incidents || [];
    } catch {
      return [];
    }
  },

  /** GET /api/v1/risk/pods/:uid/report – risk report for pod */
  getPodRiskReport: async (podUid: string): Promise<{ insights: Insight[]; summary?: PodRiskReportSummary }> => {
    try {
      const data = await request<{ insights?: unknown[]; summary?: Record<string, unknown> }>(
        `/risk/pods/${encodeURIComponent(podUid)}/report`,
      );
      const insights = (data.insights || []).map((i: any) => ({
        id: String(i.id ?? ''),
        cveId:
          i.cveId != null ? String(i.cveId) : i.cve_id != null ? String(i.cve_id) : undefined,
        affectedComponent:
          i.affectedComponent != null
            ? String(i.affectedComponent)
            : i.affected_component != null
              ? String(i.affected_component)
              : undefined,
        affectedVersion:
          i.affectedVersion != null
            ? String(i.affectedVersion)
            : i.affected_version != null
              ? String(i.affected_version)
              : undefined,
        title: String(i.title ?? ''),
        description: i.description != null ? String(i.description) : undefined,
        severity: (i.severity || 'medium').toLowerCase(),
        score: i.cvss != null ? Math.round(Number(i.cvss) * 10) : undefined,
        status: i.status === 'active' ? 'new' : i.status === 'resolved' ? 'resolved' : 'acknowledged',
        timestamp: i.detectedAt ?? i.createdAt,
        insightType:
          i.insightType != null
            ? String(i.insightType)
            : i.insight_type != null
              ? String(i.insight_type)
              : undefined,
        affectedResources: [{ id: i.resourceUid ?? '', name: i.resourceName, kind: i.resourceType, namespace: i.resourceNamespace }],
      })) as Insight[];
      const s = data.summary;
      const summary: PodRiskReportSummary | undefined = s
        ? {
            runtimeSignals24h: typeof s.runtimeSignals24h === 'number' ? s.runtimeSignals24h : Number(s.runtimeSignals24h) || undefined,
            podDirectInsightCount:
              typeof s.podDirectInsightCount === 'number' ? s.podDirectInsightCount : Number(s.podDirectInsightCount) || undefined,
            runtimePolicyInsightCount:
              typeof s.runtimePolicyInsightCount === 'number'
                ? s.runtimePolicyInsightCount
                : Number(s.runtimePolicyInsightCount) || undefined,
            insightsInReport:
              typeof s.insightsInReport === 'number' ? s.insightsInReport : Number(s.insightsInReport) || undefined,
            clusterAdminBindings:
              typeof s.clusterAdminBindings === 'number' ? s.clusterAdminBindings : Number(s.clusterAdminBindings) || undefined,
            wildcardRoles: typeof s.wildcardRoles === 'number' ? s.wildcardRoles : Number(s.wildcardRoles) || undefined,
            overprivilegedRoles:
              typeof s.overprivilegedRoles === 'number' ? s.overprivilegedRoles : Number(s.overprivilegedRoles) || undefined,
            riskLevel: s.riskLevel != null ? String(s.riskLevel) : undefined,
          }
        : undefined;
      return { insights, summary };
    } catch {
      return { insights: [] };
    }
  },

  /** byType 'all' = all insight types (default 'vulnerability' = CVE only). */
  getThreatVelocity: async (days = 7, clusterId?: string | null, byType?: 'all' | 'vulnerability'): Promise<ThreatVelocityPoint[]> => {
    try {
      const params = new URLSearchParams({ days: String(days) });
      if (clusterId?.trim()) params.set('clusterId', clusterId.trim());
      if (byType === 'all') params.set('byType', 'all');
      const qs = params.toString();
      const data = await request<{ trend?: ThreatVelocityPoint[] }>(`/dashboard/metrics/threat-velocity?${qs}`);
      const raw = Array.isArray(data)
        ? data
        : (Array.isArray((data as any)?.trend) ? (data as any).trend : Array.isArray((data as any)?.Trend) ? (data as any).Trend : []);
      const out = (raw || []).map((p: any) => ({
        date: String(p.date ?? p.Date ?? ''),
        critical: Number(p.critical ?? p.CriticalCount ?? 0),
        high: Number(p.high ?? p.HighCount ?? 0),
        medium: Number(p.medium ?? p.MediumCount ?? 0),
        low: Number(p.low ?? p.LowCount ?? 0),
      })).filter((p: { date: string }) => p.date);
      return out;
    } catch {
      return [];
    }
  },

  getPceSummaryByCluster: async (): Promise<PodCapabilitySummaryCluster[]> => {
    try {
      const data = await request<{ summary: PodCapabilitySummaryCluster[] }>('/inventory/pod-capabilities/summary/cluster');
      return data.summary || [];
    } catch (err) {
      return [];
    }
  },

  getPceSummaryByCapability: async (): Promise<PodCapabilitySummaryCapability[]> => {
    try {
      const data = await request<{ summary: PodCapabilitySummaryCapability[] }>('/inventory/pod-capabilities/summary/capability');
      return data.summary || [];
    } catch (err) {
      return [];
    }
  },

  getPceSummaryByNamespace: async (params?: { namespace?: string; severity?: string; capabilityId?: string; clusterId?: string }): Promise<PodCapabilitySummaryNamespace[]> => {
    try {
      const query = new URLSearchParams();
      if (params?.namespace) query.set('namespace', params.namespace);
      if (params?.severity) query.set('severity', params.severity);
      if (params?.capabilityId) query.set('capabilityId', params.capabilityId);
      if (params?.clusterId) query.set('clusterId', params.clusterId);
      const suffix = query.toString() ? `?${query.toString()}` : '';
      const data = await request<{ summary: PodCapabilitySummaryNamespace[] }>(`/inventory/pod-capabilities/summary/namespace${suffix}`);
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
      const data = await request<{ summary: PodCapabilitySummarySeverity[] }>(`/inventory/pod-capabilities/summary/severity${suffix}`);
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
      const data = await request<{ points?: PodCapabilityTrendPoint[] }>(`/inventory/pod-capabilities/trends?${query.toString()}`);
      const raw = Array.isArray(data)
        ? data
        : (Array.isArray((data as any)?.points) ? (data as any).points : Array.isArray((data as any)?.Points) ? (data as any).Points : []);
      const out = (raw || []).map((p: any) => ({
        date: String(p.date ?? p.Date ?? ''),
        critical: Number(p.critical ?? 0),
        high: Number(p.high ?? 0),
        medium: Number(p.medium ?? 0),
        low: Number(p.low ?? 0),
      })).filter((p: { date: string }) => p.date);
      return out;
    } catch {
      return [];
    }
  },

  getPceCapabilities: async (params?: {
    clusterId?: string;
    namespace?: string;
    capabilityId?: string;
    severity?: string;
    podName?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ capabilities: PodCapabilityDetail[]; total: number }> => {
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
      const data = await request<{
        capabilities: Array<PodCapabilityDetail & { pod_name?: string }>;
        total?: number;
        Total?: number;
      }>(`/inventory/pod-capabilities${suffix}`);
      const raw = data.capabilities || [];
      const totalRaw = data.total ?? data.Total ?? raw.length;
      const total = typeof totalRaw === 'number' && Number.isFinite(totalRaw) ? totalRaw : raw.length;
      return {
        capabilities: raw.map((c) => ({
          ...c,
          podName: c.podName ?? (c as any).pod_name ?? undefined,
        })) as PodCapabilityDetail[],
        total,
      };
    } catch (err) {
      return { capabilities: [], total: 0 };
    }
  },
  getPodCapabilities: async (podUid: string): Promise<PodCapabilityDetail[]> => {
    if (!podUid) return [];
    try {
      const data = await requestV2<{ capabilities: PodCapabilityDetail[] }>(
        `/runtime/pods/${encodeURIComponent(podUid)}/capabilities`,
      );
      return data.capabilities || [];
    } catch {
      try {
        const data = await request<{ capabilities: PodCapabilityDetail[] }>(
          `/inventory/pods/${encodeURIComponent(podUid)}/capabilities`,
        );
        return data.capabilities || [];
      } catch {
        return [];
      }
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
      const data = await request<{ podUid: string; steps: PodAttackStep[]; count: number }>(`/risk/pods/${encodeURIComponent(podUid)}/attack-steps`);
      return data.steps || [];
    } catch (err) {
      return [];
    }
  },

  getAttackStepsSummary: async (): Promise<AttackStepSummary[]> => {
    try {
      const data = await request<{ summary: AttackStepSummary[]; count: number }>('/risk/attack-steps/summary');
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
  getRuntimeSignals: async (params?: { podUid?: string; signalType?: string; category?: string; startDate?: string; endDate?: string; sinceMinutes?: number; limit?: number; offset?: number }): Promise<{ signals: RuntimeSignal[]; count: number; total: number }> => {
    try {
      const queryParams = new URLSearchParams();
      if (params?.podUid) queryParams.append('podUid', params.podUid);
      if (params?.signalType) queryParams.append('signalType', params.signalType);
      if (params?.category) queryParams.append('category', params.category);
      if (params?.sinceMinutes != null && params.sinceMinutes > 0) queryParams.append('sinceMinutes', params.sinceMinutes.toString());
      if (params?.startDate) queryParams.append('startDate', params.startDate);
      if (params?.endDate) queryParams.append('endDate', params.endDate);
      if (params?.limit) queryParams.append('limit', params.limit.toString());
      if (params?.offset) queryParams.append('offset', params.offset.toString());
      
      const query = queryParams.toString();
      const url = query ? `/runtime/signals?${query}` : '/runtime/signals';
      const data = await request<{ signals: RuntimeSignal[]; count: number; total: number; limit: number; offset: number }>(url);
      return { signals: data.signals || [], count: data.count || 0, total: data.total || 0 };
    } catch (err) {
      return { signals: [], count: 0, total: 0 };
    }
  },

  getRuntimeSignalsByPod: async (podUid: string, params?: { signalType?: string; category?: string; sinceMinutes?: number; limit?: number }): Promise<RuntimeSignal[]> => {
    try {
      const queryParams = new URLSearchParams();
      if (params?.sinceMinutes != null && params.sinceMinutes > 0) queryParams.append('sinceMinutes', params.sinceMinutes.toString());
      if (params?.signalType) queryParams.append('signalType', params.signalType);
      if (params?.category) queryParams.append('category', params.category);
      if (params?.limit) queryParams.append('limit', params.limit.toString());
      
      const query = queryParams.toString();
      const url = query ? `/runtime/pods/${encodeURIComponent(podUid)}/signals?${query}` : `/runtime/pods/${encodeURIComponent(podUid)}/signals`;
      const data = await request<{ podUid: string; signals: RuntimeSignal[]; count: number }>(url);
      return data.signals || [];
    } catch (err) {
      return [];
    }
  },

  getRuntimeSignalSuppressionStats: async (params?: { podUid?: string; sinceMinutes?: number }): Promise<RuntimeSignalSuppressionStats | null> => {
    try {
      const queryParams = new URLSearchParams();
      if (params?.podUid) queryParams.append('podUid', params.podUid);
      if (params?.sinceMinutes != null && params.sinceMinutes > 0) queryParams.append('sinceMinutes', params.sinceMinutes.toString());
      const query = queryParams.toString();
      const url = query ? `/runtime/signals/suppression-stats?${query}` : '/runtime/signals/suppression-stats';
      return await request<RuntimeSignalSuppressionStats>(url);
    } catch {
      return null;
    }
  },

  // Attack Paths Graph
  getAttackPathsGraph: async (): Promise<AttackPathGraphData> => {
    try {
      const data = await request<{ data: { nodes: any[]; links: any[] } }>('/graph/attack-paths/graph');
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
        type: link.type || '',
        value: link.weight || link.value || 1,
      }));
      return { nodes, links };
    } catch (err) {
      // If API fails, return empty graph (no mock data)
      return { nodes: [], links: [] };
    }
  },

  // Attack Paths Summary
  getAttackPathsSummary: async (clusterId?: string): Promise<AttackPathSummary | null> => {
    try {
      const query = clusterId ? `?cluster_id=${encodeURIComponent(clusterId)}` : '';
      const data = await request<{ data: AttackPathSummary }>(`/graph/attack-paths/summary${query}`);
      return data.data || null;
    } catch {
      return null;
    }
  },

  // Attack Paths for a specific pod
  getAttackPathsForPod: async (podUid: string): Promise<AttackPath[]> => {
    try {
      const data = await request<{ paths: AttackPath[]; count: number }>(`/graph/attack-paths/${encodeURIComponent(podUid)}`);
      return data.paths || [];
    } catch {
      return [];
    }
  },
};
