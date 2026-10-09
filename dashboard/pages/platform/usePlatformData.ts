import { useCallback, useRef, useState } from 'react';
import { api } from '../../lib/api';
import type { Agent, CatalogHealth, Certificate, ErrorLog, PipelineHealth, RuntimeHealth, SyncStatus } from '../../types';

/** One Platform section: its last good data, and whether the latest load failed. */
export interface PlatformSection<T> {
  data: T | null;
  failed: boolean;
}

export interface PlatformAccess {
  metrics: boolean;
  agents: boolean;
  logs: boolean;
  certificates: boolean;
  /** Cluster-scoped users do not get the cross-cluster integrity check (catalog and runtime ingest). */
  clusterScoped: boolean;
}

/** Cross-cluster integrity check: catalog and runtime ingest health, and the alerts Core derived from them. */
export interface PlatformIntegrity {
  catalog: CatalogHealth;
  runtime: RuntimeHealth | null;
  alerts: string[];
}

export interface PlatformData {
  agents: PlatformSection<Agent[]>;
  pipeline: PlatformSection<PipelineHealth>;
  sync: PlatformSection<SyncStatus>;
  integrity: PlatformSection<PlatformIntegrity>;
  certificates: PlatformSection<Certificate[]>;
  recentErrors: PlatformSection<ErrorLog[]>;
  loadedAt: Date | null;
  refresh: () => Promise<void>;
}

/** Errors read for the "last 24 hours" summary; the log API has no time filter, so the newest page is read. */
export const RECENT_ERROR_SAMPLE = 200;

const empty = <T,>(): PlatformSection<T> => ({ data: null, failed: false });

function settle<T>(prev: PlatformSection<T>, result: PromiseSettledResult<T> | null): PlatformSection<T> {
  if (result === null) return { data: null, failed: false };
  return result.status === 'fulfilled' ? { data: result.value, failed: false } : { data: prev.data, failed: true };
}

/** Loads every Platform section in one round; a section the role cannot read is skipped. */
export function usePlatformData(access: PlatformAccess): PlatformData {
  const [agents, setAgents] = useState(empty<Agent[]>);
  const [pipeline, setPipeline] = useState(empty<PipelineHealth>);
  const [sync, setSync] = useState(empty<SyncStatus>);
  const [integrity, setIntegrity] = useState(empty<PlatformIntegrity>);
  const [certificates, setCertificates] = useState(empty<Certificate[]>);
  const [recentErrors, setRecentErrors] = useState(empty<ErrorLog[]>);
  const [loadedAt, setLoadedAt] = useState<Date | null>(null);
  const seq = useRef(0);
  const { metrics, agents: canAgents, logs, certificates: canCerts, clusterScoped } = access;

  const refresh = useCallback(async () => {
    const mine = ++seq.current;
    const maybe = <T,>(allowed: boolean, load: () => Promise<T>) =>
      allowed ? Promise.allSettled([load()]).then(([r]) => r) : Promise.resolve(null);
    const [a, p, s, c, cert, e] = await Promise.all([
      maybe(canAgents, () => api.getAgents()),
      maybe(metrics, () => api.getPipelineHealth()),
      maybe(metrics, () => api.getSyncStatus()),
      maybe(metrics && !clusterScoped, () => api.getDashboardDataIntegrity().then((d): PlatformIntegrity => ({ catalog: d.catalogHealth, runtime: d.runtimeHealth ?? null, alerts: Array.isArray(d.alerts) ? d.alerts : [] }))),
      maybe(canCerts, () => api.getCertificates()),
      maybe(logs, () => api.getErrorLogs({ page: 1, pageSize: RECENT_ERROR_SAMPLE }).then((r) => r.logs)),
    ]);
    if (mine !== seq.current) return;
    setAgents((prev) => settle(prev, a));
    setPipeline((prev) => settle(prev, p));
    setSync((prev) => settle(prev, s));
    setIntegrity((prev) => settle(prev, c));
    setCertificates((prev) => settle(prev, cert));
    setRecentErrors((prev) => settle(prev, e));
    setLoadedAt(new Date());
  }, [canAgents, metrics, clusterScoped, canCerts, logs]);

  return { agents, pipeline, sync, integrity, certificates, recentErrors, loadedAt, refresh };
}
