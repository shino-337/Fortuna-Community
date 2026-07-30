/** CVE vulnerability record with severity, CVSS score and remediation state. */
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

/** SBOM component (library, OS package or language runtime) with its vulnerabilities. */
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

/** Malware detection match with confidence and family attribution. */
export interface MalwareMatch {
  reason: 'MALWARE' | 'TELEMETRY' | string;
  confidence: number;
  malwareFamily?: string;
}

/** Image trust signals are separate from package CVEs. */
export interface ImageTrust {
  registry: string;
  registryClass: 'trusted' | 'blocked' | 'public' | 'internal' | 'unknown' | string;
  digestAvailable: boolean;
  mutableTag: boolean;
  status: 'strong' | 'weak' | 'blocked' | 'unknown' | string;
  evidence?: string[];
  warnings?: string[];
}

/** Threat summary for a pod or cluster (CVEs, malware, protestware). */
export interface ThreatSummary {
  totalThreats: number;
  malwareCount: number;
  telemetryCount: number;
  protestwareCount: number;
  highestSeverity: string;
  affectedPackages: { name: string; version: string; reason: string }[];
  requiresAction: boolean;
}

/** SBOM summary for a pod (scan metadata, package count and vulnerability counts). */
export interface PodSbomSummary {
  podId: string;
  podName: string;
  namespace: string;
  image: string;
  imageDigest?: string;
  imageTrust?: ImageTrust;
  containerName?: string;
  lastScan: string;
  packageCount?: number;
  podCreatedAt?: string; // pod creation time (from pods table when available)
  podStatus?: string; // K8s phase from pods table when synced (Running, Pending, …)
  activePod?: boolean;
  lifecycleState?: 'current' | 'stale' | string;
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
  imageDigest?: string;
  imageTrust?: ImageTrust;
  container?: string;
  generatedAt?: string;
  packageCount?: number;
  vulnerablePackageCount?: number;
  vulnerabilitySummary?: {
    critical: number;
    high: number;
    medium: number;
    low: number;
  };
  components: SbomComponent[];
  activePod?: boolean;
  lifecycleState?: 'current' | 'stale' | string;
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
  riskCount?: number; // Active insights for this cluster
  agentCount?: number; // Agents on nodes in this cluster
  connectionStatus?: string; // "connected" | "degraded" | "disconnected"
  healthScore?: number; // derived on frontend if not from API
  /** From GET /inventory/clusters/stats (Kubernetes RBAC inventory counts) */
  serviceAccountCount?: number;
  roleCount?: number;
  clusterRoleCount?: number;
  roleBindingCount?: number;
  clusterRoleBindingCount?: number;
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
  pods?: Array<{
    id: number;
    uid: string;
    name: string;
    namespace: string;
    riskCount: number;
  }>;
}

/** Graph→resource interpretation from Core (attack path projection); does not replace unified score. */
export interface ResourceRiskSignals {
  hasAttackPath: boolean;
  isEntryPoint: boolean;
  isPivot: boolean;
  maxImpact: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | string;
  /** Chain id that determined max_impact (confidence-filtered subset). */
  maxImpactSourceChainId?: string;
  /** Distinct attack path ids across touching chains. */
  pathCount?: number;
  summary: string;
  chainIds: string[];
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
  /** Kubernetes ServiceAccount UID resolved from inventory when available. */
  serviceAccountUid?: string;
  /** Pod phase from API (phase): Running, Pending, Succeeded, Failed, Unknown */
  status?: string;
  phase?: string;
  riskCount: number;
  /** From GET /inventory/pods when risk_scores v3 row exists (authoritative; same family as GET /risk/scores/:uid). */
  unifiedScore?: number;
  /** ADR band: low | medium | high | critical — derived from unifiedScore on server. */
  finalLevel?: string;
  scorerVersion?: string;
  /** Unified V3 total (alias of unifiedScore when API sends total_score). */
  totalScore?: number;
  /** When present (cluster-scoped list + graph): attack path semantics for the workload row. */
  riskSignals?: ResourceRiskSignals;
  /** Short attack-path hint from Core (persisted description or chain step names). */
  pathPreview?: string;
  /** Distinct graph entities on touching chains (pods, identities, …). */
  blastEntityCount?: number;
  /** Advisory % risk reduction if remediated (heuristic from unified score). */
  riskFixHintPct?: number;
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

/** Kubernetes ServiceAccount inventory row. */
export interface K8sServiceAccount {
  id: number;
  clusterId: string;
  name: string;
  namespace: string;
  uid: string;
  labels?: string;
  secrets?: string;
  linkedPods?: string;
  lastUsed?: string | null;
  createdAt?: string;
  updatedAt?: string;
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

/** GET /api/v1/risk/insights/summary — severity breakdown for dashboard cards */
export interface InsightsSummaryByClusterItem {
  clusterId: string;
  clusterName?: string;
  total: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface RiskLevelCounts {
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
  riskLevelCounts?: RiskLevelCounts;
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
  severityBreakdown: {
    critical: number;
    high: number;
    medium: number;
    low: number;
  };
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

/** Aggregated remote endpoints for a pod (GET /runtime/pods/:uid/network/top-destinations) — optional API; primary UI is Network activity. */
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
  /** pods.pod_ip = dest_ip (pod-to-pod) */
  destWorkloadName?: string;
  destWorkloadNamespace?: string;
  /** Kubernetes Service whose spec.clusterIP equals destIp */
  destServiceName?: string;
  destServiceNamespace?: string;
  destServiceFqdn?: string;
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
  /** Count of distinct destination tuples (IP|port|proto) */
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
  destWorkloadName?: string;
  destWorkloadNamespace?: string;
  destServiceName?: string;
  destServiceNamespace?: string;
  destServiceFqdn?: string;
  /** view=edges: number of pod_network_connections rows rolled up per (pod×destination×proto) group */
  observationCount?: number;
}

/** Kubernetes event for a pod from the API. */
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
  eventId?: string;
  observedAt?: string;
  ingestedAt?: string;
  resolutionState?: string;
  sourceKind?: string;
  sourceSensorId?: string;
  sourceRule?: string;
  podUid: string;
  podName?: string;
  namespace: string;
  nodeName?: string;
  runtime?: string;
  eventType?: string;
  signal?: string;
  mitreTechnique?: string;
  severity?: string;
  confidence?: number;
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
/** Runtime attack step for a pod (from PCE enrichment). */
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

/** Aggregated attack steps by technique (from GET /risk/pods/:uid/attack-steps). */
export interface AttackStepSummary {
  stepId: string;
  category: string;
  count: number;
  avgConfidence: number;
}

// Attack Paths (Phase 3 — graph visualization)
/** Graph node representing a K8s identity or resource in an attack path. */
export interface AttackPathNode {
  id: string;
  type: string; // Pod, ServiceAccount, RoleBinding, ClusterRoleBinding, Role, ClusterRole
  properties: Record<string, unknown>;
}

/** Graph edge representing a K8s relationship (e.g., USES_SERVICE_ACCOUNT). */
export interface AttackPathEdge {
  type: string; // USES_SERVICE_ACCOUNT, REFERENCED_BY, GRANTS_ROLE
  source: string;
  target: string;
  properties?: Record<string, unknown>;
}

/** Full attack path with graph data and scoring. */
export interface AttackPath {
  /** Stable id within one bundle (p0, p1, …); matches AttackChain.paths[] */
  path_id?: string;
  nodes: AttackPathNode[];
  edges: AttackPathEdge[];
  total_risk: number;
  difficulty: number;
  impact: number;
  length: number;
  description: string;
  explainability?: {
    class: string;
    strength: number;
    feasibility: {
      network_decision: 'allow' | 'soft_allow' | 'deny' | 'n/a';
      reason: string;
      confidence: number;
    };
    scoring: {
      weakest_link: number;
      length_penalty: number;
      uncertain_penalty: number;
      final_strength: number;
    };
    risk_injection: {
      E: number;
      R: number;
      I: number;
      scale: number;
      e_points: number;
      r_points: number;
      i_points: number;
      estimated_score_delta: number;
    };
    evidence: {
      capabilities: string[];
      attack_steps: string[];
    };
  };
}

/** Summary of attack paths (counts, target breakdown and top risky pods). */
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
    /** Unified V3 score 0–100 when risk_scores row exists */
    unified_score?: number;
  }>;
}

/** Attack objective (what the adversary wants to achieve). */
export interface AttackObjective {
  objective: string;
  final_target?: string;
  priority: 'high' | 'medium' | 'low';
  chain_count?: number;
  path_count: number;
  max_strength: number;
  involved_pods: number;
  pod_uids?: string[];
  confidence: 'high' | 'medium' | 'low';
}

// Phase 3 types (Spec §2)

export interface ChainValidity {
  is_valid: boolean;
  severity?: string; // "" | "SOFT" | "HARD"
  reason?: string;
}

/** MITRE ATT&CK reference (Containers / Enterprise) from technique overlay */
export interface MitreTechniqueRef {
  id: string;
  name: string;
  tactic?: string;
  url?: string;
}

export interface CapabilityValidationResult {
  confidence: number;
  gaps?: string[];
  soft_mode: boolean;
}

/** Minimal MITRE gap / detection coverage status (Phase 3). */
export interface MitreCoverageItem {
  mitre_id: string;
  status: 'observed' | 'inferred' | 'not_covered' | 'unknown';
  /** When set: chain.realism × avg step grounding (inference quality). Omitted for unknown registry. */
  confidence?: number;
  /** HIGH = urgent gap (not_covered + high confidence); MEDIUM = inferred; LOW otherwise */
  priority?: 'HIGH' | 'MEDIUM' | 'LOW';
}

export interface AttackStep {
  technique_id: string;
  name: string;
  input_caps: string[];
  output_caps: string[];
  realism: number;
  cost: number;
  evidence_refs?: string[];
  mitre_techniques?: MitreTechniqueRef[];
  /** True when runtime_events on chain pods matched a step MITRE id (7d window, server-side). */
  runtime_observed?: boolean;
  /** pod | node | control_plane from technique overlay */
  context_scopes?: string[];
  /** Events matching this step's MITRE ÷ chain runtime event pool (7d); exposes false-grounding vs chain precision */
  runtime_grounding_score?: number;
}

/** Assumption used in attack story generation. */
export interface Assumption {
  id: string;
  description: string;
  source: string; // inferred | derived | heuristic
  confidence: number;
}

/** Evidence item backing an attack step or assumption. */
export interface Evidence {
  type: string; // FACT | CONFIG | RELATION
  source_ref: string;
  detail: string;
}

/** Narrative explaining the attack chain (headline, steps, evidence). */
export interface AttackStory {
  headline: string;
  narrative: string;
  impact_text: string;
  exploit_text: string;
  steps: string[];
  evidence_summary?: string[];
  assumption_summary?: string[];
}

/** Full attack chain with scoring, steps and provenance. */
export interface AttackChain {
  chain_id: string;
  id?: string;
  source_id?: string;
  target_id?: string;
  paths: string[];
  objective: string;
  /** LOW | MEDIUM | HIGH | CRITICAL — graph interpretation band */
  impact?: string;
  involved_resources?: string[];
  chain_strength: number;
  confidence: 'high' | 'medium' | 'low';
  confidence_raw: number;
  final_target: string;
  explanation: string;
  type: string;

  // Phase 2 & 3 scoring fields
  realism: number;
  exploit_cost: number;
  blast_radius: number;
  validity: ChainValidity;
  story: AttackStory;
  primary: boolean;

  // Phase 3 technique-bound steps & provenance
  steps?: AttackStep[];
  assumptions?: Assumption[];
  evidence?: Evidence[];

  impact_scope?: string;
  variant_spread?: number;
  variant_nodes?: string[];

  /** Path MITRE ids vs runtime_events correlation for representative scenario */
  mitre_summary?: AttackChainMitreSummary;
  /** Per-MITRE observed | inferred | not_covered (minimal gap engine) */
  mitre_coverage?: MitreCoverageItem[];
  capability_validation?: CapabilityValidationResult;
}

/** Server-enriched MITRE alignment for attack chains */
export interface AttackChainMitreSummary {
  path_mitre_distinct: number;
  runtime_mitre_distinct: number;
  /** May be null when backend omits empty correlation set */
  matched_mitre_ids?: string[] | null;
  alignment_ratio: number;
  observing_pod_count: number;
  correlation_precision?: number;
  runtime_events_considered?: number;
  runtime_events_matched?: number;
}

/** Graph data for the attack paths visualization component. */
export interface AttackPathGraphData {
  nodes: Array<{
    id: string;
    label: string;
    type: string;
    risk?: string;
    isStart?: boolean;
    isEnd?: boolean;
    stepIndex?: number;
    pathIds?: string[];
    [key: string]: unknown;
  }>;
  links: Array<{
    source: string;
    target: string;
    type?: string;
    value: number;
    stepIndex?: number;
    pathIds?: string[];
  }>;
}

/** Response from GET /graph/attack-paths/bundle (after normalizing `graph` for the D3 component). */
/** Bundle of attack path data returned by the API for page rendering. */
export interface AttackPathsPageBundle {
  graph: AttackPathGraphData;
  summary: AttackPathSummary | null;
  chains: AttackChain[];
  objectives: AttackObjective[];
  /** Full primitive paths (same build as graph); enables per-path UI and chain→graph filter */
  paths?: AttackPath[];
}

// Phase 2.3: Promotion Rules
/** Promotion rule that elevates capability state on repeated signals. */
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
/** Runtime security signal (eBPF/Falco event) for a pod. */
export interface RuntimeSignal {
  id: number;
  podUid: string;
  signalType: string;
  category: string;
  confidence: number;
  evidence: Record<string, unknown> | string;
  evidenceRefs?: Record<string, unknown> | string;
  count?: number;
  firstSeenAt?: string;
  lastSeenAt?: string;
  createdAt: string;
}

/** Suppression statistics for runtime signals (since window, event counts). */
export interface RuntimeSignalSuppressionStats {
  sinceMinutes: number;
  emittedEvents: number;
  uniqueKeys: number;
  maxRatio: number;
  perKey: Record<string, number>;
}

// Phase 2.3: Capability State History
/** Historical capability state changes for a pod. */
export interface CapabilityStateHistory {
  capabilityId: string;
  state: 'detected' | 'confirmed' | 'exploited' | 'chained';
  confidence: number;
  timestamp: string;
}

// Operations / monitoring (from API – real data)
/** Log entry from the error log API. */
export interface ErrorLog {
  id: string;
  time: string;
  level: string;
  message: string;
  source?: string;
}

/** Certificate metadata (expiry, issuer). */
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

/** Agent metadata (node, heartbeat). */
export interface Agent {
  id: string;
  node: string;
  status: string;
  lastHeartbeat: string;
}

/** Pipeline queue metrics snapshot. */
export interface QueueMetric {
  timestamp: string;
  normalizer: number;
  correlator: number;
  risk: number;
}

/** Inventory sync status (last/next scan, resource counts). */
export interface SyncStatus {
  lastScan: string;
  nextScan: string;
  drift: boolean;
  resources: { pods: number; sas: number; roles?: number; bindings?: number };
}

/** Worker status from the pipeline API. */
export interface WorkerStatus {
  name: string;
  kind?: 'pipeline-stage' | 'worker';
  source?: 'db-derived' | 'prometheus' | string;
  live?: boolean;
  description?: string;
  queueDepth: number;
  activeWorkers: number;
  processed: number;
  failed: number;
  status:
    | 'running'
    | 'idle'
    | 'not-configured'
    | 'stopped'
    | 'degraded'
    | 'blocked'
    | 'catalog-unavailable';
}

export interface CatalogHealth {
  status: 'healthy' | 'degraded' | 'stale' | 'unavailable' | string;
  mirrorVersion?: string;
  mirrorUpdatedAt?: string;
  lastCveUpdatedAt?: string;
  lastPackageVulnerabilityUpdatedAt?: string;
  lastMalwareUpdatedAt?: string;
  lastMalwareFeedSyncAt?: string;
  lastMalwareFeedSyncStatus?: string;
  activeCatalogGenerationId?: number;
  activeCatalogGenerationStatus?: string;
  activeCatalogSourceDigest?: string;
  activeCatalogActivatedAt?: string;
  activeMalwareGenerationId?: number;
  activeMalwareGenerationStatus?: string;
  activeMalwareSourceDigest?: string;
  activeMalwareActivatedAt?: string;
  cvesCount: number;
  packageVulnerabilitiesCount: number;
  osvPackagesCount: number;
  malwarePackagesCount: number;
  activeSboms: number;
  staleSboms: number;
  activeSbomsMatchedMirror: number;
  activeSbomsMissingMirrorMatch: number;
  activeSbomsMatchedGeneration: number;
  activeSbomsMissingGenerationMatch: number;
  currentMirrorSucceededRuns: number;
  currentMirrorFailedRuns: number;
  currentMirrorRunningRuns: number;
  currentGenerationSucceededRuns: number;
  currentGenerationFailedRuns: number;
  currentGenerationRunningRuns: number;
  activePodCveMatches: number;
  stalePodCveMatches: number;
}

export interface RuntimeHealth {
  status: 'healthy' | 'degraded' | 'unavailable' | string;
  source: 'db-derived' | string;
  runtimeEventsCount: number;
  runtimeSignalsCount: number;
  runtimeMetricsCount: number;
  falcoEventsCount: number;
  falcoStatus: 'active' | 'no-events' | 'unavailable' | string;
  lastRuntimeEventAt?: string;
  lastRuntimeSignalAt?: string;
  lastRuntimeMetricAt?: string;
  lastFalcoEventAt?: string;
  runtimeFreshnessMinutes: number;
  message?: string;
}

export interface DashboardDataIntegrity {
  timestamp: string;
  crossChecks: {
    activeAgentsCount: number;
    dashboardAgentsCount: number;
    clustersCount: number;
    podsCount: number;
    insightsCount: number;
    criticalInsights: number;
    cvesCount: number;
    packageVulnerabilitiesCount: number;
    osvPackagesCount: number;
    malwarePackagesCount: number;
    sbomsCount: number;
    podsMissingSbom: number;
  };
  catalogHealth: CatalogHealth;
  runtimeHealth?: RuntimeHealth;
  alerts: string[];
  endpoints: Array<{
    path: string;
    source: 'real' | 'stub' | 'placeholder' | string;
    agent: string;
    note?: string;
  }>;
}

/** User profile from the auth API. */
export interface User {
  id: string;
  name?: string;
  username: string;
  email?: string;
  role?: string;
  /** Server-derived effective permissions (JWT + /me); used for UX gating only. */
  permissions?: string[];
  /** RBAC v2 scope JSON (authoritative store); prefer operationalScope from API when present. */
  scopeJson?: string;
  /** Server-normalized operational domain from login / GET /auth/me. */
  operationalScope?: {
    clusters: string[];
    namespaces: string[];
    teams: string[];
    environments: string[];
    business_services?: string[];
    crown_jewels?: string[];
    regulatory_domains?: string[];
    restricted: boolean;
  };
  active?: boolean;
  mustChangePassword?: boolean;
  bootstrapCredential?: boolean;
  passwordChangedAt?: string;
  status?: 'active' | 'disabled' | string;
  lastLogin?: string;
}

/** JWT session record from the auth API. */
export interface FortunaUserSession {
  id: string;
  userId: string;
  issuedAt: string;
  expiresAt: string;
  revokedAt?: string;
  lastActivityAt?: string;
  sourceIp?: string;
  authMethod?: string;
  deviceFingerprint?: string;
}

/** Append-only security activity log entry (GET /governance/security-activity). */
export interface SecurityActivityItem {
  id?: number;
  eventId?: string;
  actorUserId?: number;
  actorUsername?: string;
  actorRole?: string;
  permissionsJson?: string;
  action?: string;
  resource?: string;
  resourceType?: string;
  resourceId?: string;
  result?: string;
  severity?: string;
  targetUserId?: number | null;
  requestId?: string;
  sessionId?: string;
  correlationId?: string;
  sourceIp?: string;
  userAgent?: string;
  authMethod?: string;
  beforeStateJson?: string;
  afterStateJson?: string;
  detailsJson?: string;
  createdAt?: string;
}

/** User notification (title, message, severity). */
export interface Notification {
  id: string;
  title: string;
  message: string;
  severity?: string;
  type?: string;
  source?: string;
  category?: string;
  route?: string;
  resourceUid?: string;
  resourceName?: string;
  read?: boolean;
  readAt?: string;
  timestamp?: string;
}

/** Security audit log entry (action, resource, actor). */
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

/** Security report from the reports API. */
export interface Report {
  id: string;
  name: string;
  generatedAt: string;
  resource?: string;
  action?: string;
  count?: number;
  title?: string;
  status?: string;
}

/** Security rule from the rules API (CEL expression, severity). */
export interface SecurityRule {
  id: string;
  uid?: string;
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

/** Policy template from the policy API. */
export interface PolicyTemplateRow {
  id?: number;
  templateId: string;
  version: string;
  name: string;
  description?: string;
  category: string;
  defaultSeverity: string;
  celExpression?: string;
  defaultAction?: string;
  isSystem?: boolean;
  supportsRemediation?: boolean;
}

/** Active policy instance from the policy API. */
export interface PolicyInstanceRow {
  id?: number;
  instanceName: string;
  templateId: string;
  templateVersion: string;
  description?: string;
  enabled: boolean;
  action?: string;
  severity?: string;
  clusters?: string[];
  namespaces?: string[];
  resourceTypes?: string[];
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

/** K8s resource from the inventory API (UID, kind). */
export interface K8sResource {
  id: string; // UID from API (used for Identity/Role links)
  name: string;
  namespace: string;
  kind: string;
  clusterId?: string;
  age?: string;
  status?: string;
}

export interface K8sRbacResourceDetail {
  kind: 'Role' | 'RoleBinding' | 'ClusterRole' | 'ClusterRoleBinding' | string;
  resource?: Record<string, unknown>;
  rules?: K8sEffectiveRule[];
  roleRef?: Record<string, unknown>;
  subjects?: Array<Record<string, unknown>>;
  resolvedRole?: Record<string, unknown>;
  referencedBy?: Array<Record<string, unknown>>;
  roleBindings?: Array<Record<string, unknown>>;
  clusterRoleBindings?: Array<Record<string, unknown>>;
}

/** GET /inventory/serviceaccounts/:uid/permissions — Kubernetes RBAC effective rules + bindings */
export interface K8sEffectiveRule {
  verbs?: string[];
  apiGroups?: string[];
  resources?: string[];
  resourceNames?: string[];
  nonResourceURLs?: string[];
}

export interface K8sRoleBindingPermission {
  roleBinding?: Record<string, unknown>;
  role?: Record<string, unknown>;
}

export interface K8sClusterRoleBindingPermission {
  clusterRoleBinding?: Record<string, unknown>;
  clusterRole?: Record<string, unknown>;
}

/** RBAC permissions for a Kubernetes ServiceAccount. */
export interface ServiceAccountK8sPermissions {
  serviceAccountId?: number;
  roleBindings?: K8sRoleBindingPermission[];
  clusterRoleBindings?: K8sClusterRoleBindingPermission[];
  effectiveRules?: K8sEffectiveRule[];
}

/** Certificate rotation event (success/failure). */
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

/** Flattened pipeline ref (one row per id); mirrors Core `evidence_chain_refs`. */
export interface InsightEvidenceChainRef {
  layer: string;
  ref: string;
}

/** Resolved fact rows when GET /risk/insights/:id?enrich=1 */
export interface InsightEnrichedFactRef {
  factId: string;
  factType?: string;
  domain?: string;
  observedAt?: string;
}

/** Enrichment refs for an insight (facts, domains). */
export interface InsightEnrichedRefs {
  facts?: InsightEnrichedFactRef[];
}

/** Score breakdown item for unified risk scoring. */
export interface InsightScoreBreakdownItem {
  factor_id: string;
  category: string;
  scope: string;
  source: string;
  contribution: number;
  evidence_refs?: string[];
}

/** An insight (risk finding) from the API with full details. */
export interface Insight {
  id: string; // PK from API (used for GET /risk/insights/:id and route /risks/:id)
  cveId?: string; // CVE or internal dedup key (e.g. CVE-2024-123, supply-malware:pkg@ver)
  /** Populated from API for supply_chain_malware rows */
  affectedComponent?: string;
  affectedVersion?: string;
  title: string;
  description?: string;
  severity: string;
  /** P0 unified model: detector/engine hint, not final risk output */
  severityHint?: string;
  /** P0 unified model: authoritative final risk level derived from score */
  finalLevel?: 'low' | 'medium' | 'high' | 'critical';
  score?: number;
  category?: string;
  /** Insight type from API (e.g. vulnerability, rbac) – for Risk Center Type column */
  insightType?: string;
  status?: string;
  timestamp?: string;
  clusterId?: string;
  clusterName?: string;
  affectedResources?: Array<{
    id: string;
    name?: string;
    kind?: string;
    namespace?: string;
  }>;
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
  /** P0 unified model alias of totalScore for clearer semantics */
  finalScore?: number;
  breakdown?: InsightScoreBreakdownItem[];
  /** Phase 3.2: persisted sub-scores from risk_scores / V3 factors (when withScores=1) */
  exploitabilityScore?: number;
  businessImpactScore?: number;
  timeDecay?: number;
  evidence_refs?: InsightEvidenceRefsPayload;
  explanation_chain?: InsightExplanationChainStep[];
  /** Flattened ordered refs from Core `evidence_chain_refs` (GET /risk/insights/:id). */
  evidence_chain_refs?: InsightEvidenceChainRef[];
  enriched_refs?: InsightEnrichedRefs;
  /** Row from GET /risk/insights?view=group (id is sample_insight_id for drill-down). */
  isGroupRow?: boolean;
  groupMemberCount?: number;
}

/** One aggregated group from GET /risk/insights?view=group */
/** Aggregated insight group (e.g., by CVE or type). */
export interface RiskInsightGroupSummary {
  group_key: string;
  insight_type: string;
  title: string;
  member_count: number;
  max_score?: number;
  final_level?: string;
  max_severity?: string;
  sample_insight_id: number;
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

/** GET /risk/pods/:uid/report — pod RBAC and related finding context. */
export interface PodRiskReport {
  podUid: string;
  podName: string;
  namespace: string;
  clusterId: string;
  serviceAccount?: string;
  serviceAccountUid?: string;
  bindings?: unknown[];
  roles?: unknown[];
  insights: Insight[];
  summary?: PodRiskReportSummary;
}

// ---------------------------------------------------------------------------
// Phase 3: Unified Risk Pipeline — TypeScript types (Phase 3.4)
// ---------------------------------------------------------------------------

/**
 * UnifiedRiskScore is the V3 unified risk score returned by
 * GET /api/v1/risk/scores/:uid  (scorer_version = "v3").
 *
 * Dimensions sum to ≤90; remaining 10 pts come from blast_radius.
 * Toxic combo boosts (+5–+15) are applied after the dimension sum.
 */
export interface UnifiedRiskScore {
  resourceUid: string;
  resourceName: string;
  resourceType: string;
  namespace: string;
  clusterId: string;
  totalScore: number;
  /** ADR unified level from score (preferred for display). */
  finalLevel?: 'low' | 'medium' | 'high' | 'critical';
  scorerVersion: string; // "v3"
  dimensions: {
    vulnerability: number; // 0–15
    capabilityExposure: number; // 0–15
    attackPath: number; // 0–15
    rbacPolicy: number; // 0–15
    runtimeThreat: number; // 0–15
    exposure: number; // 0–15
    blastRadius: number; // 0–10
  };
  toxicCombos: string[];
  timeDecay: number;
  calculatedAt: string; // ISO-8601
}

/**
 * PipelineHealth aggregates the status of each layer of the Unified Risk
 * Pipeline and is returned by GET /api/v1/monitoring/pipeline-health.
 */
export interface PipelineHealth {
  layer1: {
    lastPceEval: string | null;
    lastRiskEngineEval: string | null;
    insightCount: number;
    freshnessMinutes?: number;
    status?: 'healthy' | 'degraded' | 'stale' | 'unknown';
  };
  layer2: {
    lastStateChange: string | null;
    activePromotionRules: number;
    exploitedCapCount: number;
    freshnessMinutes?: number;
    status?: 'healthy' | 'degraded' | 'stale' | 'unknown';
  };
  layer3: {
    lastPathComputation: string | null;
    totalPaths: number;
    criticalPaths: number;
    freshnessMinutes?: number;
    status?: 'healthy' | 'degraded' | 'stale' | 'unknown';
  };
  layer4: {
    lastScoreCalc: string | null;
    resourcesScored: number;
    avgScore: number;
    v3Resources: number;
    freshnessMinutes?: number;
    status?: 'healthy' | 'degraded' | 'stale' | 'unknown';
  };
}

/**
 * EnrichedAttackPath extends AttackPath with additional fields populated
 * during PCE enrichment (Phase 1.2).
 */
export interface EnrichedAttackPath extends AttackPath {
  enrichedFromPce: boolean;
}
