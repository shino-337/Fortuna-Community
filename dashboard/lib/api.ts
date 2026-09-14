
import {
  Cluster,
  ClusterOverview,
  ClusterInventory,
  ClusterAgent,
  ClusterSecuritySummary,
  NodeDetailResponse,
  PodWithRisk,
  ResourceRiskSignals,
  Insight,
  User,
  FortunaUserSession,
  SecurityActivityItem,
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
  RiskLevelCounts,
  ThreatVelocityPoint,
  RiskHistogramResponse,
  PodCapabilitySummaryCluster,
  PodCapabilitySummaryCapability,
  PodCapabilitySummaryNamespace,
  PodCapabilitySummarySeverity,
  PodCapabilityTrendPoint,
  PodCapabilityDetail,
  CapabilityMetadata,
  PolicyTemplateRow,
  PolicyInstanceRow,
  PodAttackStep,
  AttackStepSummary,
  AttackPath,
  AttackPathSummary,
  AttackChain,
  AttackObjective,
  AttackPathGraphData,
  AttackPathsPageBundle,
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
  K8sServiceAccount,
  K8sRbacResourceDetail,
  PodRiskReport,
  PodRiskReportSummary,
  ThreatSummary,
  WorkerStatus,
  UnifiedRiskScore,
  PipelineHealth,
  DashboardDataIntegrity,
  EnrichedAttackPath,
  ServiceAccountK8sPermissions,
} from '../types';
import { useAuthStore } from '../store/authStore';
import { deriveUnifiedRiskLevelFromScore } from './severity';

const CORE_API_URL = (import.meta as any).env?.VITE_CORE_API_URL || '';
const API_BASE = CORE_API_URL
  ? `${CORE_API_URL.replace(/\/$/, '')}/api/v1`
  : '/api/v1';
const API_V2_BASE = CORE_API_URL
  ? `${CORE_API_URL.replace(/\/$/, '')}/api/v2`
  : '/api/v2';

export const API_DEFAULTS = {
  PAGE: 1,
  PAGE_SIZE: 50,
  PAGE_SIZE_SMALL: 20,
  PAGE_SIZE_MAX: 1000,
  LIMIT_LIST: 200,
  LIMIT_DETAIL: 100,
  LIMIT_DASHBOARD: 5,
  TREND_DAYS: 7,
} as const;

function asArray<T = any>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : [];
}

function mapRawAttackPathGraphPayload(raw: { nodes?: any[]; links?: any[] } | null | undefined): AttackPathGraphData {
  const nodes = asArray<any>(raw?.nodes).map((node: any) => {
    const propsName =
      node.properties && typeof node.properties === 'object' && node.properties != null
        ? String((node.properties as { name?: string }).name || '').trim()
        : '';
    return {
      id: node.id || node.uid || String(node.name),
      label: (node.name || node.label || propsName || node.id) as string,
      type: node.kind?.toLowerCase() || node.type || 'pod',
      risk: node.riskLevel || node.risk || 'low',
    };
  });
  const links = asArray<any>(raw?.links).map((link: any) => ({
    source: link.source?.id || link.source || String(link.from),
    target: link.target?.id || link.target || String(link.to),
    type: link.type || '',
    value: link.weight || link.value || 1,
  }));
  return { nodes, links };
}

function normalizeAttackPath(raw: any): AttackPath | null {
  if (!raw || typeof raw !== 'object') return null;
  return {
    ...raw,
    nodes: asArray(raw.nodes),
    edges: asArray(raw.edges),
    total_risk: Number(raw.total_risk ?? 0),
    difficulty: Number(raw.difficulty ?? 0),
    impact: Number(raw.impact ?? 0),
    length: Number(raw.length ?? asArray(raw.nodes).length),
    description: String(raw.description ?? ''),
  } as AttackPath;
}

function normalizeAttackChain(raw: any): AttackChain | null {
  if (!raw || typeof raw !== 'object') return null;
  return {
    ...raw,
    paths: asArray<string>(raw.paths),
    involved_resources: asArray<string>(raw.involved_resources),
    steps: asArray(raw.steps),
    assumptions: asArray(raw.assumptions),
    evidence: asArray(raw.evidence),
    variant_nodes: asArray<string>(raw.variant_nodes),
  } as AttackChain;
}

function normalizeAttackPaths(value: unknown): AttackPath[] {
  return asArray<any>(value).map(normalizeAttackPath).filter((p): p is AttackPath => p != null);
}

function normalizeAttackChains(value: unknown): AttackChain[] {
  return asArray<any>(value).map(normalizeAttackChain).filter((c): c is AttackChain => c != null);
}

const buildUrl = (path: string) => {
  if (path.startsWith('http')) {
    return path;
  }
  return `${API_BASE}${path}`;
};
const buildUrlV2 = (path: string) => `${API_V2_BASE}${path}`;

const getToken = () => useAuthStore.getState().token;

type ApiErrorBody = {
  error?: string;
  code?: string;
  reason?: string;
  required_permission?: string;
  required_permissions?: string[];
  required_cluster_id?: string;
};

export class ApiError extends Error {
  status: number;
  body?: ApiErrorBody;

  constructor(status: number, message: string, body?: ApiErrorBody) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

async function parseErrorBody(res: Response): Promise<{ body?: ApiErrorBody; text?: string }> {
  const text = await res.text().catch(() => '');
  if (!text) return {};
  try {
    const body = JSON.parse(text) as ApiErrorBody;
    return { body, text };
  } catch {
    return { text };
  }
}

function mapRiskLevelCounts(raw: unknown): RiskLevelCounts | undefined {
  if (!raw || typeof raw !== 'object') return undefined;
  const counts = raw as Partial<Record<keyof RiskLevelCounts, unknown>>;
  const num = (v: unknown) => (typeof v === 'number' ? v : Number(v) || 0);
  return {
    critical: num(counts.critical),
    high: num(counts.high),
    medium: num(counts.medium),
    low: num(counts.low),
  };
}

function mapRiskInsightGroupToInsight(g: Record<string, unknown>): Insight {
  const memberCount = typeof g.member_count === 'number' ? g.member_count : Number(g.member_count) || 0;
  const sampleId = g.sample_insight_id != null ? String(g.sample_insight_id) : '';
  const maxScore = g.max_score != null ? Number(g.max_score) : undefined;
  const rounded = maxScore != null ? Math.min(100, Math.max(0, Math.round(maxScore))) : undefined;
  const fl = deriveUnifiedRiskLevelFromScore(rounded);
  const sev = String(g.max_severity ?? 'medium').toLowerCase();
  const itype = String(g.insight_type ?? 'vulnerability');
  const title = String(g.title ?? '');
  const displayTitle = memberCount > 1 ? `${title} · ${memberCount} findings` : title;
  const finalLevelRaw = g.final_level != null ? String(g.final_level).toLowerCase() : fl;
  return {
    id: sampleId,
    title: displayTitle,
    severity: sev,
    severityHint: sev,
    finalLevel: finalLevelRaw as Insight['finalLevel'],
    score: rounded,
    insightType: itype,
    status: 'new',
    isGroupRow: true,
    groupMemberCount: memberCount,
    affectedResources: [],
    category: itype === 'vulnerability' || itype === 'supply_chain_malware' ? 'sbom' : 'security',
  } as Insight;
}

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
    const { body, text } = await parseErrorBody(res);
    const err = body?.error;
    const message = err
      ? err === 'Authorization header required' ? 'Please log in.' : err
      : 'Session expired. Please log in again.';
    useAuthStore.getState().logout();
    throw new ApiError(res.status, message, body ?? (text ? { error: text } : undefined));
  }

  if (!res.ok) {
    const { body, text } = await parseErrorBody(res);
    throw new ApiError(res.status, getErrorMessage(res.status, text || undefined), body);
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

/** Fetch binary response (e.g. YAML download). Same auth as request(). */
const requestBlob = async (path: string): Promise<Blob> => {
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
  return res.blob();
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
    serviceAccountUid: p.serviceAccountUid != null ? String(p.serviceAccountUid) : undefined,
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
    unifiedScore:
      typeof (p as { unifiedScore?: unknown }).unifiedScore === 'number'
        ? ((p as { unifiedScore: number }).unifiedScore as number)
        : (p as { unifiedScore?: unknown }).unifiedScore != null && String((p as { unifiedScore?: unknown }).unifiedScore).trim() !== ''
          ? Number((p as { unifiedScore: unknown }).unifiedScore)
          : typeof (p as { total_score?: unknown }).total_score === 'number'
            ? ((p as { total_score: number }).total_score as number)
            : (p as { total_score?: unknown }).total_score != null && String((p as { total_score?: unknown }).total_score).trim() !== ''
              ? Number((p as { total_score: unknown }).total_score)
              : undefined,
    totalScore:
      typeof (p as { total_score?: unknown }).total_score === 'number'
        ? ((p as { total_score: number }).total_score as number)
        : (p as { total_score?: unknown }).total_score != null && String((p as { total_score?: unknown }).total_score).trim() !== ''
          ? Number((p as { total_score: unknown }).total_score)
          : undefined,
    finalLevel:
      (p as { finalLevel?: unknown }).finalLevel != null && String((p as { finalLevel?: unknown }).finalLevel).trim() !== ''
        ? String((p as { finalLevel: unknown }).finalLevel).toLowerCase()
        : undefined,
    scorerVersion:
      (p as { scorerVersion?: unknown }).scorerVersion != null && String((p as { scorerVersion?: unknown }).scorerVersion).trim() !== ''
        ? String((p as { scorerVersion: unknown }).scorerVersion)
        : undefined,
    riskSignals: mapRiskSignals((p as { risk_signals?: unknown }).risk_signals),
    pathPreview:
      (p as { path_preview?: unknown }).path_preview != null && String((p as { path_preview?: unknown }).path_preview).trim() !== ''
        ? String((p as { path_preview: unknown }).path_preview)
        : undefined,
    blastEntityCount:
      typeof (p as { blast_entity_count?: unknown }).blast_entity_count === 'number'
        ? Math.max(0, Math.floor((p as { blast_entity_count: number }).blast_entity_count))
        : undefined,
    riskFixHintPct:
      typeof (p as { risk_fix_hint_pct?: unknown }).risk_fix_hint_pct === 'number'
        ? Math.max(0, Math.min(100, Math.floor((p as { risk_fix_hint_pct: number }).risk_fix_hint_pct)))
        : undefined,
  };
}

function mapRiskSignals(raw: unknown): ResourceRiskSignals | undefined {
  if (raw == null || typeof raw !== 'object') return undefined;
  const o = raw as Record<string, unknown>;
  const str = (k: string) => (o[k] != null && String(o[k]).trim() !== '' ? String(o[k]) : '');
  const bool = (k: string) => Boolean(o[k]);
  const chainIds = Array.isArray(o.chain_ids)
    ? (o.chain_ids as unknown[]).map((x) => String(x)).filter((s) => s.trim() !== '')
    : [];
  const summary = str('summary') || str('risk_summary');
  const pathCountRaw = o.path_count;
  const pathCount =
    typeof pathCountRaw === 'number' && Number.isFinite(pathCountRaw)
      ? Math.max(0, Math.floor(pathCountRaw))
      : pathCountRaw != null && String(pathCountRaw).trim() !== ''
        ? Math.max(0, Math.floor(Number(pathCountRaw)))
        : undefined;
  return {
    hasAttackPath: bool('has_attack_path'),
    isEntryPoint: bool('is_entry_point'),
    isPivot: bool('is_pivot'),
    maxImpact: (str('max_impact') || 'LOW') as ResourceRiskSignals['maxImpact'],
    maxImpactSourceChainId: str('max_impact_source_chain_id') || undefined,
    pathCount,
    summary,
    chainIds,
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

export function normalizeInsightStatus(status: unknown): string {
  if (status === 'active' || status === 'new') return 'new';
  if (status === 'acknowledged' || status === 'resolved' || status === 'dismissed') return status;
  return 'unknown';
}

export const api = {
  login: async (username: string, password: string): Promise<{ user: User; token: string }> => {
    const resp = await request<{
      token: string;
      user: User;
      operationalScope?: User['operationalScope'];
    }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    const user =
      resp.operationalScope != null ? { ...resp.user, operationalScope: resp.operationalScope } : resp.user;
    return { user, token: resp.token };
  },

  getCurrentUser: async (): Promise<{ user: User; operationalScope?: User['operationalScope'] }> => {
    const data = await request<{ user: User; operationalScope?: User['operationalScope'] }>('/me');
    const user =
      data.operationalScope != null ? { ...data.user, operationalScope: data.operationalScope } : data.user;
    return { user };
  },

  changePassword: async (oldPassword: string, newPassword: string): Promise<User> => {
    const data = await request<{ user: User; message: string }>('/change-password', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword }),
    });
    return data.user;
  },

  /** POST /api/v1/auth/register — Admin or User admin; requires auth.register. */
  registerUser: async (body: { username: string; email: string; password: string; role: string; scopeJson?: string }): Promise<User> => {
    const data = await request<{ user: User }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    return data.user;
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

  /** GET /api/v1/risk/insights/summary — severity breakdown. Optional clusterId, sinceMinutes (time range). */
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
      riskLevelCounts: mapRiskLevelCounts(data.riskLevelCounts),
    };
  },

  /** GET /api/v1/risk/insights/summary/by-cluster — summary per cluster (for global view). Optional sinceMinutes. */
  getInsightsSummaryByCluster: async (sinceMinutes?: number): Promise<InsightsSummaryByClusterItem[]> => {
    const params = new URLSearchParams();
    if (sinceMinutes != null && sinceMinutes > 0) params.set('sinceMinutes', String(sinceMinutes));
    const qs = params.toString() ? `?${params.toString()}` : '';
    const data = await request<{ byCluster?: InsightsSummaryByClusterItem[] }>(`/risk/insights/summary/by-cluster${qs}`);
    return Array.isArray(data?.byCluster) ? data.byCluster : [];
  },

  /** GET /api/v1/risk/insights/summary/global — global (all-clusters) summary. Optional sinceMinutes. Same shape as getInsightsSummary. */
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
      riskLevelCounts: mapRiskLevelCounts(data.riskLevelCounts),
    };
  },

  /** GET /api/v1/clusters/:id/overview */
  getClusterOverview: async (id: string): Promise<ClusterOverview | null> => {
    try {
      return await request<ClusterOverview>(`/inventory/clusters/${encodeURIComponent(id)}/overview`);
    } catch {
      return null;
    }
  },

  /** GET /api/v1/inventory/clusters/:id/inventory */
  getClusterInventory: async (id: string): Promise<ClusterInventory | null> => {
    try {
      return await request<ClusterInventory>(`/inventory/clusters/${encodeURIComponent(id)}/inventory`);
    } catch {
      return null;
    }
  },

  getClusterInventoryStrict: async (id: string): Promise<ClusterInventory> => {
    return request<ClusterInventory>(`/inventory/clusters/${encodeURIComponent(id)}/inventory`);
  },

  /** GET /api/v1/clusters/:id/agents */
  getClusterAgents: async (id: string): Promise<{ agents: ClusterAgent[]; total: number }> => {
    try {
      const data = await request<{ agents: ClusterAgent[]; total: number }>(`/inventory/clusters/${encodeURIComponent(id)}/agents`);
      const rawAgents = (data.agents || []) as unknown as Array<Record<string, unknown>>;
      const agents = rawAgents.map((a) => ({
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
      return await request<ClusterSecuritySummary>(`/inventory/clusters/${encodeURIComponent(id)}/security-summary`);
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
      const c = await request<Record<string, unknown>>(`/inventory/clusters/${encodeURIComponent(id)}`);
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
          serviceAccountCount: typeof c.serviceAccountCount === 'number' ? c.serviceAccountCount : Number(c.serviceAccountCount) || undefined,
          roleCount: typeof c.roleCount === 'number' ? c.roleCount : Number(c.roleCount) || undefined,
          clusterRoleCount: typeof c.clusterRoleCount === 'number' ? c.clusterRoleCount : Number(c.clusterRoleCount) || undefined,
          roleBindingCount: typeof c.roleBindingCount === 'number' ? c.roleBindingCount : Number(c.roleBindingCount) || undefined,
          clusterRoleBindingCount:
            typeof c.clusterRoleBindingCount === 'number'
              ? c.clusterRoleBindingCount
              : Number(c.clusterRoleBindingCount) || undefined,
        } as Cluster;
      });
    } catch (err) {
      return [];
    }
  },

  /** GET /api/v1/risk/insights/:id — single insight for Risk Detail */
  getInsight: async (id: string): Promise<Insight | null> => {
    try {
      const insight = await request<Record<string, unknown>>(`/risk/insights/${id}`);
      const severity = String(insight.severity ?? 'medium').toLowerCase();
      const totalScoreFromApi = insight.totalScore != null ? Number(insight.totalScore) : null;
      const finalScoreFromApi =
        insight.final_score != null
          ? Number(insight.final_score)
          : insight.finalScore != null
            ? Number(insight.finalScore)
            : totalScoreFromApi;
      const finalLevelFromApi =
        insight.final_level != null
          ? String(insight.final_level).toLowerCase()
          : insight.finalLevel != null
            ? String(insight.finalLevel).toLowerCase()
            : deriveUnifiedRiskLevelFromScore(finalScoreFromApi ?? undefined);
      const severityHintFromApi =
        insight.severity_hint != null
          ? String(insight.severity_hint).toLowerCase()
          : insight.severityHint != null
            ? String(insight.severityHint).toLowerCase()
            : severity;
      const insightTypeRaw = (insight.insightType ?? insight.insight_type ?? '') as string;

      const evidenceRefsRaw = insight.evidence_refs ?? insight.evidenceRefs;
      const evidence_refs: Insight['evidence_refs'] =
        evidenceRefsRaw != null && typeof evidenceRefsRaw === 'object' && !Array.isArray(evidenceRefsRaw)
          ? (evidenceRefsRaw as Insight['evidence_refs'])
          : undefined;

      const explanationChainRaw = insight.explanation_chain ?? insight.explanationChain;
      let explanation_chain: Insight['explanation_chain'] = undefined;
      if (Array.isArray(explanationChainRaw) && explanationChainRaw.length > 0) {
        const steps = explanationChainRaw
          .filter((step): step is Record<string, unknown> => step != null && typeof step === 'object')
          .map((step) => ({
            layer: String(step.layer ?? ''),
            refs: Array.isArray(step.refs)
              ? step.refs.filter((x): x is string => typeof x === 'string')
              : [],
          }))
          .filter((s) => s.layer !== '' || s.refs.length > 0);
        explanation_chain = steps.length > 0 ? steps : undefined;
      }

      const ecRaw = insight.evidence_chain_refs ?? insight.evidenceChainRefs;
      let evidence_chain_refs: Insight['evidence_chain_refs'] = undefined;
      if (Array.isArray(ecRaw) && ecRaw.length > 0) {
        const rows = ecRaw
          .filter((row): row is Record<string, unknown> => row != null && typeof row === 'object')
          .map((row) => ({
            layer: String(row.layer ?? ''),
            ref: String(row.ref ?? ''),
          }))
          .filter((r) => r.ref !== '');
        evidence_chain_refs = rows.length > 0 ? rows : undefined;
      }

      const enrichedRaw = insight.enriched_refs ?? insight.enrichedRefs;
      const enriched_refs: Insight['enriched_refs'] =
        enrichedRaw != null && typeof enrichedRaw === 'object' && !Array.isArray(enrichedRaw)
          ? (enrichedRaw as Insight['enriched_refs'])
          : undefined;

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
        severityHint: severityHintFromApi,
        finalLevel: finalLevelFromApi as 'low' | 'medium' | 'high' | 'critical' | undefined,
        finalScore: finalScoreFromApi ?? undefined,
        breakdown: Array.isArray(insight.breakdown) ? insight.breakdown : undefined,
        // Unified display standard: use authoritative score from risk_scores when present.
        score: finalScoreFromApi != null ? Math.round(finalScoreFromApi) : undefined,
        status: normalizeInsightStatus(insight.status),
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
        evidence_refs,
        explanation_chain,
        evidence_chain_refs,
        enriched_refs,
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

  /** POST /api/v1/risk/insights/:id/resolve — mark insight as resolved */
  resolveInsight: async (id: string, resolution?: string): Promise<void> => {
    await request(`/risk/insights/${id}/resolve`, {
      method: 'POST',
      body: JSON.stringify({ resolution: resolution ?? '' }),
    });
  },

  /** GET /api/v1/risks – optional finalLevel filter (ADR bands on preferred risk_scores). */
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

  /** GET /api/v1/risks – defaults to all insight types. Optional finalLevel (low|medium|high|critical) filters by ADR score bands on preferred risk_scores. scoreBin: 0|10|...|90. */
  getRisks: async (params?: {
    page?: number;
    pageSize?: number;
    type?: string;
    status?: string;
    search?: string;
    clusterId?: string | null;
    namespace?: string;
    sinceMinutes?: number;
    withScores?: number;
    /** ADR unified risk level filter (server: finalLevel query param) */
    finalLevel?: '' | 'low' | 'medium' | 'high' | 'critical';
    scoreBin?: number;
    /** instance (default) | group — grouped findings by type + CVE/title */
    view?: 'instance' | 'group';
  }): Promise<{ insights: Insight[]; total: number; page: number; pageSize: number; view?: string }> => {
    try {
      const query = new URLSearchParams();
      if (params?.type) query.set('type', params.type);
      if (params?.page != null) query.set('page', String(params.page));
      if (params?.pageSize != null) query.set('pageSize', String(params.pageSize));
      if (params?.status != null) query.set('status', params.status);
      if (params?.search?.trim()) query.set('search', params.search.trim());
      if (params?.clusterId?.trim()) query.set('clusterId', params.clusterId.trim());
      if (params?.namespace?.trim()) query.set('resourceNamespace', params.namespace.trim());
      if (params?.sinceMinutes != null && params.sinceMinutes > 0) query.set('sinceMinutes', String(params.sinceMinutes));
      // Unified display standard: always request authoritative risk_scores.
      if (params?.withScores === 1 || params?.withScores == null) query.set('withScores', '1');
      if (params?.finalLevel?.trim()) query.set('finalLevel', params.finalLevel.trim());
      if (params?.scoreBin != null && params.scoreBin >= 0 && params.scoreBin <= 90 && params.scoreBin % 10 === 0) query.set('scoreBin', String(params.scoreBin));
      if (params?.view === 'group') query.set('view', 'group');
      else if (params?.view === 'instance') query.set('view', 'instance');
      const qs = query.toString();
      const url = qs ? `/risk/insights?${qs}` : '/risk/insights';
      const data = await request<{
        total: number;
        page?: number;
        pageSize?: number;
        insights: any[];
        view?: string;
        groups?: Record<string, unknown>[];
      }>(url);
      const view = String(data.view || 'instance').toLowerCase();
      if (view === 'group' && Array.isArray(data.groups)) {
        const insights = data.groups.map(mapRiskInsightGroupToInsight);
        return {
          insights,
          total: Number(data.total) ?? 0,
          page: Number(data.page) ?? 1,
          pageSize: Number(data.pageSize) ?? 20,
          view: 'group',
        };
      }
      const insights = (data.insights || []).map((insight) => {
        const severity = (insight.severity || 'medium').toLowerCase();
        // Unified display standard: score comes from authoritative risk_scores.
        const totalScoreFromApi = insight.totalScore != null ? Number(insight.totalScore) : null;
        const finalScoreFromApi =
          insight.final_score != null
            ? Number(insight.final_score)
            : insight.finalScore != null
              ? Number(insight.finalScore)
              : totalScoreFromApi;
        const finalLevel =
          insight.final_level != null
            ? String(insight.final_level).toLowerCase()
            : insight.finalLevel != null
              ? String(insight.finalLevel).toLowerCase()
              : deriveUnifiedRiskLevelFromScore(finalScoreFromApi ?? undefined);
        const severityHint =
          insight.severity_hint != null
            ? String(insight.severity_hint).toLowerCase()
            : insight.severityHint != null
              ? String(insight.severityHint).toLowerCase()
              : severity;
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
          severityHint,
          finalLevel,
          breakdown: Array.isArray(insight.breakdown) ? insight.breakdown : undefined,
          score: finalScoreFromApi != null ? Math.min(100, Math.max(0, Math.round(finalScoreFromApi))) : undefined,
          category: itype === 'vulnerability' || itype === 'supply_chain_malware' ? 'sbom' : 'security',
          insightType: itype,
          status: normalizeInsightStatus(insight.status),
          timestamp: insight.detectedAt || insight.createdAt,
          updatedAt: insight.updatedAt != null ? String(insight.updatedAt) : undefined,
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
          totalScore: finalScoreFromApi ?? undefined,
          finalScore: finalScoreFromApi ?? undefined,
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
        view: 'instance',
      };
    } catch (err) {
      return { insights: [], total: 0, page: 1, pageSize: 20, view: 'instance' };
    }
  },

  /** GET /api/v1/risks/export – download risks as CSV (same filters as getRisks). Triggers browser download. */
  exportRisksCSV: async (params?: { clusterId?: string | null; finalLevel?: string; status?: string; search?: string; sinceMinutes?: number }): Promise<void> => {
    const query = new URLSearchParams();
    if (params?.clusterId?.trim()) query.set('clusterId', params.clusterId.trim());
    if (params?.finalLevel) query.set('finalLevel', params.finalLevel);
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
  exportRisksPDF: async (params?: { clusterId?: string | null; finalLevel?: string; status?: string; search?: string; sinceMinutes?: number }): Promise<void> => {
    const query = new URLSearchParams();
    query.set('format', 'pdf');
    if (params?.clusterId?.trim()) query.set('clusterId', params.clusterId.trim());
    if (params?.finalLevel) query.set('finalLevel', params.finalLevel);
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
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 15000);
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    const token = getToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    try {
      const res = await fetch(buildUrl(`/inventory/pods/${encodeURIComponent(uid)}`), {
        headers,
        signal: controller.signal,
      });
      if (res.status === 401) {
        useAuthStore.getState().logout();
        throw new Error('Session expired. Please log in again.');
      }
      if (res.status === 404) return null;
      if (!res.ok) {
        const text = await res.text().catch(() => '');
        throw new Error(getErrorMessage(res.status, text || undefined));
      }
      const p = await res.json() as Record<string, unknown>;
      return mapApiPodToPodWithRisk(p);
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') {
        throw new Error('Pod detail request timed out.');
      }
      throw e;
    } finally {
      window.clearTimeout(timeout);
    }
  },
  /** Legacy DB-id links are resolved client-side to the canonical UID route. */
  getPodByLegacyId: async (id: string | number): Promise<PodWithRisk | null> => {
    const numericId = Number(id);
    if (!Number.isFinite(numericId) || numericId <= 0) return null;
    const pageSize = 1000;
    let page = 1;
    while (page <= 20) {
      const { pods, total } = await api.getPods({ page, pageSize });
      const match = pods.find((p) => Number(p.id) === numericId);
      if (match) return match;
      if (pods.length === 0 || page * pageSize >= total) break;
      page += 1;
    }
    return null;
  },
  /** @deprecated Use getPodByUid */
  get getPod() { return this.getPodByUid; },

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
    /** Filter by source pod UID (AND with namespace/q); use for drill-down when podName is missing. */
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
    const data = await request<{
      view?: string;
      clusterId?: string;
      total?: number;
      page?: number;
      pageSize?: number;
      items?:
        | NetworkActivityWorkloadRow[]
        | NetworkActivityConnectionRow[]
        | NetworkActivityDestinationRow[]
        | NetworkActivityTalkerRow[]
        | null;
    } | null>(`/runtime/network-activity?${q.toString()}`);
    return {
      view: data?.view ?? params.view ?? 'connections',
      clusterId: data?.clusterId ?? params.cluster,
      total: data?.total ?? 0,
      page: data?.page ?? params.page ?? 1,
      pageSize: data?.pageSize ?? params.pageSize ?? 50,
      items: data?.items ?? [],
    };
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

  getPodsStrict: async (params?: {
    cluster?: string;
    namespace?: string;
    node?: string;
    page?: number;
    pageSize?: number;
    search?: string;
    sortBy?: string;
  }): Promise<{ pods: PodWithRisk[]; total: number; page: number; pageSize: number }> => {
    const q = new URLSearchParams();
    if (params?.cluster) q.set('cluster', params.cluster);
    if (params?.namespace) q.set('namespace', params.namespace);
    if (params?.node) q.set('node', params.node);
    if (params?.page != null) q.set('page', String(params.page));
    if (params?.pageSize != null) q.set('pageSize', String(params.pageSize));
    if (params?.search?.trim()) q.set('search', params.search.trim());
    if (params?.sortBy) q.set('sortBy', params.sortBy);
    const qs = q.toString();
    const data = await request<{ pods: Array<Record<string, unknown>>; total: number; page?: number; pageSize?: number }>(
      qs ? `/inventory/pods?${qs}` : '/inventory/pods',
    );
    const pods = (data.pods || []).map((p: Record<string, unknown>) => mapApiPodToPodWithRisk(p));
    return {
      pods,
      total: Number(data.total) ?? pods.length,
      page: Number(data.page) ?? 1,
      pageSize: Number(data.pageSize) ?? 50,
    };
  },

  /** GET /api/v1/inventory/pods – list pods with riskCount; includes unifiedScore/finalLevel when risk_scores v3 exists */
  getPods: async (params?: {
    cluster?: string;
    namespace?: string;
    node?: string;
    page?: number;
    pageSize?: number;
    /** Server-side filter across all pods (name, namespace, uid, node) */
    search?: string;
    /** Server-side ordering: name_asc | namespace_asc | risk_desc | created_desc | default newest created first */
    sortBy?: string;
  }): Promise<{ pods: PodWithRisk[]; total: number; page: number; pageSize: number }> => {
    try {
      const q = new URLSearchParams();
      if (params?.cluster) q.set('cluster', params.cluster);
      if (params?.namespace) q.set('namespace', params.namespace);
      if (params?.node) q.set('node', params.node);
      if (params?.page != null) q.set('page', String(params.page));
      if (params?.pageSize != null) q.set('pageSize', String(params.pageSize));
      if (params?.search?.trim()) q.set('search', params.search.trim());
      if (params?.sortBy) q.set('sortBy', params.sortBy);
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

  getResourceDetail: async (kind: string, uid: string): Promise<K8sRbacResourceDetail | null> => {
    try {
      return await request<K8sRbacResourceDetail>(
        `/resources/${encodeURIComponent(kind)}/${encodeURIComponent(uid)}`,
      );
    } catch {
      return null;
    }
  },

  getRules: async (limit = API_DEFAULTS.LIMIT_LIST): Promise<SecurityRule[]> => {
    try {
      const data = await request<{ rules: Array<Record<string, unknown>>; total?: number; active?: number; disabled?: number }>(`/policy/rules?limit=${limit}`);
      const raw = data.rules || [];
      return raw.map((r) => ({
        id: String(r.id ?? r.uid ?? ''),
        uid: String(r.uid ?? r.id ?? ''),
        name: String(r.name ?? r.id ?? r.uid ?? ''),
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

  /** GET /api/v1/policy/rules/uid/:uid/metrics */
  getRuleMetrics: async (uid: string): Promise<{ totalMatches: number; recentMatches: Insight[] }> => {
    try {
      const data = await request<{ totalMatches?: number; recentMatches?: unknown[] }>(`/policy/rules/uid/${encodeURIComponent(uid)}/metrics`);
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

  /** POST /api/v1/policy/rules/uid/:uid/test */
  testRule: async (uid: string, resource: Record<string, unknown>): Promise<{ match: boolean; details?: Record<string, unknown> }> => {
    return await request<{ match: boolean; details?: Record<string, unknown> }>(`/policy/rules/uid/${encodeURIComponent(uid)}/test`, {
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

  /** GET /api/v1/policy/rules/uid/:uid – single rule + matchCount + recentMatches */
  getRule: async (uid: string): Promise<{ rule: SecurityRule & { description?: string }; matchCount: number; recentMatches: Insight[] } | null> => {
    try {
      const data = await request<{ uid?: string; ruleUid?: string; rule: unknown; source?: string; signature?: string; overlapGroup?: string; isCanonical?: boolean; canonicalRule?: string; impactedFindings24h?: number; impactedFindings7d?: number; relatedCapabilities?: string[]; matchCount?: number; recentMatches?: unknown[] }>(`/policy/rules/uid/${encodeURIComponent(uid)}`);
      const rule = {
        ...(data.rule as SecurityRule & { description?: string }),
        uid: data.uid ?? data.ruleUid ?? uid,
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
  getServiceAccountByUid: async (uid: string): Promise<Record<string, unknown> | null> => {
    try {
      return await request<Record<string, unknown>>(`/inventory/serviceaccounts/${encodeURIComponent(uid)}`);
    } catch {
      return null;
    }
  },
  /** @deprecated Use getServiceAccountByUid */
  get getServiceAccount() { return this.getServiceAccountByUid; },

  /** GET /api/v1/inventory/serviceaccounts — list Kubernetes ServiceAccounts, optionally scoped. */
  getServiceAccounts: async (params?: {
    clusterId?: string;
    namespace?: string;
    page?: number;
    pageSize?: number;
  }): Promise<{ serviceAccounts: K8sServiceAccount[]; total: number; page: number; pageSize: number }> => {
    const qs = new URLSearchParams();
    if (params?.clusterId) qs.set('cluster', params.clusterId);
    if (params?.namespace) qs.set('namespace', params.namespace);
    qs.set('page', String(params?.page ?? 1));
    qs.set('pageSize', String(params?.pageSize ?? 1000));

    const data = await request<{
      serviceAccounts?: unknown[];
      total?: number;
      page?: number;
      pageSize?: number;
    }>(`/inventory/serviceaccounts?${qs.toString()}`);

    const serviceAccounts = (Array.isArray(data.serviceAccounts) ? data.serviceAccounts : []).map((row) => {
      const sa = row as Record<string, unknown>;
      return {
        id: Number(sa.id ?? 0),
        clusterId: String(sa.clusterId ?? ''),
        name: String(sa.name ?? ''),
        namespace: String(sa.namespace ?? ''),
        uid: String(sa.uid ?? ''),
        labels: sa.labels != null ? String(sa.labels) : undefined,
        secrets: sa.secrets != null ? String(sa.secrets) : undefined,
        linkedPods: sa.linkedPods != null ? String(sa.linkedPods) : undefined,
        lastUsed: sa.lastUsed != null ? String(sa.lastUsed) : null,
        createdAt: sa.createdAt != null ? String(sa.createdAt) : undefined,
        updatedAt: sa.updatedAt != null ? String(sa.updatedAt) : undefined,
      };
    });

    return {
      serviceAccounts,
      total: Number(data.total ?? serviceAccounts.length),
      page: Number(data.page ?? params?.page ?? 1),
      pageSize: Number(data.pageSize ?? params?.pageSize ?? serviceAccounts.length),
    };
  },

  /** GET /api/v1/inventory/serviceaccounts/:uid/permissions (effectiveRules + roleBindings + clusterRoleBindings) */
  getServiceAccountPermissions: async (uid: string): Promise<ServiceAccountK8sPermissions> => {
    try {
      return await request<ServiceAccountK8sPermissions>(
        `/inventory/serviceaccounts/${encodeURIComponent(uid)}/permissions`,
      );
    } catch {
      return { effectiveRules: [] };
    }
  },

  getCertificates: async (): Promise<Certificate[]> => {
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
  },

  getAgents: async (): Promise<Agent[]> => {
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
  },

  getUsers: async (): Promise<User[]> => {
    const data = await request<{ users: Array<Record<string, unknown>> }>('/users');
    return (data.users || []).map((u) => ({
      id: String(u.id ?? ''),
      username: String(u.username ?? ''),
      name: String(u.username ?? ''),
      email: u.email ? String(u.email) : undefined,
      role: u.role ? String(u.role) : undefined,
      permissions: Array.isArray(u.permissions) ? u.permissions.map((p) => String(p)) : undefined,
      scopeJson: typeof u.scopeJson === 'string' ? u.scopeJson : undefined,
      operationalScope:
        u.operationalScope && typeof u.operationalScope === 'object'
          ? (u.operationalScope as User['operationalScope'])
          : undefined,
      active: typeof u.active === 'boolean' ? u.active : undefined,
      status: typeof u.active === 'boolean' ? (u.active ? 'active' : 'disabled') : undefined,
    }));
  },

  updateUser: async (id: string, body: { role?: string; active?: boolean; scopeJson?: string }): Promise<void> => {
    await request(`/users/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  },

  deleteUser: async (id: string): Promise<void> => {
    await request(`/users/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },

  getNotifications: async (limit = API_DEFAULTS.LIMIT_LIST): Promise<Notification[]> => {
    try {
      const data = await request<{ notifications: Array<Record<string, unknown>>, unreadCount?: number }>(`/notifications?limit=${limit}`);
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
          category: n.category ? String(n.category) : undefined,
          route: n.route ? String(n.route) : undefined,
          resourceUid: n.resourceUid ? String(n.resourceUid) : undefined,
          resourceName: n.resourceName ? String(n.resourceName) : undefined,
          readAt,
          read: Boolean(readAt),
          timestamp: n.timestamp ? String(n.timestamp) : undefined,
        } as Notification;
      });
    } catch (err) {
      return [];
    }
  },

  getNotificationsSummary: async (limit = 20): Promise<{ notifications: Notification[]; unreadCount: number }> => {
    try {
      const data = await request<{ notifications: Array<Record<string, unknown>>, unreadCount?: number }>(`/notifications?limit=${limit}`);
      const notifications = (data.notifications || []).map((n) => {
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
          category: n.category ? String(n.category) : undefined,
          route: n.route ? String(n.route) : undefined,
          resourceUid: n.resourceUid ? String(n.resourceUid) : undefined,
          resourceName: n.resourceName ? String(n.resourceName) : undefined,
          readAt,
          read: Boolean(readAt),
          timestamp: n.timestamp ? String(n.timestamp) : undefined,
        } as Notification;
      });
      return { notifications, unreadCount: Number(data.unreadCount || 0) };
    } catch {
      return { notifications: [], unreadCount: 0 };
    }
  },

  markNotificationRead: async (id: string): Promise<void> => {
    await request(`/notifications/${encodeURIComponent(id)}/read`, { method: 'PATCH' });
  },

  markAllNotificationsRead: async (): Promise<void> => {
    await request('/notifications/read-all', { method: 'POST' });
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
  
  getReportsStrict: async (params?: { hours?: number }): Promise<Report[]> => {
    const query = new URLSearchParams();
    if (params?.hours != null && params.hours > 0) query.set('hours', String(params.hours));
    const qs = query.toString();
    const data = await request<{ reports: Array<Record<string, unknown>> }>(qs ? `/audit/reports?${qs}` : '/audit/reports');
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
  },

  getReports: async (params?: { hours?: number }): Promise<Report[]> => {
    try {
      return await api.getReportsStrict(params);
    } catch (err) {
      return [];
    }
  },

  getErrorLogs: async (params?: { page?: number; pageSize?: number; level?: string; source?: string }): Promise<{ logs: ErrorLog[]; total: number }> => {
      const query = new URLSearchParams();
      if (params?.page != null) query.set('page', String(params.page));
      if (params?.pageSize != null) query.set('pageSize', String(params.pageSize));
      if (params?.level) query.set('level', params.level);
      if (params?.source) query.set('source', params.source);
      const qs = query.toString();
      const url = qs ? `/error-logs?${qs}` : '/error-logs';
      const data = await request<{ logs?: ErrorLog[]; total?: number }>(url);
      return { logs: data.logs ?? [], total: data.total ?? 0 };
  },

  getSyncStatus: async (): Promise<SyncStatus> => {
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
  },

  getWorkerStatus: async (): Promise<WorkerStatus[]> => {
      const data = await request<{ workers: WorkerStatus[] }>('/metrics/workers');
      return data.workers ?? [];
  },

  getDashboardDataIntegrity: async (): Promise<DashboardDataIntegrity> => {
      const data = await request<DashboardDataIntegrity>('/health/dashboard-data-integrity');
      const ch = data.catalogHealth || ({} as DashboardDataIntegrity['catalogHealth']);
      return {
        ...data,
        alerts: data.alerts || [],
        endpoints: data.endpoints || [],
        crossChecks: {
          activeAgentsCount: Number(data.crossChecks?.activeAgentsCount || 0),
          dashboardAgentsCount: Number(data.crossChecks?.dashboardAgentsCount || 0),
          clustersCount: Number(data.crossChecks?.clustersCount || 0),
          podsCount: Number(data.crossChecks?.podsCount || 0),
          insightsCount: Number(data.crossChecks?.insightsCount || 0),
          criticalInsights: Number(data.crossChecks?.criticalInsights || 0),
          cvesCount: Number(data.crossChecks?.cvesCount || 0),
          packageVulnerabilitiesCount: Number(data.crossChecks?.packageVulnerabilitiesCount || 0),
          osvPackagesCount: Number(data.crossChecks?.osvPackagesCount || 0),
          malwarePackagesCount: Number(data.crossChecks?.malwarePackagesCount || 0),
          sbomsCount: Number(data.crossChecks?.sbomsCount || 0),
          podsMissingSbom: Number(data.crossChecks?.podsMissingSbom || 0),
        },
        catalogHealth: {
          ...ch,
          status: ch.status || 'unavailable',
          activeCatalogGenerationId: Number(ch.activeCatalogGenerationId || 0),
          activeMalwareGenerationId: Number(ch.activeMalwareGenerationId || 0),
          cvesCount: Number(ch.cvesCount || 0),
          packageVulnerabilitiesCount: Number(ch.packageVulnerabilitiesCount || 0),
          osvPackagesCount: Number(ch.osvPackagesCount || 0),
          malwarePackagesCount: Number(ch.malwarePackagesCount || 0),
          activeSboms: Number(ch.activeSboms || 0),
          staleSboms: Number(ch.staleSboms || 0),
          activeSbomsMatchedMirror: Number(ch.activeSbomsMatchedMirror || 0),
          activeSbomsMissingMirrorMatch: Number(ch.activeSbomsMissingMirrorMatch || 0),
          activeSbomsMatchedGeneration: Number(ch.activeSbomsMatchedGeneration || 0),
          activeSbomsMissingGenerationMatch: Number(ch.activeSbomsMissingGenerationMatch || 0),
          currentMirrorSucceededRuns: Number(ch.currentMirrorSucceededRuns || 0),
          currentMirrorFailedRuns: Number(ch.currentMirrorFailedRuns || 0),
          currentMirrorRunningRuns: Number(ch.currentMirrorRunningRuns || 0),
          currentGenerationSucceededRuns: Number(ch.currentGenerationSucceededRuns || 0),
          currentGenerationFailedRuns: Number(ch.currentGenerationFailedRuns || 0),
          currentGenerationRunningRuns: Number(ch.currentGenerationRunningRuns || 0),
          activePodCveMatches: Number(ch.activePodCveMatches || 0),
          stalePodCveMatches: Number(ch.stalePodCveMatches || 0),
        },
      };
  },

  getSbomList: async (params?: { podName?: string; namespace?: string; limit?: number }): Promise<PodSbomSummary[]> => {
    try {
      const query = new URLSearchParams();
      query.set('limit', String(params?.limit ?? API_DEFAULTS.LIMIT_LIST));
      if (params?.podName?.trim()) query.set('podName', params.podName.trim());
      if (params?.namespace?.trim()) query.set('namespace', params.namespace.trim());
      const qs = query.toString();
      const url = `/inventory/sbom?${qs}`;
      const data = await request<{ sboms: PodSbomSummary[] }>(url);
      const list = (data.sboms || []) as unknown as Array<Record<string, unknown>>;
      return list.map((s) => ({
        ...s,
        lastScan: s.lastScan != null ? String(s.lastScan) : '',
        imageDigest: s.imageDigest != null ? String(s.imageDigest) : undefined,
        imageTrust: s.imageTrust,
        podCreatedAt: s.podCreatedAt != null ? String(s.podCreatedAt) : undefined,
        podStatus: s.podStatus != null ? String(s.podStatus) : undefined,
        activePod: Boolean(s.activePod),
        lifecycleState: s.lifecycleState != null ? String(s.lifecycleState) : undefined,
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
  getPodRiskReport: async (podUid: string): Promise<PodRiskReport> => {
    try {
      const data = await request<Record<string, unknown> & { insights?: unknown[]; summary?: Record<string, unknown> }>(
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
        status: normalizeInsightStatus(i.status),
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
      return {
        podUid: String(data.podUid ?? podUid),
        podName: String(data.podName ?? ''),
        namespace: String(data.namespace ?? ''),
        clusterId: String(data.clusterId ?? ''),
        serviceAccount: data.serviceAccount != null ? String(data.serviceAccount) : undefined,
        serviceAccountUid: data.serviceAccountUid != null ? String(data.serviceAccountUid) : undefined,
        bindings: Array.isArray(data.bindings) ? data.bindings : [],
        roles: Array.isArray(data.roles) ? data.roles : [],
        insights,
        summary,
      };
    } catch {
      return { podUid, podName: '', namespace: '', clusterId: '', insights: [] };
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

  getPceSummaryByCapability: async (params?: { clusterId?: string }): Promise<PodCapabilitySummaryCapability[]> => {
    try {
      const query = new URLSearchParams();
      if (params?.clusterId) query.set('clusterId', params.clusterId);
      const suffix = query.toString() ? `?${query.toString()}` : '';
      const data = await request<{ summary: PodCapabilitySummaryCapability[] }>(`/inventory/pod-capabilities/summary/capability${suffix}`);
      return (data.summary || []).sort((a, b) => (b.count ?? 0) - (a.count ?? 0));
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

  getPceSummaryBySeverity: async (params?: { severity?: string; capabilityId?: string; clusterId?: string }): Promise<PodCapabilitySummarySeverity[]> => {
    try {
      const query = new URLSearchParams();
      if (params?.severity) query.set('severity', params.severity);
      if (params?.capabilityId) query.set('capabilityId', params.capabilityId);
      if (params?.clusterId) query.set('clusterId', params.clusterId);
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
        critical: Number(p.critical ?? p.critical_count ?? p.CriticalCount ?? 0),
        high: Number(p.high ?? p.high_count ?? p.HighCount ?? 0),
        medium: Number(p.medium ?? p.medium_count ?? p.MediumCount ?? 0),
        low: Number(p.low ?? p.low_count ?? p.LowCount ?? 0),
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
  getCapabilityMetadata: async (params?: {
    search?: string;
    domain?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ metadata: CapabilityMetadata[]; total: number; count: number; limit: number; offset: number }> => {
    try {
      const q = new URLSearchParams();
      if (params?.search?.trim()) q.set('search', params.search.trim());
      if (params?.domain && params.domain !== 'all') q.set('domain', params.domain);
      if (params?.limit != null && params.limit > 0) q.set('limit', String(params.limit));
      if (params?.offset != null && params.offset >= 0) q.set('offset', String(params.offset));
      const qs = q.toString();
      const data = await request<{
        metadata: CapabilityMetadata[];
        count: number;
        total?: number;
        limit?: number;
        offset?: number;
      }>(`/capability-metadata${qs ? `?${qs}` : ''}`);
      const meta = data.metadata || [];
      const total = data.total ?? data.count ?? meta.length;
      return {
        metadata: meta,
        total,
        count: data.count ?? meta.length,
        limit: data.limit ?? meta.length,
        offset: data.offset ?? 0,
      };
    } catch {
      return { metadata: [], total: 0, count: 0, limit: 0, offset: 0 };
    }
  },

  getPolicyTemplates: async (): Promise<PolicyTemplateRow[]> => {
    try {
      const data = await request<{ templates: PolicyTemplateRow[] }>('/policy/templates');
      return data.templates || [];
    } catch {
      return [];
    }
  },

  getPolicyInstances: async (): Promise<PolicyInstanceRow[]> => {
    try {
      const data = await request<{ instances: PolicyInstanceRow[] }>('/policy/instances');
      return data.instances || [];
    } catch {
      return [];
    }
  },

  createPolicyTemplate: async (tpl: Omit<PolicyTemplateRow, 'id'>): Promise<PolicyTemplateRow> => {
    return request<PolicyTemplateRow>('/policy/templates', {
      method: 'POST',
      body: JSON.stringify(tpl),
    });
  },

  updatePolicyTemplate: async (
    templateId: string,
    version: string,
    updates: Partial<PolicyTemplateRow>,
  ): Promise<PolicyTemplateRow> => {
    return request<PolicyTemplateRow>(`/policy/templates/${encodeURIComponent(templateId)}/${encodeURIComponent(version)}`, {
      method: 'PUT',
      body: JSON.stringify(updates),
    });
  },

  deletePolicyTemplate: async (templateId: string, version: string): Promise<void> => {
    await request(`/policy/templates/${encodeURIComponent(templateId)}/${encodeURIComponent(version)}`, {
      method: 'DELETE',
    });
  },

  createPolicyInstance: async (inst: Omit<PolicyInstanceRow, 'id'>): Promise<PolicyInstanceRow> => {
    return request<PolicyInstanceRow>('/policy/instances', {
      method: 'POST',
      body: JSON.stringify(inst),
    });
  },

  updatePolicyInstance: async (
    instanceName: string,
    updates: Partial<PolicyInstanceRow>,
  ): Promise<PolicyInstanceRow> => {
    return request<PolicyInstanceRow>(`/policy/instances/${encodeURIComponent(instanceName)}`, {
      method: 'PUT',
      body: JSON.stringify(updates),
    });
  },

  deletePolicyInstance: async (instanceName: string): Promise<void> => {
    await request(`/policy/instances/${encodeURIComponent(instanceName)}`, {
      method: 'DELETE',
    });
  },

  getCapabilityMetadataById: async (capabilityId: string): Promise<CapabilityMetadata | null> => {
    try {
      const data = await request<CapabilityMetadata>(`/capability-metadata/${encodeURIComponent(capabilityId)}`);
      return data;
    } catch (err) {
      return null;
    }
  },

  // Phase 2.2: Attack Steps
  getPodAttackStepsStrict: async (podUid: string): Promise<PodAttackStep[]> => {
    if (!podUid) return [];
    const data = await request<{ podUid: string; steps: PodAttackStep[]; count: number }>(`/risk/pods/${encodeURIComponent(podUid)}/attack-steps`);
    return data.steps || [];
  },

  getPodAttackSteps: async (podUid: string): Promise<PodAttackStep[]> => {
    try {
      return await api.getPodAttackStepsStrict(podUid);
    } catch {
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
  getRuntimeSignals: async (params?: { clusterId?: string; search?: string; sort?: string; podUid?: string; signalType?: string; category?: string; startDate?: string; endDate?: string; sinceMinutes?: number; limit?: number; offset?: number }): Promise<{ signals: RuntimeSignal[]; count: number; total: number }> => {
    const queryParams = new URLSearchParams();
    if (params?.clusterId) queryParams.append('clusterId', params.clusterId);
    if (params?.search) queryParams.append('search', params.search);
    if (params?.sort) queryParams.append('sort', params.sort);
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

  // Attack Analysis Graph
  getAttackPathsGraphStrict: async (): Promise<AttackPathGraphData> => {
    const data = await request<{ data: { nodes: any[]; links: any[] } }>('/graph/attack-paths/graph');
    return mapRawAttackPathGraphPayload(data.data);
  },

  getAttackPathsGraph: async (): Promise<AttackPathGraphData> => {
    try {
      return await api.getAttackPathsGraphStrict();
    } catch (err) {
      // If API fails, return empty graph (no mock data)
      return { nodes: [], links: [] };
    }
  },

  /** One round-trip: graph + summary + chains + objectives (Core ≥ bundle route). */
  getAttackPathsBundleStrict: async (clusterId?: string): Promise<AttackPathsPageBundle | null> => {
    const query = clusterId ? `?cluster_id=${encodeURIComponent(clusterId)}` : '';
    const data = await request<{
      data: {
        graph?: { nodes: any[]; links: any[] };
        summary: AttackPathSummary | null;
        chains: AttackChain[];
        objectives: AttackObjective[];
        paths?: AttackPath[];
      };
    }>(`/graph/attack-paths/bundle${query}`);
    const d = data.data;
    if (!d) return null;
    return {
      graph: mapRawAttackPathGraphPayload(d.graph),
      summary: d.summary ?? null,
      chains: normalizeAttackChains(d.chains),
      objectives: d.objectives || [],
      paths: normalizeAttackPaths(d.paths),
    };
  },

  getAttackPathsBundle: async (clusterId?: string): Promise<AttackPathsPageBundle | null> => {
    try {
      return await api.getAttackPathsBundleStrict(clusterId);
    } catch {
      return null;
    }
  },

  // Attack Analysis Summary
  getAttackPathsSummaryStrict: async (clusterId?: string): Promise<AttackPathSummary | null> => {
    const query = clusterId ? `?cluster_id=${encodeURIComponent(clusterId)}` : '';
    const data = await request<{ data: AttackPathSummary }>(`/graph/attack-paths/summary${query}`);
    return data.data || null;
  },

  getAttackPathsSummary: async (clusterId?: string): Promise<AttackPathSummary | null> => {
    try {
      return await api.getAttackPathsSummaryStrict(clusterId);
    } catch {
      return null;
    }
  },

  getAttackPathObjectives: async (clusterId?: string): Promise<AttackObjective[]> => {
    try {
      const query = clusterId ? `?cluster_id=${encodeURIComponent(clusterId)}` : '';
      const data = await request<{ data: AttackObjective[] }>(`/graph/attack-paths/objectives${query}`);
      return asArray<AttackObjective>(data.data);
    } catch {
      return [];
    }
  },

  getAttackChainsStrict: async (clusterId?: string): Promise<AttackChain[]> => {
    const query = clusterId ? `?cluster_id=${encodeURIComponent(clusterId)}` : '';
    const data = await request<{ data: AttackChain[] }>(`/graph/attack-paths/chains${query}`);
    return normalizeAttackChains(data.data);
  },

  getAttackChains: async (clusterId?: string): Promise<AttackChain[]> => {
    try {
      return await api.getAttackChainsStrict(clusterId);
    } catch {
      return [];
    }
  },

  // Attack Analysis for a specific pod
  getAttackPathsForPodStrict: async (podUid: string): Promise<AttackPath[]> => {
    const data = await request<{ paths: AttackPath[]; count: number }>(`/graph/attack-paths/${encodeURIComponent(podUid)}`);
    return normalizeAttackPaths(data.paths);
  },

  getAttackPathsForPod: async (podUid: string): Promise<AttackPath[]> => {
    try {
      return await api.getAttackPathsForPodStrict(podUid);
    } catch {
      return [];
    }
  },

  // ---------------------------------------------------------------------------
  // Phase 3.5: Unified Risk Pipeline — API functions
  // ---------------------------------------------------------------------------

  /**
   * getUnifiedRiskScore returns the V3 unified risk score for a pod/resource.
   * Calls GET /api/v1/risk/scores/:uid — pass cluster when known so reads are scoped (avoids ambiguous rows).
   */
  getUnifiedRiskScore: async (uid: string, clusterId?: string): Promise<UnifiedRiskScore | null> => {
    try {
      const qs =
        clusterId != null && String(clusterId).trim() !== ''
          ? `?cluster=${encodeURIComponent(String(clusterId).trim())}`
          : '';
      const data = await request<UnifiedRiskScore & Record<string, unknown> & { data?: UnifiedRiskScore }>(
        `/risk/scores/${encodeURIComponent(uid)}${qs}`,
      );
      const obj = (data?.data ?? data) as (UnifiedRiskScore & Record<string, unknown>) | null;
      if (!obj || typeof obj !== 'object') return null;
      const total = Number(obj.totalScore ?? 0);
      const flRaw = obj.final_level ?? obj.finalLevel;
      const finalLevel = (flRaw != null
        ? String(flRaw).toLowerCase()
        : deriveUnifiedRiskLevelFromScore(total)) as UnifiedRiskScore['finalLevel'];
      return { ...obj, finalLevel };
    } catch {
      return null;
    }
  },

  /**
   * getTopRiskyPods returns the top N pods by unified risk score (V3 rows only, backend).
   * Optional clusterId matches dashboard scope; omit for global top N.
   * Uses GET /api/v1/risk/scores?sortBy=score&page=1&pageSize=N[&cluster=...]
   */
  getTopRiskyPods: async (limit: number = 5, clusterId?: string): Promise<UnifiedRiskScore[]> => {
    try {
      const clusterQs =
        clusterId && String(clusterId).trim() !== ''
          ? `&cluster=${encodeURIComponent(String(clusterId).trim())}`
          : '';
      const data = await request<{ data?: UnifiedRiskScore[]; scores?: UnifiedRiskScore[] }>(
        `/risk/scores?sortBy=score&page=1&pageSize=${limit}${clusterQs}`,
        { cache: 'no-store' }
      );
      const raw = Array.isArray(data?.data) ? data.data : Array.isArray(data?.scores) ? data.scores : [];
      return raw.map((row) => ({
        ...row,
        finalLevel: row.finalLevel ?? deriveUnifiedRiskLevelFromScore(row.totalScore),
      }));
    } catch {
      return [];
    }
  },

  /**
   * getPipelineHealth returns the status of each layer of the Unified Risk Pipeline.
   * Calls GET /api/v1/monitoring/pipeline-health
   */
  getPipelineHealth: async (): Promise<PipelineHealth | null> => {
    try {
      const data = await request<{ data: PipelineHealth }>('/monitoring/pipeline-health');
      return data.data || null;
    } catch {
      return null;
    }
  },

  /**
   * getEnrichedAttackPaths returns PCE-enriched attack paths for a pod.
   * Calls GET /api/v1/graph/attack-paths/:podUid (same endpoint, enrichedFromPce field added)
   */
  getEnrichedAttackPaths: async (podUid: string): Promise<EnrichedAttackPath[]> => {
    try {
      const data = await request<{ paths: EnrichedAttackPath[]; count: number }>(
        `/graph/attack-paths/${encodeURIComponent(podUid)}`
      );
      return data.paths || [];
    } catch {
      return [];
    }
  },

  listUserSessions: async (userId?: string): Promise<FortunaUserSession[]> => {
    const qs =
      userId != null && String(userId).trim() !== ''
        ? `?user_id=${encodeURIComponent(String(userId).trim())}`
        : '';
    const data = await request<{ sessions: Array<Record<string, unknown>> }>(`/sessions${qs}`);
    return (data.sessions || []).map((s) => ({
      id: String(s.id ?? ''),
      userId: String(s.userId ?? ''),
      issuedAt: s.issuedAt ? String(s.issuedAt) : '',
      expiresAt: s.expiresAt ? String(s.expiresAt) : '',
      revokedAt: s.revokedAt != null ? String(s.revokedAt) : undefined,
      lastActivityAt: s.lastActivityAt != null ? String(s.lastActivityAt) : undefined,
      sourceIp: s.sourceIp != null ? String(s.sourceIp) : undefined,
      authMethod: s.authMethod != null ? String(s.authMethod) : undefined,
      deviceFingerprint: s.deviceFingerprint != null ? String(s.deviceFingerprint) : undefined,
    }));
  },

  revokeUserSession: async (sessionId: string): Promise<void> => {
    await request(`/sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' });
  },

  revokeAllUserSessions: async (userId?: string): Promise<void> => {
    const qs =
      userId != null && String(userId).trim() !== ''
        ? `?user_id=${encodeURIComponent(String(userId).trim())}`
        : '';
    await request(`/sessions/revoke-all${qs}`, { method: 'DELETE' });
  },

  listSecurityActivity: async (params?: {
    limit?: number;
    offset?: number;
    action?: string;
    resourceType?: string;
    severity?: string;
    result?: string;
    actorUserId?: string;
    from?: string;
    to?: string;
    domain?: string;
    correlationId?: string;
    resourceId?: string;
    actionPrefix?: string;
  }): Promise<{ items: SecurityActivityItem[]; total: number; limit: number; offset: number }> => {
    const q = new URLSearchParams();
    if (params?.limit != null) q.set('limit', String(params.limit));
    if (params?.offset != null) q.set('offset', String(params.offset));
    if (params?.action?.trim()) q.set('action', params.action.trim());
    if (params?.resourceType?.trim()) q.set('resource_type', params.resourceType.trim());
    if (params?.severity?.trim()) q.set('severity', params.severity.trim());
    if (params?.result?.trim()) q.set('result', params.result.trim());
    if (params?.actorUserId?.trim()) q.set('actor_user_id', params.actorUserId.trim());
    if (params?.from?.trim()) q.set('from', params.from.trim());
    if (params?.to?.trim()) q.set('to', params.to.trim());
    if (params?.domain?.trim()) q.set('domain', params.domain.trim());
    if (params?.correlationId?.trim()) q.set('correlation_id', params.correlationId.trim());
    if (params?.resourceId?.trim()) q.set('resource_id', params.resourceId.trim());
    if (params?.actionPrefix?.trim()) q.set('action_prefix', params.actionPrefix.trim());
    const qs = q.toString();
    const path = qs ? `/governance/security-activity?${qs}` : '/governance/security-activity';
    const data = await request<{
      items: Array<Record<string, unknown>>;
      total?: number;
      limit?: number;
      offset?: number;
    }>(path);
    const items: SecurityActivityItem[] = (data.items || []).map((row) => ({
      id: typeof row.id === 'number' ? row.id : Number(row.id) || undefined,
      eventId: row.eventId != null ? String(row.eventId) : undefined,
      actorUserId: row.actorUserId != null ? Number(row.actorUserId) : undefined,
      actorUsername: row.actorUsername != null ? String(row.actorUsername) : undefined,
      actorRole: row.actorRole != null ? String(row.actorRole) : undefined,
      permissionsJson: row.permissionsJson != null ? String(row.permissionsJson) : undefined,
      action: row.action != null ? String(row.action) : undefined,
      resource: row.resource != null ? String(row.resource) : undefined,
      resourceType: row.resourceType != null ? String(row.resourceType) : undefined,
      resourceId: row.resourceId != null ? String(row.resourceId) : undefined,
      result: row.result != null ? String(row.result) : undefined,
      severity: row.severity != null ? String(row.severity) : undefined,
      targetUserId:
        row.targetUserId === null || row.targetUserId === undefined
          ? undefined
          : Number(row.targetUserId),
      requestId: row.requestId != null ? String(row.requestId) : undefined,
      sessionId: row.sessionId != null ? String(row.sessionId) : undefined,
      correlationId: row.correlationId != null ? String(row.correlationId) : undefined,
      sourceIp: row.sourceIp != null ? String(row.sourceIp) : undefined,
      userAgent: row.userAgent != null ? String(row.userAgent) : undefined,
      authMethod: row.authMethod != null ? String(row.authMethod) : undefined,
      beforeStateJson: row.beforeStateJson != null ? String(row.beforeStateJson) : undefined,
      afterStateJson: row.afterStateJson != null ? String(row.afterStateJson) : undefined,
      detailsJson: row.detailsJson != null ? String(row.detailsJson) : undefined,
      createdAt: row.createdAt != null ? String(row.createdAt) : undefined,
    }));
    return {
      items,
      total: Number(data.total ?? 0),
      limit: Number(data.limit ?? params?.limit ?? 50),
      offset: Number(data.offset ?? params?.offset ?? 0),
    };
  },

  getGovernancePermissionExplorer: async (): Promise<{
    items: Array<{
      permission: string;
      level: string;
      destructive: boolean;
      riskBand: string;
      roles: string[];
      routeCount: number;
      routes: string[];
      graphTierExposure: boolean;
    }>;
  }> => {
    return await request('/governance/permission-explorer');
  },

  getGovernanceAccessReview: async (): Promise<{
    signals: Array<{ code: string; severity: string; userId?: number; username?: string; detail?: Record<string, unknown> }>;
    userTotal?: number;
  }> => {
    return await request('/governance/access-review');
  },

  getGovernanceCorrelationSignals: async (): Promise<{
    signals: Array<{ code: string; severity: string; detail?: Record<string, unknown> }>;
  }> => {
    return await request('/governance/correlation-signals');
  },

  getInvestigationEvents: async (params?: {
    limit?: number;
    offset?: number;
    domain?: string;
    action?: string;
    correlationId?: string;
    resourceId?: string;
  }): Promise<{ items: SecurityActivityItem[]; total: number; limit: number; offset: number }> => {
    const q = new URLSearchParams();
    if (params?.limit != null) q.set('limit', String(params.limit));
    if (params?.offset != null) q.set('offset', String(params.offset));
    if (params?.domain?.trim()) q.set('domain', params.domain.trim());
    if (params?.action?.trim()) q.set('action', params.action.trim());
    if (params?.correlationId?.trim()) q.set('correlation_id', params.correlationId.trim());
    if (params?.resourceId?.trim()) q.set('resource_id', params.resourceId.trim());
    const qs = q.toString();
    const path = qs ? `/governance/investigation-events?${qs}` : '/governance/investigation-events';
    const data = await request<{
      items: Array<Record<string, unknown>>;
      total?: number;
      limit?: number;
      offset?: number;
    }>(path);
    const items: SecurityActivityItem[] = (data.items || []).map((row) => ({
      id: typeof row.id === 'number' ? row.id : Number(row.id) || undefined,
      eventId: row.eventId != null ? String(row.eventId) : undefined,
      actorUserId: row.actorUserId != null ? Number(row.actorUserId) : undefined,
      actorUsername: row.actorUsername != null ? String(row.actorUsername) : undefined,
      actorRole: row.actorRole != null ? String(row.actorRole) : undefined,
      action: row.action != null ? String(row.action) : undefined,
      resourceType: row.resourceType != null ? String(row.resourceType) : undefined,
      resourceId: row.resourceId != null ? String(row.resourceId) : undefined,
      result: row.result != null ? String(row.result) : undefined,
      severity: row.severity != null ? String(row.severity) : undefined,
      correlationId: row.correlationId != null ? String(row.correlationId) : undefined,
      createdAt: row.createdAt != null ? String(row.createdAt) : undefined,
    }));
    return {
      items,
      total: Number(data.total ?? 0),
      limit: Number(data.limit ?? params?.limit ?? 100),
      offset: Number(data.offset ?? params?.offset ?? 0),
    };
  },

  listInvestigationCases: async (): Promise<{ items: InvestigationCaseApi[]; total: number }> => {
    const data = await request<{ items: InvestigationCaseApi[]; total: number }>('/investigations');
    return { items: data.items ?? [], total: Number(data.total ?? 0) };
  },

  getInvestigationCaseStats: async (): Promise<{ openCases: number; overdueRemediation: number }> => {
    return request<{ openCases: number; overdueRemediation: number }>('/investigations/stats');
  },

  getInvestigationCase: async (id: string): Promise<InvestigationCaseApi> => {
    return request<InvestigationCaseApi>(`/investigations/${encodeURIComponent(id)}`);
  },

  createInvestigationCase: async (body: {
    title?: string;
    owner?: string;
    clusterId?: string | null;
  }): Promise<InvestigationCaseApi> => {
    return request<InvestigationCaseApi>('/investigations', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },

  patchInvestigationCase: async (
    id: string,
    body: Partial<{
      title: string;
      status: string;
      owner: string;
      clusterId: string | null;
      slaDueAt: string | null;
      entities: InvestigationEntityApi[];
      notes: InvestigationNoteApi[];
      remediationActions: InvestigationRemediationApi[];
      collaboration: InvestigationCollaborationApi;
    }>,
  ): Promise<InvestigationCaseApi> => {
    return request<InvestigationCaseApi>(`/investigations/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    });
  },

  deleteInvestigationCase: async (
    id: string,
  ): Promise<{ archived: boolean; id: string; retentionUntil?: string }> => {
    return request<{ archived: boolean; id: string; retentionUntil?: string }>(
      `/investigations/${encodeURIComponent(id)}`,
      { method: 'DELETE' },
    );
  },

  pinInvestigationEntity: async (
    caseId: string,
    body: {
      type: string;
      label: string;
      href?: string;
      meta?: Record<string, string>;
    },
  ): Promise<{ pinned: boolean; case: InvestigationCaseApi }> => {
    return request<{ pinned: boolean; case: InvestigationCaseApi }>(
      `/investigations/${encodeURIComponent(caseId)}/pin`,
      { method: 'POST', body: JSON.stringify(body) },
    );
  },

  listInvestigationTimeline: async (
    caseId: string,
  ): Promise<{ items: InvestigationTimelineEntryApi[]; total: number }> => {
    const data = await request<{ items: InvestigationTimelineEntryApi[]; total: number }>(
      `/investigations/${encodeURIComponent(caseId)}/timeline`,
    );
    return { items: data.items ?? [], total: Number(data.total ?? 0) };
  },
};

export type InvestigationEntitySnapshotApi = {
  capturedAt: string;
  entity?: Record<string, unknown>;
  evidence?: Record<string, unknown>;
  score?: Record<string, unknown>;
  graph?: Record<string, unknown>;
};

export type InvestigationEntityApi = {
  id: string;
  type: string;
  label: string;
  href?: string;
  meta?: Record<string, string>;
  pinnedAt: string;
  snapshot?: InvestigationEntitySnapshotApi;
};

export type InvestigationNoteApi = {
  id: string;
  body: string;
  createdAt: string;
  author: string;
};

export type InvestigationRemediationApi = {
  id: string;
  title: string;
  status: string;
  owner: string;
  dueAt?: string;
  notes?: string;
  createdAt: string;
  externalSystem?: string;
  externalTicketUrl?: string;
  externalTicketKey?: string;
};

export type InvestigationHandoffNoteApi = {
  id: string;
  body: string;
  author: string;
  createdAt: string;
  mentions?: string[];
};

export type InvestigationCollaborationApi = {
  assignees: string[];
  watchers: string[];
  teamId?: string;
  handoffNotes: InvestigationHandoffNoteApi[];
};

export type InvestigationTimelineEntryApi = {
  id: number;
  eventId: string;
  caseId: string;
  eventType: string;
  summary: string;
  actorUserId: number;
  actorUsername: string;
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
  details?: Record<string, unknown>;
  createdAt: string;
};

export type InvestigationCaseApi = {
  id: string;
  title: string;
  status: string;
  owner: string;
  clusterId?: string | null;
  notes: InvestigationNoteApi[];
  entities: InvestigationEntityApi[];
  remediationActions: InvestigationRemediationApi[];
  collaboration?: InvestigationCollaborationApi;
  createdAt: string;
  updatedAt: string;
  slaDueAt?: string | null;
  archivedAt?: string | null;
  retentionUntil?: string | null;
  createdByUserId: number;
};
