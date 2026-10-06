import { useCallback, useRef, useState } from 'react';
import { api } from '../../lib/api';
import type { Insight, PipelineHealth, ThreatVelocityPoint } from '../../types';

/** One Home section: its last good data, plus whether the latest load failed. */
export interface HomeSection<T> {
  data: T | null;
  loading: boolean;
  failed: boolean;
}

export interface HomeData {
  /** Open findings nobody has acknowledged yet, highest risk score first. */
  queue: HomeSection<{ items: Insight[]; total: number }>;
  /** Open findings at the critical risk level (score 70 or higher). */
  criticalOpen: HomeSection<number>;
  /** Workloads whose risk score puts them at high or critical. */
  exposedWorkloads: HomeSection<number>;
  /** New findings per day for the last 30 days, by risk level. */
  trend: HomeSection<ThreatVelocityPoint[]>;
  inventory: HomeSection<{ agents: number; clusters: number; clusterName?: string }>;
  /** Only loaded for roles that can read platform health. */
  pipeline: HomeSection<PipelineHealth>;
  loadedAt: Date | null;
  refresh: () => Promise<void>;
}

export const TREND_DAYS = 30;
const QUEUE_SIZE = 5;

function emptySection<T>(): HomeSection<T> {
  return { data: null, loading: true, failed: false };
}

function settle<T>(prev: HomeSection<T>, result: PromiseSettledResult<T>): HomeSection<T> {
  return result.status === 'fulfilled'
    ? { data: result.value, loading: false, failed: false }
    : { data: prev.data, loading: false, failed: true };
}

/** Loads everything Home shows in one round, scoped to the header cluster and time window. */
export function useHomeData(
  clusterId: string | null | undefined,
  sinceMinutes: number | undefined,
  options: { canReadPipeline: boolean },
): HomeData {
  const [queue, setQueue] = useState(emptySection<{ items: Insight[]; total: number }>);
  const [criticalOpen, setCriticalOpen] = useState(emptySection<number>);
  const [exposedWorkloads, setExposedWorkloads] = useState(emptySection<number>);
  const [trend, setTrend] = useState(emptySection<ThreatVelocityPoint[]>);
  const [inventory, setInventory] = useState(emptySection<{ agents: number; clusters: number; clusterName?: string }>);
  const [pipeline, setPipeline] = useState(emptySection<PipelineHealth>);
  const [loadedAt, setLoadedAt] = useState<Date | null>(null);
  const requestSeq = useRef(0);
  const { canReadPipeline } = options;

  const refresh = useCallback(async () => {
    // A cluster or time-window change can overlap a poll; only the newest load may write state.
    const seq = ++requestSeq.current;
    const cluster = clusterId ?? undefined;
    const [queueResult, summaryResult, podsResult, trendResult, statsResult, pipelineResult] = await Promise.allSettled([
      api
        .getRisks({ status: 'active', sort: 'score', order: 'desc', page: 1, pageSize: QUEUE_SIZE, clusterId: cluster, sinceMinutes })
        .then(({ insights, total }) => ({ items: insights, total: total ?? insights.length })),
      api.getInsightsSummary(cluster, sinceMinutes).then((s) => {
        // Risk levels only; a summary without them must not fall back to rule severity.
        if (!s?.riskLevelCounts) throw new Error('Risk levels unavailable');
        return Number(s.riskLevelCounts.critical ?? 0);
      }),
      // Score 40 is where the high risk level starts.
      api.countRiskScoresAtLeast(40, cluster),
      api.getThreatVelocity(TREND_DAYS, cluster, 'all'),
      api.getStats(cluster, sinceMinutes, 'all').then((s) => ({
        agents: Number(s.agents ?? 0),
        clusters: Number(s.clusters ?? 0),
        clusterName: s.clusterName,
      })),
      canReadPipeline ? api.getPipelineHealth() : Promise.reject(new Error('Not permitted')),
    ]);
    if (seq !== requestSeq.current) return;

    setQueue((prev) => settle(prev, queueResult));
    setCriticalOpen((prev) => settle(prev, summaryResult));
    setExposedWorkloads((prev) => settle(prev, podsResult));
    setTrend((prev) => settle(prev, trendResult));
    setInventory((prev) => settle(prev, statsResult));
    setPipeline((prev) =>
      canReadPipeline ? settle(prev, pipelineResult) : { data: null, loading: false, failed: false },
    );
    setLoadedAt(new Date());
  }, [clusterId, sinceMinutes, canReadPipeline]);

  return { queue, criticalOpen, exposedWorkloads, trend, inventory, pipeline, loadedAt, refresh };
}
