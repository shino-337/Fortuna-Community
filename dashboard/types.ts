export interface Vulnerability {
  id: string; // CVE-ID
  severity: 'critical' | 'high' | 'medium' | 'low';
  description: string;
  fixedVersion?: string;
  cvssScore: number;
  status?: 'active' | 'allowed' | 'fixed';
  exploitKnown?: boolean;
  exploitMaturity?: string;
  allowed?: boolean;
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
  cveCount?: number;
  maxSeverity?: string;
  maxCvss?: number;
  fixVersion?: string;
  status?: string;
  malwareMatch?: MalwareMatch;
}

export interface MalwareMatch {
  reason: 'MALWARE' | 'TELEMETRY' | string;
  confidence: number;
  malwareFamily?: string;
}

export interface ThreatSummary {
  totalThreats: number;
  malwareCount: number;
  telemetryCount: number;
  protestwareCount: number;
  highestSeverity: string;
  affectedPackages: { name: string; version: string; reason: string }[];
  requiresAction: boolean;
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
  podStatus?: string;     // K8s phase from pods table when synced (Running, Pending, …)
  /** parsers | distroless-heuristic | label-metadata */
  sbomSource?: string;
  /** low | medium | high */
  confidence?: string;
  /** Go toolchain / stdlib context from agent (when applicable) */
  goVersion?: string;
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
  vulnerablePackageCount?: number;
  vulnerabilitySummary?: { critical: number; high: number; medium: number; low: number };
  components: SbomComponent[];
  /** Finding #8.4: parsers | distroless-heuristic | label-metadata – for badge "Distroless SBOM (heuristic)" */
  sbomSource?: string;
  /** low | medium | high */
  confidence?: string;
  /** Go version (stdlib / image config) used for matcher context */
  goVersion?: string;
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

/** GET /pods, GET /pods/by-id/:id, GET /pods/by-uid/:uid – pod with riskCount (POD_DETAIL_SPEC fields when available) */
export interface PodWithRisk {
  id: number;
  clusterId: string;
  name: string;
  namespace: string;
  uid: string;
  nodeName?: string;
  serviceAccount?: string;
  /** Pod phase from API (phase): Running, Pending, Succeeded, Failed, Unknown */
  status?: string;
  phase?: string;
  riskCount: number;
  createdAt?: string;
  /** Pod Detail (POD_DETAIL_SPEC): header and overview */
  podIP?: string;
  startTime?: string | null;
  restartCount?: number;
  ownerKind?: string;
  ownerName?: string;
  replicaSetName?: string;
  qosClass?: string;
}

export interface DashboardStats {
  totalClusters: number;
  activeAgents: number;
  runningPods: number;
  totalRisks: number;
  criticalRisks: number;
  resolved24h?: number;
  affectedPodCount?: number; // Affected Workloads: distinct pods with active insight
  /** When clusterId filter is set: display name from K8s (via agent sync). Use for labels, not hash. */
  clusterName?: string;
}

/** GET /api/v1/insights/summary – severity breakdown for dashboard cards */
export interface InsightsSummaryByClusterItem {
  clusterId: string;
  clusterName?: string;
  total: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

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

/** GET /risk/histogram – Risk Score Distribution (bins 0–100) for histogram chart */
export interface RiskHistogramBin {
  bin: number;
  count: number;
  percent: number;
  severityBreakdown: { critical: number; high: number; medium: number; low: number };
  critical_count: number;
  high_count: number;
  medium_count: number;
  low_count: number;
}

export interface RiskHistogramResponse {
  bins: RiskHistogramBin[];
  totalFindings: number;
  averageScore: number;
  p0Count: number;
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
  /** RFC3339; for PCE drill-down "Last Seen" column (backend last_seen_at) */
  lastSeenAt?: string;
}

// Phase 2.2: Capability Metadata (extended per Capability Specification – MITRE ATT&CK)
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
  // Extended (spec)
  name?: string;
  summary?: string;
  fullDescription?: string;
  mitreTactic?: string;
  mitreTechnique?: string;
  mitreSubtechnique?: string;
  killChainStage?: string;
  technicalIndicators?: string[];
  impact?: string[];
  recommendedMitigations?: string[];
  falsePositiveConsiderations?: string[];
  references?: string[];
}

// Pod Detail services (runtime-metrics, processes, network-connections, events)
export interface PodRuntimeMetric {
  id?: number;
  podUid?: string;
  containerName: string;
  cpuUsageMillicore?: number;
  memoryUsageBytes?: number;
  memoryLimitBytes?: number;
  restartCount?: number;
  state?: string;
  lastObservedAt?: string;
}

export interface PodProcessItem {
  id?: number;
  podUid?: string;
  containerName: string;
  pid: number;
  ppid?: number;
  userName?: string;
  userId?: number;
  groupId?: number;
  cpuPercent?: number;
  memoryPercent?: number;
  command?: string;
  binaryPath?: string;
  workingDir?: string;
  capEff?: string;
  startedAt?: string;
  observedAt?: string;
  /** "host" | "exec" - Runtime Source indicator (Host Inspection vs Container Exec) */
  runtimeSource?: string;
}

export interface PodNetworkConnectionItem {
  id?: number;
  podUid?: string;
  containerName?: string;
  sourceIp?: string;
  sourcePort?: number;
  destIp?: string;
  destPort?: number;
  protocol?: string;
  state?: string;
  /** tx_queue bytes from /proc/net/* snapshot (not cumulative bytes sent) */
  bytesSent?: number;
  /** rx_queue bytes from /proc/net/* snapshot (not cumulative bytes received) */
  bytesRecv?: number;
  /** Start of 5-minute UTC bucket for this row (dedupe window) */
  bucket5m?: string;
  observedAt?: string;
  /** "host" | "exec" - Runtime Source indicator */
  runtimeSource?: string;
}

/** Aggregated remote endpoints for a pod (GET /runtime/pods/:uid/network/top-destinations) — optional API, UI chính dùng Network activity. */
export interface PodNetworkTopDestinationItem {
  destIp: string;
  destPort: number;
  protocol: string;
  /** Rows in window (each row is one signature × 5m bucket observation) */
  observationCount: number;
  lastObservedAt?: string;
  /** How many distinct 5m buckets contributed */
  distinctBucketCount?: number;
}

/** Cluster-wide aggregate by dest (GET /runtime/network-activity?view=destinations) */
export interface NetworkActivityDestinationRow {
  destIp: string;
  destPort: number;
  protocol: string;
  observationCount: number;
  lastObservedAt?: string;
  distinctBucketCount?: number;
  distinctPodCount: number;
  /** pods.pod_ip = dest_ip (pod-to-pod); không phải ClusterIP Service */
  destWorkloadName?: string;
  destWorkloadNamespace?: string;
}

/** Cluster-wide aggregate by source pod (GET /runtime/network-activity?view=talkers) */
export interface NetworkActivityTalkerRow {
  podUid: string;
  namespace: string;
  clusterId: string;
  podName?: string;
  ownerKind?: string;
  ownerName?: string;
  nodeName?: string;
  observationCount: number;
  lastObservedAt?: string;
  distinctBucketCount?: number;
  /** Số tuple đích (IP|port|proto) khác nhau */
  distinctDestCount: number;
}

/** Aggregated per-pod row from GET /runtime/network-activity?view=pods */
export interface NetworkActivityWorkloadRow {
  podUid: string;
  namespace: string;
  clusterId: string;
  connectionCount: number;
  lastObservedAt?: string;
  podName?: string;
  ownerKind?: string;
  ownerName?: string;
  nodeName?: string;
}

/** Flat connection row from GET /runtime/network-activity (join pods for names) */
export interface NetworkActivityConnectionRow extends PodNetworkConnectionItem {
  clusterId?: string;
  namespace?: string;
  podName?: string;
  ownerKind?: string;
  ownerName?: string;
  nodeName?: string;
  /** view=edges: số dòng pod_network_connections gom trong nhóm (pod×đích×proto) */
  observationCount?: number;
}

export interface PodK8sEventItem {
  id?: number;
  eventUid?: string;
  namespace: string;
  eventName?: string;
  involvedKind?: string;
  involvedUid?: string;
  involvedName?: string;
  reason?: string;
  message?: string;
  eventType?: string;
  count?: number;
  lastTimestamp?: string;
}

/** GET /risk/pods/:uid/runtime/events — raw runtime_events (Falco, eBPF, agent diffs, …) */
export interface PodRuntimeSecurityEvent {
  id: number;
  podUid: string;
  podName?: string;
  namespace: string;
  nodeName?: string;
  runtime?: string;
  eventType?: string;
  signal?: string;
  mitreTechnique?: string;
  severity?: string;
  syscall: string;
  targetPath: string;
  capability: string;
  createdAt: string;
}

/** GET /api/v2/runtime/pods/:uid/facts */
export interface PodRuntimeBehaviorFact {
  id: number;
  factId: string;
  podUid: string;
  namespace: string;
  factType: string;
  domain: string;
  observedAt?: string;
  createdAt?: string;
}

/** GET /api/v2/runtime/pods/:uid/incidents */
export interface PodRuntimeIncident {
  id: number;
  incidentId: string;
  podUid: string;
  namespace: string;
  incidentType: string;
  severityHint?: string;
  confidence?: number;
  firstSeenAt?: string;
  lastSeenAt?: string;
  window?: string;
  createdAt?: string;
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

// Attack Paths (Phase 3 — graph visualization)
export interface AttackPathNode {
  id: string;
  type: string; // Pod, ServiceAccount, RoleBinding, ClusterRoleBinding, Role, ClusterRole
  properties: Record<string, unknown>;
}

export interface AttackPathEdge {
  type: string; // USES_SERVICE_ACCOUNT, REFERENCED_BY, GRANTS_ROLE
  source: string;
  target: string;
  properties?: Record<string, unknown>;
}

export interface AttackPath {
  nodes: AttackPathNode[];
  edges: AttackPathEdge[];
  total_risk: number;
  difficulty: number;
  impact: number;
  length: number;
  description: string;
}

export interface AttackPathSummary {
  totalPaths: number;
  criticalPaths: number;
  highPaths: number;
  mediumPaths: number;
  targetBreakdown: Record<string, number>;
  topPods: Array<{
    uid: string;
    name: string;
    namespace: string;
    service_account_name: string;
    risk_score: number;
    risk_reason: string;
  }>;
}

export interface AttackPathGraphData {
  nodes: Array<{ id: string; label: string; type: string; risk?: string; [key: string]: unknown }>;
  links: Array<{ source: string; target: string; type?: string; value: number }>;
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

export interface RuntimeSignalSuppressionStats {
  sinceMinutes: number;
  emittedEvents: number;
  uniqueKeys: number;
  maxRatio: number;
  perKey: Record<string, number>;
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
  status?: 'valid' | 'warning' | 'expired' | string;
  issuer?: string;
  subject?: string;
  serialNumber?: string;
  expiryDate?: string;
  usage?: string[];
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
  name?: string;
  username: string;
  email?: string;
  role?: string;
  active?: boolean;
  status?: 'active' | 'disabled' | string;
}

export interface Notification {
  id: string;
  title: string;
  message: string;
  severity?: string;
  type?: string;
  source?: string;
  read?: boolean;
  readAt?: string;
  timestamp?: string;
}

export interface AuditLog {
  id: string;
  action: string;
  resource: string;
  resourceId?: string;
  timestamp: string;
  user?: string;
  actor?: string;
  ip?: string;
  status?: 'success' | 'failure' | 'denied' | string;
  details?: string;
}

export interface Report {
  id: string;
  name: string;
  generatedAt: string;
  resource?: string;
  action?: string;
  count?: number;
  title?: string;
  type?: string;
  status?: string;
}

export interface SecurityRule {
  id: string;
  name: string;
  severity: string;
  enabled: boolean;
  category?: string;
  type?: string;
  description?: string;
  logic?: string;
  evalTime?: string;
  lastUpdated?: string;
  matches?: number;
  lastMatchedAt?: string;
  source?: 'db' | 'files' | 'built-in' | string;
  signature?: string;
  overlapGroup?: string;
  isCanonical?: boolean;
  canonicalRuleId?: string;
  impactedFindings24h?: number;
  impactedFindings7d?: number;
  relatedCapabilities?: string[];
}

/** Risk rule from DB (GET /risk-rules CRUD). */
export interface RiskRuleItem {
  id: string;
  name: string;
  severity: string;
  description?: string;
  category?: string;
  enabled: boolean;
  file?: string;
}

/** Condition inside a risk rule. */
export interface RiskRuleCondition {
  type?: string;
  field?: string;
  operator?: string;
  value?: unknown;
  expression?: string;
}

/** Full risk rule for create/update (POST/PUT /risk-rules). */
export interface RiskRuleFull {
  id: string;
  name: string;
  severity: string;
  description?: string;
  category?: string;
  enabled: boolean;
  conditions: RiskRuleCondition[];
  aggregation?: string;
  base_score?: number;
  tags?: string[];
}

export interface K8sResource {
  id: string;   // UID from API (used for Identity/Role links)
  name: string;
  namespace: string;
  kind: string;
  clusterId?: string;
  age?: string;
  status?: string;
}

export interface RotationEvent {
  id: string;
  timestamp: string;
  success: boolean;
}

/** Structured refs from Core GET /risk/insights* (snake_case JSON). */
export interface InsightEvidenceRefsPayload {
  eventIds?: string[];
  factIds?: string[];
  signalTypes?: string[];
  incidentTypes?: string[];
  capabilityIds?: string[];
  ruleIds?: string[];
}

/** Ordered pipeline layers for explainability (G-EXP-01 MVP). */
export interface InsightExplanationChainStep {
  layer: string;
  refs: string[];
}

/** Resolved fact rows when GET /risk/insights/:id?enrich=1 */
export interface InsightEnrichedFactRef {
  factId: string;
  factType?: string;
  domain?: string;
  observedAt?: string;
}

export interface InsightEnrichedRefs {
  facts?: InsightEnrichedFactRef[];
}

export interface Insight {
  id: string;           // PK from API (used for GET /insights/:id and route /risks/:id)
  cveId?: string;       // CVE or internal dedup key (e.g. CVE-2024-123, supply-malware:pkg@ver)
  /** Populated from API for supply_chain_malware rows */
  affectedComponent?: string;
  affectedVersion?: string;
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
  /** Risk Detail: structured explanation text from backend insights.risk_explanation */
  riskExplanation?: string;
  /** Risk Detail: structured remediation JSON from backend insights.remediation */
  remediation?: unknown[] | Record<string, unknown> | string;
  /** Phase 3.1: from risk_scores when GET /risks?withScores=1 */
  totalScore?: number;
  priorityLevel?: string;
  /** Phase 3.2: V2 scoring breakdown from risk_scores (when withScores=1) */
  exploitabilityScore?: number;
  businessImpactScore?: number;
  timeDecay?: number;
  evidence_refs?: InsightEvidenceRefsPayload;
  explanation_chain?: InsightExplanationChainStep[];
  enriched_refs?: InsightEnrichedRefs;
}

/** Summary from GET /risk/pods/:uid/report — DB/agent/runtime alignment */
export interface PodRiskReportSummary {
  runtimeSignals24h?: number;
  podDirectInsightCount?: number;
  runtimePolicyInsightCount?: number;
  insightsInReport?: number;
  clusterAdminBindings?: number;
  wildcardRoles?: number;
  overprivilegedRoles?: number;
  riskLevel?: string;
}
