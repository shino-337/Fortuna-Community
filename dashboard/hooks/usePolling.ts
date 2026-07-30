import { useEffect, useRef, useCallback } from 'react';

/**
 * Polling intervals (ms) for fresh UI data. Align with agent sync and core schedulers.
 * - 30s: stats, clusters, agents (near real-time)
 * - 1 min: SBOM list, Risk list (insights)
 * - 5 min: PCE summary/trend, threat velocity (heavier)
 */
export const REFRESH_INTERVALS = {
  STATS_CLUSTERS: 30 * 1000,   // 30s
  SBOM_RISK_LIST: 60 * 1000,   // 1 min
  PCE_TREND: 5 * 60 * 1000,    // 5 min
  METRICS: 30 * 1000,          // 30s
} as const;

export type UsePollingOptions = {
  enabled?: boolean;
  /** When this value changes (e.g. manual refresh bump), run fetch once. */
  refreshTrigger?: number;
};

/**
 * Runs a fetch function on mount and then at the given interval.
 * If refreshTrigger is provided and changes (e.g. user clicked Manual Refresh), runs fetch once.
 */
export function usePolling(
  fetchFn: () => void | Promise<void>,
  intervalMs: number,
  options: boolean | UsePollingOptions = true
): void {
  const opts = typeof options === 'boolean' ? { enabled: options } : options;
  const enabled = opts?.enabled !== false;
  const refreshTrigger = opts?.refreshTrigger ?? 0;

  const savedFn = useRef(fetchFn);
  const enabledRef = useRef(enabled);
  savedFn.current = fetchFn;
  enabledRef.current = enabled;

  const stableFetch = useCallback(() => {
    if (enabledRef.current) {
      void Promise.resolve(savedFn.current());
    }
  }, []);

  useEffect(() => {
    if (!enabled || intervalMs <= 0) return;
    stableFetch();
    const id = setInterval(stableFetch, intervalMs);
    return () => clearInterval(id);
  }, [enabled, intervalMs, stableFetch]);

  useEffect(() => {
    if (refreshTrigger > 0) stableFetch();
  }, [refreshTrigger, stableFetch]);
}
