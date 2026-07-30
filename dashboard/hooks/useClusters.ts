import { useEffect, useRef } from 'react';
import { EMPTY_CLUSTERS, useEntityStore } from '../store/entityStore';
import { api } from '../lib/api';
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
  refresh: () => void;
} {
  const enabled = options.enabled !== false;
  const clusters = useEntityStore((s) => s.clusters?.data ?? EMPTY_CLUSTERS);
  const fetchClusters = useEntityStore((s) => s.fetchClusters);
  const loadingRef = useRef(false);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    loadingRef.current = true;
    fetchClusters(() => api.getClusters())
      .catch(() => [] as Cluster[])
      .finally(() => {
        if (!cancelled) loadingRef.current = false;
      });
    return () => { cancelled = true; };
  }, [enabled, fetchClusters]);

  const refresh = () => {
    if (!enabled) return;
    useEntityStore.getState().invalidateClusters();
    fetchClusters(() => api.getClusters()).catch(() => {});
  };

  return { clusters, loading: loadingRef.current, refresh };
}
