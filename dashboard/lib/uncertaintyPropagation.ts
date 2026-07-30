/**
 * Uncertainty propagation — how trust degrades through chains, graphs, and metrics.
 */
import type { TelemetryContext } from './telemetryContext';
import type { GraphTrustPosture } from './graphTrustSemantics';
import { assessTelemetryHealth } from './telemetryHealthCognition';

export type UncertaintyKind =
  | 'trust_decay'
  | 'inferred_chain_decay'
  | 'stale_telemetry'
  | 'sampled_graph'
  | 'policy_hidden'
  | 'scope_limited';

export interface UncertaintySignal {
  kind: UncertaintyKind;
  severity: 'low' | 'medium' | 'high';
  source: string;
  propagatedTo: string[];
  message: string;
  confidenceMultiplier: number;
}

export interface UncertaintyPropagationResult {
  signals: UncertaintySignal[];
  /** 0–1 aggregate confidence for operational decisions in this view. */
  aggregateConfidence: number;
  summary: string;
}

export function propagateUncertainty(input: {
  telemetry: TelemetryContext;
  graphPosture?: GraphTrustPosture;
  inferredEdgeRatio?: number;
  runtimeVerified?: boolean;
  identityLagMinutes?: number;
}): UncertaintyPropagationResult {
  const signals: UncertaintySignal[] = [];
  const telemetryHealth = assessTelemetryHealth(input.telemetry, {
    identityLagMinutes: input.identityLagMinutes,
  });

  if (input.telemetry.ingestionStale || input.telemetry.pipelineDegraded) {
    signals.push({
      kind: 'stale_telemetry',
      severity: 'high',
      source: telemetryHealth.primaryCause ?? 'Telemetry ingestion',
      propagatedTo: ['metrics', 'attack_paths', 'runtime_findings', 'investigation_evidence'],
      message: telemetryHealth.metricConfidenceImpact,
      confidenceMultiplier: 0.55,
    });
  }

  if (telemetryHealth.pipelines.find((p) => p.pipeline === 'identity_inventory' && p.healthy === false)) {
    signals.push({
      kind: 'trust_decay',
      severity: 'high',
      source: 'Identity inventory lag',
      propagatedTo: ['attack_paths', 'blast_radius', 'graph_topology'],
      message: telemetryHealth.graphConfidenceImpact,
      confidenceMultiplier: 0.5,
    });
  }

  const inferredRatio = input.inferredEdgeRatio ?? 0;
  if (inferredRatio > 0.35) {
    signals.push({
      kind: 'inferred_chain_decay',
      severity: inferredRatio > 0.6 ? 'high' : 'medium',
      source: 'Inferred attack-step ratio',
      propagatedTo: ['exploitability_assessment', 'remediation_priority'],
      message: `${Math.round(inferredRatio * 100)}% of visible chain steps are inferred, not runtime-observed — exploitability confidence decays along the path.`,
      confidenceMultiplier: Math.max(0.35, 1 - inferredRatio * 0.7),
    });
  }

  if (input.graphPosture?.sampled || input.telemetry.graphSampled) {
    signals.push({
      kind: 'sampled_graph',
      severity: 'medium',
      source: 'Graph sampling / LOD',
      propagatedTo: ['attack_paths', 'blast_radius'],
      message: 'Sampled or clustered graph reduces certainty about hidden intermediate nodes.',
      confidenceMultiplier: 0.65,
    });
  }

  if (input.graphPosture?.policyLimited || input.telemetry.graphPolicyLimited) {
    signals.push({
      kind: 'policy_hidden',
      severity: 'medium',
      source: 'Policy-limited graph query',
      propagatedTo: ['attack_paths', 'network_topology'],
      message: 'Policy or permissions hide some relationships — blast radius may be understated.',
      confidenceMultiplier: 0.7,
    });
  }

  if (input.graphPosture?.completeness === 'scope_limited') {
    signals.push({
      kind: 'scope_limited',
      severity: 'medium',
      source: 'Operational scope',
      propagatedTo: ['all_views'],
      message: 'Current cluster or namespace scope limits visible assets.',
      confidenceMultiplier: 0.85,
    });
  }

  if (input.runtimeVerified === false) {
    signals.push({
      kind: 'inferred_chain_decay',
      severity: 'medium',
      source: 'Runtime unverified',
      propagatedTo: ['critical_findings', 'remediation_actions'],
      message: 'No runtime confirmation in scope — treat critical counts as latent exposure until validated.',
      confidenceMultiplier: 0.6,
    });
  }

  const aggregateConfidence =
    signals.length === 0
      ? 1
      : signals.reduce((acc, s) => Math.min(acc, s.confidenceMultiplier), 1);

  const summary =
    signals.length === 0
      ? 'Operational confidence is high for your current scope and telemetry posture.'
      : signals
          .slice(0, 2)
          .map((s) => s.message)
          .join(' ');

  return { signals, aggregateConfidence, summary };
}
