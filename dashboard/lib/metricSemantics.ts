/**
 * Layer 4 — Metric semantics: distinguish zero vs hidden vs sampled vs stale vs scoped.
 */
import type { OwnershipContext } from './ownershipContext';
import { DEFAULT_TELEMETRY_CONTEXT, type TelemetryContext } from './telemetryContext';
import { scopeSummaryLabel } from './operationalScope';

export type MetricTrust = 'exact' | 'sample' | 'page';

export type MetricSemanticState =
  | 'scoped_exact'
  | 'exact_zero'
  | 'unknown'
  | 'hidden'
  | 'sampled'
  | 'stale'
  | 'partial'
  | 'inferred';

export interface MetricSemanticsInput {
  value: number | null | undefined;
  label: string;
  ownership: OwnershipContext;
  telemetry?: TelemetryContext;
  trust?: MetricTrust;
  clusterName?: string | null;
}

export interface MetricSemanticsResult {
  displayValue: string;
  subtitle: string;
  semanticState: MetricSemanticState;
  ariaLabel: string;
  showCaveat: boolean;
}

export function resolveMetricSemantics(input: MetricSemanticsInput): MetricSemanticsResult {
  const {
    value,
    label,
    ownership,
    telemetry = DEFAULT_TELEMETRY_CONTEXT,
    trust = 'exact',
    clusterName,
  } = input;
  const scopeLabel = scopeSummaryLabel(ownership.operationalScope, clusterName ?? undefined);
  const n = value == null || Number.isNaN(Number(value)) ? null : Number(value);

  if (!ownership.hasOperationalScope) {
    return {
      displayValue: '—',
      subtitle: 'Outside your operational scope — no assigned clusters or environments.',
      semanticState: 'hidden',
      ariaLabel: `${label}: not visible in your scope`,
      showCaveat: true,
    };
  }

  if (ownership.selectedClusterOutOfScope) {
    return {
      displayValue: '—',
      subtitle: 'Selected cluster is outside your assigned scope.',
      semanticState: 'hidden',
      ariaLabel: `${label}: selected cluster out of scope`,
      showCaveat: true,
    };
  }

  if (telemetry.ingestionStale === true || telemetry.pipelineDegraded === true) {
    const display = n == null ? '—' : String(n);
    return {
      displayValue: display,
      subtitle: `Count may be incomplete — telemetry is stale or degraded for ${scopeLabel}.`,
      semanticState: 'stale',
      ariaLabel: `${label}: ${display}, telemetry stale in scope`,
      showCaveat: true,
    };
  }

  if (trust === 'sample') {
    const display = n == null ? '—' : String(n);
    return {
      displayValue: display,
      subtitle: `Sampled count (≤1,000 rows) within ${scopeLabel} — not a guaranteed full inventory.`,
      semanticState: 'sampled',
      ariaLabel: `${label}: ${display}, sampled in scope`,
      showCaveat: true,
    };
  }

  if (trust === 'page') {
    const display = n == null ? '—' : String(n);
    return {
      displayValue: display,
      subtitle: `Partial count from current page only within ${scopeLabel}.`,
      semanticState: 'partial',
      ariaLabel: `${label}: ${display}, page-only in scope`,
      showCaveat: true,
    };
  }

  if (n == null) {
    return {
      displayValue: '—',
      subtitle: `Unknown — data not loaded for ${scopeLabel}.`,
      semanticState: 'unknown',
      ariaLabel: `${label}: unknown in scope`,
      showCaveat: false,
    };
  }

  if (n === 0) {
    return {
      displayValue: '0',
      subtitle: `No ${label.toLowerCase()} exist within ${scopeLabel}.`,
      semanticState: 'exact_zero',
      ariaLabel: `${label}: zero in assigned scope`,
      showCaveat: false,
    };
  }

  if (telemetry.graphPolicyLimited) {
    return {
      displayValue: String(n),
      subtitle: `${n} in ${scopeLabel}; some related paths may be policy-limited.`,
      semanticState: 'inferred',
      ariaLabel: `${label}: ${n} in scope, policy may limit completeness`,
      showCaveat: true,
    };
  }

  return {
    displayValue: String(n),
    subtitle: `${n} within ${scopeLabel}.`,
    semanticState: 'scoped_exact',
    ariaLabel: `${label}: ${n} in assigned scope`,
    showCaveat: false,
  };
}

export function metricTrustFromSeverityCountTrust(trust: 'exact' | 'sample' | 'page'): MetricTrust {
  return trust;
}
