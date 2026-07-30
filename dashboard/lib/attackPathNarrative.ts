import type { AttackChain, AttackPath } from '../types';

/* ── capability → human mapping (A2 from spec) ────────────── */

const CAP_HUMAN: Record<string, string> = {
  ESC_HOSTPATH_NODE: 'Escape container via hostPath mount',
  ESC_HOSTPATH: 'Escape container via hostPath mount',
  ESC_HOSTPID: 'Escape container via hostPID',
  ESC_HOSTPID_POD: 'Escape container via hostPID',
  ESC_HOSTIPC: 'Escape container via hostIPC',
  ESC_HOSTNETWORK: 'Escape container via host network',
  HOST_ACCESS: 'Access the underlying node filesystem',
  NODE_ACCESS: 'Gain direct access to the cluster node',
  PRIVILEGED_CONTAINER: 'Run a privileged container',
  SERVICE_ACCOUNT_ACCESS: 'Access service account credentials',
  SA_TOKEN: 'Access service account token',
  GRANTS_ROLE: 'Gain permissions via RBAC binding',
  GRANTS_CLUSTER_ROLE: 'Gain cluster-wide permissions via ClusterRoleBinding',
  NETWORK_REACH_SOFT: 'Potential network access (unverified)',
  NETWORK_REACH: 'Network access to target workload',
  SECRET_ACCESS: 'Access Kubernetes secrets',
  CLUSTER_ADMIN: 'Cluster-admin level privilege',
};

/** Returns a human-readable description for an attack capability. */
export function capabilityHumanSnippet(cap: string): string {
  const key = cap.toUpperCase().trim();
  if (CAP_HUMAN[key]) return CAP_HUMAN[key];
  if (key.includes('HOSTPATH') || key.includes('HOST_PATH') || key.includes('HOST_ACCESS'))
    return 'host filesystem or node access exposure';
  if (key.includes('PRIVILEGED')) return 'privileged container configuration';
  if (key.includes('CLUSTERROLE') || key.startsWith('ROLE:'))
    return 'RBAC role or cluster-wide permissions';
  if (key.includes('SERVICE_ACCOUNT') || key === 'SA_TOKEN')
    return 'service account credentials';
  if (key.includes('NETWORK')) return 'network reachability';
  if (key.includes('SECRET')) return 'access to secrets';
  return cap.replace(/_/g, ' ').toLowerCase();
}

/* ── chain type labels ─────────────────────────────────────── */

/** Returns a human-readable label for an attack path chain type. */
export function chainTypeHumanLabel(type: string): string {
  const t = String(type || '').toUpperCase().trim();
  const m: Record<string, string> = {
    ESCAPE_TO_PRIV_ESC: 'Container escape → cluster privilege escalation',
    LATERAL_TO_PRIV_ESC: 'Lateral movement → privilege escalation',
    PRIV_ESC_LADDER: 'RBAC privilege escalation ladder',
    ACCESS_TO_DATA_EXFIL: 'Access foothold → sensitive data exposure',
    NETWORK_BRIDGE: 'Network reach bridging into another workload',
    SA_TOKEN_REUSE: 'Service account token reuse attack',
    SAME_SERVICE_ACCOUNT: 'Shared service account linking two paths',
    NODE_DOMINANCE: 'Node-level control enabling further attack moves',
  };
  if (m[t]) return m[t];
  return t.replace(/_/g, ' ').toLowerCase().replace(/^\w/, (c) => c.toUpperCase());
}

/* ── scenario dedup / grouping (Phase 1: Fix Trust) ─────── */

/** GroupedScenario represents a cluster of related attack chains. */
export interface GroupedScenario {
  key: string;
  headline: string;
  objective: string;
  finalTarget: string;
  type: string;
  confidence: 'high' | 'medium' | 'low';
  maxStrength: number;
  variants: AttackChain[];
  representativeChain: AttackChain;
  narrative: string;
  impactStatements: string[];
  detectionConfidence: string;
  exploitRealism: string;
}

/** Groups attack chains by (objective, final_target, type) and returns sorted scenarios. */
function scenarioGroupKey(c: AttackChain): string {
  return `${c.objective}::${c.final_target}::${c.type}`;
}

/** Groups attack chains into scenarios with narrative text and impact statements. */
export function groupChains(chains: AttackChain[], paths: AttackPath[]): GroupedScenario[] {
  const groups = new Map<string, AttackChain[]>();
  for (const c of Array.isArray(chains) ? chains : []) {
    const key = scenarioGroupKey(c);
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(c);
  }

  const pathMap = new Map<string, AttackPath>();
  for (const p of Array.isArray(paths) ? paths : []) {
    if (p.path_id) pathMap.set(p.path_id, p);
  }

  const result: GroupedScenario[] = [];

  for (const [key, variants] of groups) {
    const best = variants.reduce((a, b) => (b.chain_strength > a.chain_strength ? b : a));

    const allPathIds = new Set<string>();
    for (const v of variants) for (const pid of Array.isArray(v.paths) ? v.paths : []) allPathIds.add(pid);
    const relatedPaths = [...allPathIds].map((id) => pathMap.get(id)).filter(Boolean) as AttackPath[];

    const narrative = buildDynamicNarrative(best, relatedPaths);
    const impacts = buildImpactStatements(best);
    const { detection, realism } = splitConfidence(best, relatedPaths);

    result.push({
      key,
      headline: buildScenarioHeadline(best, relatedPaths),
      objective: best.objective,
      finalTarget: best.final_target,
      type: best.type,
      confidence: best.confidence,
      maxStrength: Math.max(...variants.map((v) => v.chain_strength)),
      variants,
      representativeChain: best,
      narrative,
      impactStatements: impacts,
      detectionConfidence: detection,
      exploitRealism: realism,
    });
  }

  return result.sort((a, b) => b.maxStrength - a.maxStrength);
}

function clamp01(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(1, value));
}

function confidenceFromStrength(strength: number): 'high' | 'medium' | 'low' {
  if (strength >= 0.5) return 'high';
  if (strength >= 0.25) return 'medium';
  return 'low';
}

function nodeLabel(node: AttackPath['nodes'][number] | undefined, fallback: string): string {
  return String(node?.properties?.name || node?.id || fallback);
}

function classToObjective(pathClass: string): string {
  const cls = pathClass.toUpperCase();
  if (cls.includes('NODE') || cls.includes('ESCAPE')) return 'NODE_COMPROMISE';
  if (cls.includes('DATA') || cls.includes('EXFIL')) return 'DATA_EXFILTRATION';
  if (cls.includes('LATERAL')) return 'LATERAL_MOVEMENT';
  return 'CLUSTER_PRIVILEGE_ESCALATION';
}

function classToChainType(pathClass: string): string {
  const cls = pathClass.toUpperCase();
  if (cls.includes('ESCAPE') || cls.includes('NODE')) return 'ESCAPE_TO_PRIV_ESC';
  if (cls.includes('LATERAL')) return 'LATERAL_TO_PRIV_ESC';
  if (cls.includes('DATA') || cls.includes('EXFIL')) return 'ACCESS_TO_DATA_EXFIL';
  if (cls.includes('SERVICE_ACCOUNT') || cls.includes('SA_TOKEN')) return 'SA_TOKEN_REUSE';
  return 'PRIV_ESC_LADDER';
}

/** Builds scenario cards from primitive attack paths when the chain API has not produced grouped chains yet. */
export function groupPrimitivePaths(paths: AttackPath[]): GroupedScenario[] {
  const safePaths = Array.isArray(paths) ? paths : [];
  if (safePaths.length === 0) return [];

  const syntheticChains: AttackChain[] = safePaths.map((path, index) => {
    const nodes = Array.isArray(path.nodes) ? path.nodes : [];
    const edges = Array.isArray(path.edges) ? path.edges : [];
    const source = nodes[0];
    const target = nodes[nodes.length - 1];
    const sourceName = nodeLabel(source, 'workload');
    const targetName = nodeLabel(target, 'sensitive target');
    const pathClass = path.explainability?.class || '';
    const strength = clamp01(path.explainability?.strength ?? path.explainability?.scoring?.final_strength ?? path.total_risk / 10);
    const confidence = confidenceFromStrength(strength);
    const pathId = path.path_id || `path-${index + 1}`;
    const capabilities = Array.isArray(path.explainability?.evidence?.capabilities)
      ? path.explainability!.evidence.capabilities
      : edges.map((edge) => edge.type).filter(Boolean);
    const evidenceSummary = capabilities.slice(0, 4).map(capabilityHumanSnippet);
    const softNetwork = path.explainability?.feasibility?.network_decision === 'soft_allow';

    return {
      chain_id: `primitive-${pathId}`,
      source_id: source?.id,
      target_id: target?.id,
      paths: [pathId],
      objective: classToObjective(pathClass),
      impact: riskLabelFromPath(path),
      involved_resources: nodes.map((node) => nodeLabel(node, node.id)).filter(Boolean),
      chain_strength: strength,
      confidence,
      confidence_raw: strength,
      final_target: targetName,
      explanation: path.description || buildWhyItMattersHuman(path),
      type: classToChainType(pathClass),
      realism: strength,
      exploit_cost: clamp01(path.difficulty ?? 0),
      blast_radius: clamp01(path.impact ?? 0),
      validity: {
        is_valid: true,
        severity: softNetwork ? 'SOFT' : '',
        reason: softNetwork ? 'Network reachability is inferred.' : 'Derived from attack path graph evidence.',
      },
      story: {
        headline: `${sourceName} → ${targetName}`,
        narrative: path.description || buildWhyItMattersHuman(path),
        impact_text: buildWhyItMattersHuman(path),
        exploit_text: strengthSemantic(strength),
        steps: edges.slice(0, 4).map((edge) => capabilityHumanSnippet(edge.type)),
        evidence_summary: evidenceSummary,
        assumption_summary: softNetwork ? ['Network reachability needs runtime validation.'] : [],
      },
      primary: index === 0,
    };
  });

  return groupChains(syntheticChains, safePaths);
}

function riskLabelFromPath(path: AttackPath): string {
  const score = path.total_risk ?? 0;
  if (score >= 9) return 'CRITICAL';
  if (score >= 7) return 'HIGH';
  if (score >= 4) return 'MEDIUM';
  return 'LOW';
}

/* ── dynamic narrative (Phase 2) ───────────────────────────── */

/** Extracts key actors from an attack chain and related paths. */
function extractActors(chain: AttackChain, paths: AttackPath[]): {
  sourcePod: string; nodeName: string; targetRole: string; capabilities: string[];
} {
  let sourcePod = '';
  let nodeName = '';
  let targetRole = cleanTargetRole(chain.final_target || '');
  const capabilities: string[] = [];

  for (const p of Array.isArray(paths) ? paths : []) {
    for (const n of Array.isArray(p.nodes) ? p.nodes : []) {
      const name = String(n.properties?.name || '');
      const type = n.type?.toLowerCase();
      if (type === 'pod' && !sourcePod && name) sourcePod = name;
      if (type === 'node' && !nodeName) nodeName = name || n.id;
      if ((type === 'clusterrole' || type === 'cluster_role' || type === 'role') && name) targetRole = targetRole || name;
    }
    for (const e of Array.isArray(p.edges) ? p.edges : []) {
      const capKey = e.type?.toUpperCase() || '';
      if (capKey && !capabilities.includes(capKey)) capabilities.push(capKey);
    }
    if (p.explainability?.evidence?.capabilities) {
      for (const c of Array.isArray(p.explainability.evidence.capabilities) ? p.explainability.evidence.capabilities : []) {
        if (!capabilities.includes(c)) capabilities.push(c);
      }
    }
  }

  return { sourcePod, nodeName, targetRole, capabilities };
}

function cleanTargetRole(value: string): string {
  const raw = String(value || '').trim();
  if (!raw) return '';
  const parts = raw.split(':');
  const last = parts[parts.length - 1] || raw;
  return last.replace(/^role\//, '').replace(/^clusterrole\//, '');
}

function pathsHaveSoftReach(paths: AttackPath[]): boolean {
  return (Array.isArray(paths) ? paths : []).some((p) => {
    if (p.explainability?.feasibility?.network_decision === 'soft_allow') return true;
    return (Array.isArray(p.edges) ? p.edges : []).some((e) => String(e.type || '').toUpperCase() === 'NETWORK_REACH_SOFT');
  });
}

function firstPathHop(paths: AttackPath[]): { source: string; target: string } {
  for (const p of Array.isArray(paths) ? paths : []) {
    const nodes = Array.isArray(p.nodes) ? p.nodes : [];
    const source = nodes.find((n) => String(n.type || '').toLowerCase() === 'pod');
    const target = nodes[nodes.length - 1];
    const sourceName = String(source?.properties?.name || source?.id || '').trim();
    const targetName = String(target?.properties?.name || target?.id || '').trim();
    if (sourceName || targetName) return { source: sourceName, target: cleanTargetRole(targetName) || targetName };
  }
  return { source: '', target: '' };
}

function buildDynamicNarrative(chain: AttackChain, paths: AttackPath[]): string {
  const { sourcePod, nodeName, targetRole, capabilities } = extractActors(chain, paths);
  const t = chain.type?.toUpperCase() || '';
  const softReach = pathsHaveSoftReach(paths);
  const hop = firstPathHop(paths);

  if (softReach && (t.includes('SA_TOKEN') || t.includes('PRIV_ESC') || t.includes('LATERAL'))) {
    return `Potential path: if ${hop.source || sourcePod || 'the source workload'} can reach the pivot workload, existing service-account and RBAC bindings lead toward ${targetRole || hop.target || 'privileged access'}. Treat this as inferred until runtime or network evidence confirms reachability.`;
  }

  if (t.includes('ESCAPE') && sourcePod && nodeName) {
    const capDesc = capabilities.find((c) => c.includes('HOST'))
      ? 'hostPath mount'
      : capabilities.find((c) => c.includes('PRIVILEGED'))
        ? 'privileged container'
        : 'container escape vector';
    return `If an attacker compromises ${sourcePod}, they can escape the container using ${capDesc}, gain access to node ${nodeName}, and escalate privileges${targetRole ? ` via ${targetRole}` : ''}.`;
  }
  if (t.includes('LATERAL') && sourcePod) {
    return `An attacker may move laterally from ${sourcePod}${nodeName ? ` through node ${nodeName}` : ''} to reach ${targetRole || 'sensitive workloads'}, chaining service account credentials and RBAC bindings.`;
  }
  if (t.includes('SA_TOKEN') || t.includes('SERVICE_ACCOUNT')) {
    return `An attacker may reuse service account credentials from ${sourcePod || 'a compromised pod'} to execute actions targeting ${targetRole || 'other workloads'}.`;
  }
  if (t.includes('PRIV_ESC') && sourcePod) {
    return `Starting from ${sourcePod}, an attacker can chain RBAC bindings to escalate from pod-level access to ${targetRole || 'cluster-wide privileges'}.`;
  }
  if (t.includes('DATA') || t.includes('EXFIL')) {
    return `Access from ${sourcePod || 'the source workload'} can be chained toward ${targetRole || 'a sensitive role or workload'}, exposing privileged Kubernetes actions or data access. Review the referenced binding and service account before treating this as confirmed exfiltration.`;
  }

  if (chain.explanation?.trim()) {
    return chain.explanation.trim();
  }

  return `Attack scenario through ${sourcePod || 'compromised workload'}${nodeName ? ` via node ${nodeName}` : ''} targeting ${targetRole || 'sensitive resources'}.`;
}

function buildScenarioHeadline(chain: AttackChain, paths: AttackPath[]): string {
  const { sourcePod, nodeName, targetRole } = extractActors(chain, paths);
  const t = chain.type?.toUpperCase() || '';
  const softReach = pathsHaveSoftReach(paths);

  if (softReach && (t.includes('SA_TOKEN') || t.includes('PRIV_ESC') || t.includes('LATERAL'))) {
    return `Potential reach${sourcePod ? ` from ${sourcePod}` : ''} → ${targetRole || 'privileged target'}`;
  }

  if (t.includes('ESCAPE') && nodeName) {
    return `Container escape → node ${nodeName} → ${targetRole || 'cluster privilege escalation'}`;
  }
  if (t.includes('LATERAL')) {
    return `Lateral movement${sourcePod ? ` from ${sourcePod}` : ''} → ${targetRole || 'privilege escalation'}`;
  }
  if (t.includes('SA_TOKEN') || t.includes('SERVICE_ACCOUNT')) {
    return `Service account token reuse → ${targetRole || 'follow-on attack'}`;
  }
  if (t.includes('PRIV_ESC')) {
    return `RBAC escalation${sourcePod ? ` from ${sourcePod}` : ''} → ${targetRole || 'cluster-admin'}`;
  }
  if (t.includes('DATA') || t.includes('EXFIL')) {
    return `Access path${sourcePod ? ` from ${sourcePod}` : ''} → ${targetRole || 'sensitive target'}`;
  }

  return chainTypeHumanLabel(chain.type);
}

/* ── impact-driven "Why it matters" (Phase 3) ──────────────── */

function buildImpactStatements(chain: AttackChain): string[] {
  const t = chain.type?.toUpperCase() || '';
  const obj = chain.objective?.toUpperCase() || '';
  const out: string[] = [];

  if (obj.includes('CLUSTER_PRIVILEGE') || t.includes('PRIV_ESC')) {
    out.push('Attacker can gain cluster-wide admin privileges');
    out.push('Can deploy malicious workloads in any namespace');
    out.push('Can access secrets across all namespaces');
    out.push('Can disrupt or take over the entire cluster');
  } else if (obj.includes('NODE_COMPROMISE') || t.includes('ESCAPE') || t.includes('NODE')) {
    out.push('Attacker can control the underlying node');
    out.push('Can access all pods running on the compromised node');
    out.push('Can intercept network traffic between pods');
    out.push('Can persist across pod restarts');
  } else if (obj.includes('DATA_EXFIL') || t.includes('DATA')) {
    out.push('Sensitive data may be exfiltrated from the cluster');
    out.push('Secrets and credentials could be exposed');
    out.push('Compliance and data protection policies may be violated');
  } else if (t.includes('LATERAL')) {
    out.push('Attacker can spread across workloads');
    out.push('Can reach sensitive services from a less-protected entry point');
    out.push('Blast radius expands with each lateral step');
  } else {
    out.push('Attacker can escalate access beyond intended boundaries');
    out.push('Sensitive resources may be compromised');
  }

  return out;
}

/* ── confidence split (Phase 4) ────────────────────────────── */

function splitConfidence(chain: AttackChain, paths: AttackPath[]): { detection: string; realism: string } {
  const safePaths = Array.isArray(paths) ? paths : [];
  const hasRbacData = safePaths.some((p) => (Array.isArray(p.nodes) ? p.nodes : []).some((n) =>
    ['clusterrole', 'role', 'rolebinding', 'clusterrolebinding'].includes(n.type?.toLowerCase() || '')));
  const hasSoftAllow = safePaths.some((p) => p.explainability?.feasibility?.network_decision === 'soft_allow');
  const hasRuntime = safePaths.some((p) =>
    (p.explainability?.evidence?.attack_steps?.length ?? 0) > 0);

  let detection: string;
  if (chain.confidence === 'high') {
    const basis = hasRbacData ? 'RBAC data' : hasRuntime ? 'runtime signals' : 'static analysis';
    detection = `HIGH (based on ${basis})`;
  } else if (chain.confidence === 'medium') {
    detection = 'MEDIUM (partial evidence)';
  } else {
    detection = 'LOW (limited signals)';
  }

  let realism: string;
  if (hasSoftAllow) {
    realism = 'MEDIUM (network access is inferred, not confirmed)';
  } else if (chain.confidence === 'high' && chain.chain_strength >= 0.25) {
    realism = 'HIGH (observed connectivity and permissions)';
  } else if (chain.chain_strength >= 0.15) {
    realism = 'MEDIUM (feasible but requires multiple steps)';
  } else {
    realism = 'LOW (theoretical, many assumptions)';
  }

  return { detection, realism };
}

/* ── semantic strength labels (Phase 6) ────────────────────── */

/** Convert numeric strength to human-readable threshold text. */
export function strengthSemantic(strength: number): string {
  if (strength >= 0.5) return 'Strong (direct, few steps)';
  if (strength >= 0.25) return 'Moderate (requires multiple steps but feasible)';
  if (strength >= 0.12) return 'Weak (multiple assumptions required)';
  return 'Very weak (largely theoretical)';
}

/** Convert numeric impact to human-readable threshold text. */
export function impactSemantic(path: AttackPath): string {
  const impact = path.impact ?? 0;
  if (impact >= 0.7) return 'Critical (cluster-wide)';
  if (impact >= 0.4) return 'High (multiple workloads affected)';
  if (impact >= 0.15) return 'Medium (limited blast radius)';
  return 'Low (single workload)';
}

/* ── "Why it matters" for PathCard (Phase 3) ───────────────── */

/** Builds a human-readable "why it matters" explanation for an attack path. */
export function buildWhyItMattersHuman(path: AttackPath): string {
  const nodes = Array.isArray(path.nodes) ? path.nodes : [];
  const target = nodes[nodes.length - 1];
  const targetLabel = String(target?.properties?.name || target?.id || 'a sensitive target');
  const source = nodes[0];
  const sourceLabel = String(source?.properties?.name || source?.id || 'a compromised pod');
  const x = path.explainability;

  const cls = (x?.class || '').toUpperCase();

  if (cls.includes('ESCAPE') || cls.includes('NODE')) {
    return `An attacker controlling ${sourceLabel} can escape to the node and gain access to all co-located workloads, potentially controlling the entire node and pivoting to cluster-level resources through ${targetLabel}.`;
  }
  if (cls.includes('PRIV_ESC')) {
    return `By chaining RBAC bindings from ${sourceLabel}, an attacker can escalate to ${targetLabel}, gaining broad permissions to deploy, modify, or destroy workloads cluster-wide.`;
  }
  if (cls.includes('LATERAL')) {
    return `An attacker in ${sourceLabel} can move laterally to reach ${targetLabel}, expanding the blast radius and accessing services beyond the original compromise.`;
  }
  if (cls.includes('DATA') || cls.includes('EXFIL')) {
    return `This path enables data exfiltration from ${targetLabel}, risking exposure of secrets, credentials, and sensitive application data.`;
  }

  const caps = (Array.isArray(x?.evidence?.capabilities) ? x?.evidence?.capabilities : [])?.slice(0, 3).map(capabilityHumanSnippet) ?? [];
  const capPart = caps.length ? ` Key exposures: ${caps.join('; ')}.` : '';
  return `This path chains existing permissions and mounts from ${sourceLabel} toward ${targetLabel}, enabling the attacker to escalate access beyond intended boundaries.${capPart}`;
}

/* ── legacy exports (keep for backward compat) ─────────────── */

/** Returns a concise headline for an attack chain. */
export function scenarioHeadlineFromChain(c: AttackChain): string {
  const ex = (c.explanation || '').trim();
  if (ex) {
    const first = ex.split(/(?<=[.!?])\s+/)[0]?.trim() || ex;
    if (first.length <= 160) return first;
    return `${first.slice(0, 157)}…`;
  }
  return chainTypeHumanLabel(c.type);
}

/** Returns weak links in an attack path for explainability. */
export function weakLinksFromPath(path: AttackPath): string[] {
  const out: string[] = [];
  const x = path.explainability;
  const feasibility = x?.feasibility;
  if (!x || !feasibility) return out;
  if (feasibility.network_decision === 'soft_allow')
    out.push('Network segment is inferred — verify with runtime evidence.');
  if ((feasibility.confidence ?? 1) < 0.65)
    out.push('Lower confidence on feasibility — validate assumptions.');
  if ((x.scoring?.uncertain_penalty ?? 0) > 0.12)
    out.push('Scoring applies an uncertainty penalty — path may be less reliable.');
  return out;
}

/* ── variant label (for grouped view) ──────────────────────── */

/** Returns a variant label for grouped attack path display. */
export function variantLabel(chain: AttackChain, paths: AttackPath[]): string {
  const actors = extractActors(chain, paths);
  const parts: string[] = [];
  if (actors.sourcePod) parts.push(actors.sourcePod);
  if (actors.nodeName) parts.push(actors.nodeName);
  if (actors.targetRole) parts.push(actors.targetRole);
  if (parts.length > 0) return parts.join(' → ');
  return (Array.isArray(chain.paths) ? chain.paths : []).join(', ') || chain.chain_id;
}

/** Builds root cause labels for an attack chain. */
export function buildRootCauses(chain: AttackChain): string[] {
  const causes = new Set<string>();
  (Array.isArray(chain.steps) ? chain.steps : []).forEach(s => {
    const id = s.technique_id || '';
    if (id.includes('ESCAPE_HOSTPATH')) causes.add('hostPath volume mounted to container');
    if (id.includes('ESCAPE_HOSTPID') || id.includes('ESCAPE_HOSTIPC')) causes.add('host namespaces (PID/IPC) shared with container');
    if (id.includes('ESCAPE_PRIVILEGED')) causes.add('Container running in privileged mode');
    if (id.includes('KUBELET_TOKEN_HARVEST')) causes.add('Service Account token automounted on potentially compromised pods');
    if (id.includes('RBAC_PRIV_ESC') || id.includes('CLUSTER_ADMIN_ESC')) causes.add('Overly permissive ClusterRole/Role bindings granted to Service Account');
  });
  if (causes.size === 0) causes.add('Excessive privileges or misconfigured boundaries');
  return Array.from(causes);
}

/** Builds fix recommendations for an attack chain. */
export function buildFixRecommendations(chain: AttackChain): { label: string; priority: string }[] {
  const fixes: { label: string; priority: string }[] = [];
  const causes = buildRootCauses(chain);
  const refs = (Array.isArray(chain.evidence) ? chain.evidence : [])
    .map((e) => String(e.source_ref || '').trim())
    .filter(Boolean);
  const byPrefix = (prefixes: string[]) => refs
    .filter((ref) => prefixes.some((prefix) => ref.toLowerCase().startsWith(prefix)))
    .map((ref) => ref.split('/').slice(1).join('/') || ref)
    .filter(Boolean);
  const unique = (items: string[]) => Array.from(new Set(items));
  const firstBinding = unique(byPrefix(['binding/']))[0];
  const firstSA = unique(byPrefix(['serviceaccount/']))[0];
  const firstRole = unique(byPrefix(['clusterrole/', 'role/']))[0];
  const firstPod = unique(byPrefix(['pod/']))[0];
  const caps = unique(byPrefix(['capability/']).map((c) => c.toUpperCase()));

  if (causes.some(c => c.includes('hostPath')) || caps.some((c) => c.includes('HOSTPATH') || c.includes('HOST_ACCESS'))) {
    fixes.push({ label: `Remove or make hostPath read-only${firstPod ? ` on ${firstPod}` : ''}`, priority: 'HIGH' });
  }
  if (causes.some(c => c.includes('privileged mode')) || caps.some((c) => c.includes('PRIVILEGED'))) {
    fixes.push({ label: `Disable privileged mode${firstPod ? ` on ${firstPod}` : ' for affected workloads'}`, priority: 'CRITICAL' });
  }
  if (causes.some(c => c.includes('host namespaces'))) fixes.push({ label: `Remove hostPID/hostIPC${firstPod ? ` from ${firstPod}` : ' from pod specs'}`, priority: 'HIGH' });
  if (causes.some(c => c.includes('automounted')) || firstSA) {
    fixes.push({ label: `Scope ${firstSA || 'the service account'} least-privilege and disable token automount where unused`, priority: 'MEDIUM' });
  }
  if (causes.some(c => c.includes('Overly permissive')) || firstBinding || firstRole) {
    const bindingText = firstBinding ? ` binding ${firstBinding}` : '';
    const roleText = firstRole ? ` to ${firstRole}` : '';
    fixes.push({ label: `Replace or narrow${bindingText}${roleText} with least-privilege RBAC`, priority: 'CRITICAL' });
  }
  if (chain.capability_validation?.soft_mode || (Array.isArray(chain.capability_validation?.gaps) && chain.capability_validation.gaps.length > 0)) {
    fixes.push({ label: 'Validate inferred reachability before scheduling disruptive remediation', priority: 'MEDIUM' });
  }

  if (fixes.length === 0) fixes.push({ label: 'Review workload isolation and RBAC policies', priority: 'MEDIUM' });
  return fixes.slice(0, 4);
}
