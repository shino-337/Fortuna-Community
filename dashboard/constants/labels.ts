/**
 * Shared display labels for metrics across Dashboard, Risk Center, Clusters, etc.
 * Use these so numbers shown on different screens refer to the same data.
 */

/** Dashboard stats – same definitions as backend GET /api/v1/dashboard/stats */
export const STAT_LABELS = {
  /** Active clusters (last_sync within 7 days) – matches Clusters page count */
  CLUSTERS: 'Clusters',
  /** Vulnerability insights (insight_type=vulnerability) – matches Risk Center Risks tab total */
  SECURITY_RISKS: 'Security Risks',
  /** Pod count from pods table (deleted_at IS NULL) – total pods tracked */
  PODS: 'Pods',
  /** Active agents (status=ready, last_seen within 10 min) */
  AGENTS: 'Agents',
} as const;

/** Risk Center – same as STAT_LABELS.SECURITY_RISKS (vulnerability findings) */
export const RISK_CENTER_DESCRIPTION = 'Vulnerability findings (same count as Dashboard Security Risks). Triage, investigate, and remediate.';
