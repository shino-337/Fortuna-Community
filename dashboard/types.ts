export interface Vulnerability {
  id: string; // CVE-ID
  severity: 'critical' | 'high' | 'medium' | 'low';
  description: string;
  fixedVersion?: string;
  cvssScore: number;
}

export interface SbomComponent {
  id: string | number;
  name: string;
  version: string;
  type: 'library' | 'os-package' | 'language-runtime' | string;
  language?: string;
  license?: string;
  purl?: string;
  vulnerabilities: Vulnerability[];
}

export interface PodSbomSummary {
  podId: string;
  podName: string;
  namespace: string;
  image: string;
  containerName?: string;
  lastScan: string;
  packageCount?: number;
  podCreatedAt?: string;   // pod creation time (from pods table when available)
  podStatus?: string;     // e.g. Running, Pending (when available)
  vulnerabilitySummary?: {
    critical: number;
    high: number;
    medium: number;
    low: number;
  };
}

export interface PodSbom {
  podId: string;
  podName: string;
  namespace: string;
  image: string;
  container?: string;
  generatedAt?: string;
  packageCount?: number;
  components: SbomComponent[];
}

// Cluster (from /api/v1/clusters or /api/v1/clusters/stats) – SSOT from DB, no hardcoded fallback
export interface Cluster {
  id: string;
  name: string;
  region?: string;
  endpoint?: string;
  status?: string;
  lastSync?: string;
  /** K8s version from agent (e.g. v1.29.1); prefer k8sVersion, fallback version for compat */
  version?: string;
  k8sVersion?: string;
  source?: string; // "auto" | "env"
  distribution?: string; // "eks" | "gke" | "aks" | "kubeadm" | "unknown"
  nodes?: number;
  pods?: number;
  /** From GET /clusters/stats */
  podCount?: number;
  deploymentCount?: number;
  riskCount?: number;   // Active insights for this cluster
  agentCount?: number;  // Agents on nodes in this cluster
  connectionStatus?: string; // "connected" | "degraded" | "disconnected"
  healthScore?: number; // derived on frontend if not from API
}

// Response shape from GET /api/v1/clusters/stats
export interface ClusterStatsResponse {
  clusters: ClusterStatsItem[];
  total: number;
}

export interface ClusterStatsItem extends Cluster {
  serviceAccountCount?: number;
  roleCount?: number;
  clusterRoleCount?: number;
  roleBindingCount?: number;
  clusterRoleBindingCount?: number;
  podCount?: number;
  deploymentCount?: number;
  connectionStatus?: string;
  agentVersion?: string;
}

/** GET /clusters/:id/overview */
export interface ClusterOverview {
  podCount: number;
  nodeCount: number;
  namespaceCount: number;
}

/** GET /clusters/:id/inventory */
export interface ClusterInventory {
  nodes: string[];
  namespaces: string[];
}

/** GET /clusters/:id/agents – agent item */
export interface ClusterAgent {
  agentId: string;
  nodeName?: string;
  status: string;
  lastHeartbeat: string;
  version?: string;
}

/** GET /clusters/:id/security-summary */
export interface ClusterSecuritySummary {
  riskBySeverity: Record<string, number>;
  capabilityCount: number;
  criticalCount: number;
  highCount: number;
  mediumCount: number;
  lowCount: number;
}

/** GET /clusters/:id/nodes/:nodeName – Node Detail (metadata + optional pods) */
export interface NodeDetailResponse {
  clusterId: string;
  nodeName: string;
  ip?: string;
  kubeletVersion?: string;
  role?: string;
  os?: string;
  runtime?: string;
  lastSeen?: string;
  podCount: number;
  pods?: Array<{ id: number; uid: string; name: string; namespace: string; riskCount: number }>;
}

/** GET /pods – pod with riskCount */
export interface PodWithRisk {
  id: number;
  clusterId: string;
  name: string;
  namespace: string;
  uid: string;
  nodeName?: string;
  serviceAccount?: string;
  riskCount: number;
}

export interface DashboardStats {
  totalClusters: number;
  activeAgents: number;
  runningPods: number;
  totalRisks: number;
  criticalRisks: number;
  resolved24h?: number;
  affectedPodCount?: number; // Affected Workloads: distinct pods with active insight
}

/** GET /api/v1/insights/summary – severity breakdown for dashboard cards */
export interface InsightsSummary {
  total: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  byType?: Record<string, number>;
}

export interface ThreatVelocityPoint {
  date: string;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface PodCapabilitySummaryCluster {
  clusterId: string;
  count: number;
}

export interface PodCapabilitySummaryCapability {
  capabilityId: string;
  severity: string;
  count: number;
}

export interface PodCapabilitySummaryNamespace {
  namespace: string;
  severity: string;
  count: number;
}

export interface PodCapabilitySummarySeverity {
  severity: string;
  count: number;
}

export interface PodCapabilityTrendPoint {
  date: string;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface PodCapabilityDetail {
  podUid: string;
  podName?: string;
  namespace: string;
  capabilityId: string;
  group: string;
  severity: string;
  state?: 'detected' | 'confirmed' | 'exploited' | 'chained';
  confidence?: number;
  evidence: Record<string, unknown>;
  mitreTechniques?: string[];
  createdAt?: string;
  updatedAt?: string;
}

// Phase 2.2: Capability Metadata
export interface CapabilityMetadata {
  capabilityId: string;
  domain: string;
  category: string;
  description: string;
  severityBase: string;
  confidenceBase: number;
  preconditions?: string[];
  producesAttackSteps?: string[];
  expiresWithInstance: boolean;
  supportsRuntimePromotion: boolean;
}

// Phase 2.2: Attack Steps
export interface PodAttackStep {
  podUid: string;
  stepId: string;
  description?: string;
  category: string;
  confidence: number;
  evidence?: Record<string, unknown>;
  createdAt?: string;
  updatedAt?: string;
}

export interface AttackStepSummary {
  stepId: string;
  category: string;
  count: number;
  avgConfidence: number;
}

// Phase 2.3: Promotion Rules
export interface PromotionRule {
  id: number;
  capabilityId: string;
  signalType: string;
  minOccurrences: number;
  requiredCapabilities?: string[];
  promoteTo: 'detected' | 'confirmed' | 'exploited' | 'chained';
  confidenceBoost: number;
  createdAt?: string;
  updatedAt?: string;
}

// Phase 2.3: Runtime Signals
export interface RuntimeSignal {
  id: number;
  podUid: string;
  signalType: string;
  category: string;
  confidence: number;
  evidence: Record<string, unknown>;
  createdAt: string;
}

// Phase 2.3: Capability State History
export interface CapabilityStateHistory {
  capabilityId: string;
  state: 'detected' | 'confirmed' | 'exploited' | 'chained';
  confidence: number;
  timestamp: string;
}

// Operations / monitoring (from API – real data)
export interface ErrorLog {
  id: string;
  time: string;
  level: string;
  message: string;
  source?: string;
}

export interface Certificate {
  id: string;
  name: string;
  daysRemaining?: number;
}

export interface Agent {
  id: string;
  node: string;
  status: string;
  lastHeartbeat: string;
}

export interface QueueMetric {
  timestamp: string;
  normalizer: number;
  correlator: number;
  risk: number;
}

export interface SyncStatus {
  lastScan: string;
  nextScan: string;
  drift: boolean;
  resources: { pods: number; sas: number; roles?: number; bindings?: number };
}

export interface User {
  id: string;
  username: string;
  role?: string;
}

export interface Notification {
  id: string;
  title: string;
  message: string;
  severity?: string;
  timestamp?: string;
}

export interface AuditLog {
  id: string;
  action: string;
  resource: string;
  timestamp: string;
  user?: string;
}

export interface Report {
  id: string;
  name: string;
  generatedAt: string;
}

export interface SecurityRule {
  id: string;
  name: string;
  severity: string;
  enabled: boolean;
}

export interface K8sResource {
  id: string;
  name: string;
  namespace: string;
  kind: string;
  age?: string;
  status?: string;
}

export interface RotationEvent {
  id: string;
  timestamp: string;
  success: boolean;
}

export interface Insight {
  id: string;           // PK from API (used for GET /insights/:id and route /risks/:id)
  cveId?: string;       // CVE identifier for display (e.g. CVE-2024-123)
  title: string;
  description?: string;
  severity: string;
  score?: number;
  category?: string;
  /** Insight type from API (e.g. vulnerability, rbac) – for Risk Center Type column */
  insightType?: string;
  status?: string;
  timestamp?: string;
  clusterId?: string;
  clusterName?: string;
  affectedResources?: Array<{ id: string; name?: string; kind?: string; namespace?: string }>;
  impact?: string;
  resolvedAt?: string;
  updatedAt?: string;
  /** Risk Detail: evidence (JSON from backend insights.evidence) */
  evidence?: Record<string, unknown> | string;
  /** Risk Detail: violated rules (JSON from backend insights.violated_rules) */
  violatedRules?: unknown[] | Record<string, unknown> | string;
}
