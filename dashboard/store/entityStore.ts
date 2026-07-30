import { create } from 'zustand';
import type { Cluster, Insight, PodWithRisk } from '../types';

/* ─── cache entry with staleness tracking ─────────────────── */

interface CacheEntry<T> {
  data: T;
  fetchedAt: number;
  /** In-flight promise to prevent duplicate fetches */
  pending?: Promise<T>;
}

const DEFAULT_TTL_MS = 30_000; // 30 seconds
export const EMPTY_CLUSTERS: Cluster[] = [];

function isStale(entry: CacheEntry<unknown> | undefined, ttlMs = DEFAULT_TTL_MS): boolean {
  if (!entry) return true;
  return Date.now() - entry.fetchedAt > ttlMs;
}

/* ─── cluster cache (global, rarely changes) ──────────────── */

interface ClusterCache {
  clusters: CacheEntry<Cluster[]> | null;
  /** Get clusters with dedup — only one fetch in flight at a time */
  fetchClusters: (fetcher: () => Promise<Cluster[]>) => Promise<Cluster[]>;
  /** Direct set (from WebSocket or manual refresh) */
  setClusters: (clusters: Cluster[]) => void;
  /** Get cached value synchronously (may be stale) */
  getClustersSync: () => Cluster[];
  invalidateClusters: () => void;
}

/* ─── insight cache (by id) ───────────────────────────────── */

interface InsightCache {
  insightsById: Map<string, CacheEntry<Insight>>;
  setInsight: (insight: Insight) => void;
  setInsights: (insights: Insight[]) => void;
  getInsight: (id: string) => Insight | undefined;
  invalidateInsight: (id: string) => void;
  /** Optimistic status update (ack/resolve/dismiss) with rollback */
  optimisticUpdateInsight: (id: string, patch: Partial<Insight>) => () => void;
}

/* ─── pod cache (by uid) ──────────────────────────────────── */

interface PodCache {
  podsByUid: Map<string, CacheEntry<PodWithRisk>>;
  setPod: (pod: PodWithRisk) => void;
  getPod: (uid: string) => PodWithRisk | undefined;
  isPodStale: (uid: string, ttlMs?: number) => boolean;
  invalidatePod: (uid: string) => void;
}

/* ─── combined store ──────────────────────────────────────── */

type EntityState = ClusterCache & InsightCache & PodCache;

// We use a singleton promise to deduplicate cluster fetches
let clusterFetchPromise: Promise<Cluster[]> | null = null;

export const useEntityStore = create<EntityState>()((set, get) => ({
  /* ── clusters ─────────────────────────────────────────────── */
  clusters: null,

  /** Fetch clusters with deduplication — only one fetch in flight at a time. */
  fetchClusters: async (fetcher) => {
    const state = get();
    // Return cached if fresh
    if (state.clusters && !isStale(state.clusters, 60_000)) {
      return state.clusters.data;
    }
    // Deduplicate in-flight requests
    if (clusterFetchPromise) {
      return clusterFetchPromise;
    }
    clusterFetchPromise = fetcher()
      .then((data) => {
        set({ clusters: { data, fetchedAt: Date.now() } });
        clusterFetchPromise = null;
        return data;
      })
      .catch((err) => {
        clusterFetchPromise = null;
        throw err;
      });
    return clusterFetchPromise;
  },

  /** Direct set (from WebSocket or manual refresh). */
  setClusters: (clusters) => {
    set({ clusters: { data: clusters, fetchedAt: Date.now() } });
  },

  /** Get cached value synchronously (may be stale). */
  getClustersSync: () => {
    return get().clusters?.data ?? EMPTY_CLUSTERS;
  },

  invalidateClusters: () => {
    set({ clusters: null });
    clusterFetchPromise = null;
  },

  /* ── insights ─────────────────────────────────────────────── */
  insightsById: new Map(),

  /** Set a single insight in the cache. */
  setInsight: (insight) => {
    set((state) => {
      const next = new Map(state.insightsById);
      next.set(insight.id, { data: insight, fetchedAt: Date.now() });
      return { insightsById: next };
    });
  },

  /** Set multiple insights in the cache. */
  setInsights: (insights) => {
    set((state) => {
      const next = new Map(state.insightsById);
      const now = Date.now();
      for (const insight of insights) {
        next.set(insight.id, { data: insight, fetchedAt: now });
      }
      return { insightsById: next };
    });
  },

  /** Get an insight by id. */
  getInsight: (id) => {
    return get().insightsById.get(id)?.data;
  },

  invalidateInsight: (id) => {
    set((state) => {
      const next = new Map(state.insightsById);
      next.delete(id);
      return { insightsById: next };
    });
  },

  /** Optimistic update with rollback function. */
  optimisticUpdateInsight: (id, patch) => {
    const prev = get().insightsById.get(id);
    if (!prev) return () => {};

    const rollbackData = { ...prev.data };
    set((state) => {
      const next = new Map(state.insightsById);
      next.set(id, { data: { ...prev.data, ...patch }, fetchedAt: Date.now() });
      return { insightsById: next };
    });

    // Return rollback function
    return () => {
      set((state) => {
        const next = new Map(state.insightsById);
        next.set(id, { data: rollbackData, fetchedAt: prev.fetchedAt });
        return { insightsById: next };
      });
    };
  },

  /* ── pods ──────────────────────────────────────────────────── */
  podsByUid: new Map(),

  /** Set a pod in the cache. */
  setPod: (pod) => {
    set((state) => {
      const next = new Map(state.podsByUid);
      next.set(pod.uid, { data: pod, fetchedAt: Date.now() });
      return { podsByUid: next };
    });
  },

  /** Get a pod by uid. */
  getPod: (uid) => {
    return get().podsByUid.get(uid)?.data;
  },

  isPodStale: (uid, ttlMs) => {
    return isStale(get().podsByUid.get(uid), ttlMs);
  },

  /** Invalidate a pod from the cache. */
  invalidatePod: (uid) => {
    set((state) => {
      const next = new Map(state.podsByUid);
      next.delete(uid);
      return { podsByUid: next };
    });
  },
}));
