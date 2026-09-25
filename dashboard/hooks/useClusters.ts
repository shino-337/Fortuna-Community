import { useCallback, useEffect, useRef, useState } from 'react';
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
  const clusterGeneration = useEntityStore((s) => s.clusterGeneration);
  const [loading, setLoading] = useState(false);
  const [availabilityIssue, setAvailabilityIssue] = useState<AvailabilityIssue | null>(null);
  const requestRef = useRef(0);

  const runFetch = useCallback(async () => {
    if (!enabled) return;
    const requestSeq = ++requestRef.current;
    const generation = clusterGeneration;
    setLoading(true);
    try {
      await fetchClusters(() => api.getClustersStrict());
      if (requestSeq !== requestRef.current || useEntityStore.getState().clusterGeneration !== generation) return;
      setAvailabilityIssue(null);
    } catch (err) {
      if (requestSeq !== requestRef.current || useEntityStore.getState().clusterGeneration !== generation) return;
      setAvailabilityIssue(getAvailabilityIssue(err, 'Cluster inventory'));
    } finally {
      if (requestSeq === requestRef.current && useEntityStore.getState().clusterGeneration === generation) {
        setLoading(false);
      }
    }
  }, [clusterGeneration, enabled, fetchClusters]);

  useEffect(() => {
    if (!enabled) {
      requestRef.current += 1;
      setLoading(false);
      setAvailabilityIssue(null);
      return;
    }
    void runFetch();
  }, [enabled, runFetch]);

  const refresh = () => {
    if (!enabled) return;
    // The shared generation change re-runs every enabled hook against the same
    // deduplicated fresh request; do not invoke the stale closure directly.
    useEntityStore.getState().invalidateClusters();
  };

  return { clusters, loading, availabilityIssue, refresh };
}
