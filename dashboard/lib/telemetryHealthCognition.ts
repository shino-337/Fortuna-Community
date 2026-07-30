/**
 * Telemetry dependency awareness — explains which pipelines affect which views.
 */
import type { TelemetryContext } from './telemetryContext';

export type TelemetryPipeline =
  | 'agent_heartbeat'
  | 'runtime_events'
  | 'identity_inventory'
  | 'network_flows'
  | 'risk_scoring'
  | 'graph_topology';

export interface PipelineHealth {
  pipeline: TelemetryPipeline;
  label: string;
  healthy: boolean | null;
  lagMinutes?: number;
  impacts: string[];
}

const PIPELINE_META: Record<TelemetryPipeline, { label: string; impacts: string[] }> = {
  agent_heartbeat: { label: 'Agent heartbeat', impacts: ['cluster health', 'agent coverage'] },
  runtime_events: { label: 'Runtime security events', impacts: ['runtime-confirmed findings', 'exploit validation'] },
  identity_inventory: { label: 'Identity / RBAC inventory', impacts: ['attack path graph', 'RBAC blast radius'] },
  network_flows: { label: 'Network Activity', impacts: ['network topology graph', 'lateral movement edges'] },
  risk_scoring: { label: 'Risk scoring pipeline', impacts: ['severity counts', 'risk operations queue'] },
  graph_topology: { label: 'Graph topology sync', impacts: ['attack paths', 'dependency chains'] },
};

export interface TelemetryHealthCognition {
  overallDegraded: boolean;
  pipelines: PipelineHealth[];
  graphConfidenceImpact: string;
  metricConfidenceImpact: string;
  primaryCause: string | null;
}

export function assessTelemetryHealth(
  telemetry: TelemetryContext,
  opts?: { identityLagMinutes?: number; runtimeLagMinutes?: number },
): TelemetryHealthCognition {
  const pipelines: PipelineHealth[] = [
    {
      pipeline: 'agent_heartbeat',
      label: PIPELINE_META.agent_heartbeat.label,
      healthy: telemetry.agentsHealthy,
      impacts: PIPELINE_META.agent_heartbeat.impacts,
    },
    {
      pipeline: 'runtime_events',
      label: PIPELINE_META.runtime_events.label,
      healthy: telemetry.ingestionStale === true ? false : telemetry.agentsHealthy,
      lagMinutes: opts?.runtimeLagMinutes,
      impacts: PIPELINE_META.runtime_events.impacts,
    },
    {
      pipeline: 'identity_inventory',
      label: PIPELINE_META.identity_inventory.label,
      healthy:
        opts?.identityLagMinutes != null && opts.identityLagMinutes > 30
          ? false
          : telemetry.pipelineDegraded === true
            ? false
            : null,
      lagMinutes: opts?.identityLagMinutes,
      impacts: PIPELINE_META.identity_inventory.impacts,
    },
    {
      pipeline: 'network_flows',
      label: PIPELINE_META.network_flows.label,
      healthy: telemetry.pipelineDegraded === true ? false : null,
      impacts: PIPELINE_META.network_flows.impacts,
    },
    {
      pipeline: 'risk_scoring',
      label: PIPELINE_META.risk_scoring.label,
      healthy: telemetry.pipelineDegraded === true ? false : null,
      impacts: PIPELINE_META.risk_scoring.impacts,
    },
    {
      pipeline: 'graph_topology',
      label: PIPELINE_META.graph_topology.label,
      healthy:
        telemetry.ingestionStale === true || telemetry.pipelineDegraded === true
          ? false
          : telemetry.graphSampled
            ? null
            : true,
      impacts: PIPELINE_META.graph_topology.impacts,
    },
  ];

  const degraded = pipelines.filter((p) => p.healthy === false);
  const identityBad = degraded.find((p) => p.pipeline === 'identity_inventory');
  const runtimeBad = degraded.find((p) => p.pipeline === 'runtime_events');

  let graphConfidenceImpact = 'Graph relationships reflect the latest available inventory and telemetry.';
  if (identityBad) {
    graphConfidenceImpact = `Graph confidence is degraded because ${identityBad.label} is lagging${identityBad.lagMinutes ? ` (~${identityBad.lagMinutes} min)` : ''} — RBAC and identity edges may be incomplete.`;
  } else if (telemetry.graphSampled) {
    graphConfidenceImpact = 'Graph is sampled or clustered; relationship completeness is limited for performance.';
  } else if (telemetry.graphPolicyLimited) {
    graphConfidenceImpact = 'Graph traversal is policy-limited; some edges are intentionally hidden.';
  }

  let metricConfidenceImpact = 'Metrics use summary APIs within your operational scope.';
  if (telemetry.ingestionStale || telemetry.pipelineDegraded) {
    metricConfidenceImpact = 'Metric counts may under-represent active threats while ingestion or scoring pipelines are degraded.';
  }

  const primaryCause =
    identityBad?.label ??
    runtimeBad?.label ??
    (telemetry.pipelineDegraded ? 'Pipeline degradation' : null) ??
    (telemetry.ingestionStale ? 'Stale ingestion' : null);

  return {
    overallDegraded: degraded.length > 0 || telemetry.ingestionStale === true || telemetry.pipelineDegraded === true,
    pipelines,
    graphConfidenceImpact,
    metricConfidenceImpact,
    primaryCause,
  };
}
