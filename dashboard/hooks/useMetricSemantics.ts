import { useMemo } from 'react';
import { useOperationalContext } from './useOperationalContext';
import {
  resolveMetricSemantics,
  type MetricSemanticsInput,
  type MetricSemanticsResult,
  type MetricTrust,
} from '../lib/metricSemantics';

/**
 * Hook that resolves metric semantics from partial input combined with operational context.
 * @param partial.value - The numeric value to evaluate (required)
 * @param partial.label - Optional label for the metric
 * @param partial.trust - Trust level: 'exact', 'sample', or 'page'
 * @param partial.clusterName - Cluster name override (defaults to operational context)
 * @param partial.telemetry - Partial telemetry data to merge with base telemetry
 */
export function useMetricSemantics(
  partial: Omit<MetricSemanticsInput, 'ownership' | 'telemetry'> & {
    telemetry?: Partial<MetricSemanticsInput['telemetry']>;
  },
): MetricSemanticsResult {
  const { ownership, telemetry: baseTelemetry } = useOperationalContext(partial.telemetry);
  return useMemo(
    () =>
      resolveMetricSemantics({
        ...partial,
        ownership,
        telemetry: { ...baseTelemetry, ...partial.telemetry },
      }),
    [
      partial.value,
      partial.label,
      partial.trust,
      partial.clusterName,
      ownership,
      baseTelemetry,
      partial.telemetry,
    ],
  );
}

export function useMetricTrust(severityCountTrust: 'exact' | 'sample' | 'page'): MetricTrust {
  return severityCountTrust;
}
