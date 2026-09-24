import { useCallback, useEffect, useState } from 'react';
import { EMPTY_CLUSTERS, useEntityStore } from '../store/entityStore';
import { api, getAvailabilityIssue, type AvailabilityIssue } from '../lib/api';
import type { Cluster } from '../types';

/**
 * Shared hook for cluster data. Uses the entity store to deduplicate
 * api.getClusters() calls across Layout, Insights, NetworkActivity,
 * PodDetail, and any other consumer.
 *
 * Returns cached clusters synchronously and triggers background
 * revalidation if the cache is stale (>60s).
 */
export function useClusters(options: { enabled?: boolean } = {}): {
  clusters: Cluster[];
  loading: boolean;
  availabilityIssue: AvailabilityIssue | null;
  refresh: () => void;
} {
  const enabled = options.enabled !== false;
  const clusters = useEntityStore((s) => s.clusters?.data ?? EMPTY_CLUSTERS);
  const fetchClusters = useEntityStore((s) => s.fetchClusters);
  const [loading, setLoading] = useState(false);
  const [availabilityIssue, setAvailabilityIssue] = useState<AvailabilityIssue | null>(null);

  const runFetch = useCallback(async () => {
    if (!enabled) return;
    setLoading(true);
    try {
      await fetchClusters(() => api.getClustersStrict());
      setAvailabilityIssue(null);
    } catch (err) {
      setAvailabilityIssue(getAvailabilityIssue(err, 'Cluster inventory'));
    } finally {
      setLoading(false);
    }
  }, [enabled, fetchClusters]);

  useEffect(() => {
    void runFetch();
  }, [runFetch]);

  const refresh = () => {
    if (!enabled) return;
    useEntityStore.getState().invalidateClusters();
    void runFetch();
  };

  return { clusters, loading, availabilityIssue, refresh };
}
