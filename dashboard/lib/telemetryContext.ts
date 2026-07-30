/**
 * Layer 3 — Telemetry / ingestion posture for visibility (UX signals only).
 */
export interface TelemetryContext {
  /** At least one agent heartbeat recent enough for ops. */
  agentsHealthy: boolean | null;
  /** Pipeline or worker degradation detected. */
  pipelineDegraded: boolean | null;
  /** Graph/path data may be incomplete due to sampling or LOD. */
  graphSampled: boolean;
  /** Stale ingestion beyond SLA threshold. */
  ingestionStale: boolean | null;
  /** Policy-limited graph traversal (summary-only graph access). */
  graphPolicyLimited: boolean;
}

export const DEFAULT_TELEMETRY_CONTEXT: TelemetryContext = {
  agentsHealthy: null,
  pipelineDegraded: null,
  graphSampled: false,
  ingestionStale: null,
  graphPolicyLimited: false,
};
