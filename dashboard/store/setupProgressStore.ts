import { create } from 'zustand';
import { api } from '../lib/api';

export type SetupStepId = 'agent' | 'scan' | 'triage' | 'team' | 'cluster';

/** done: finished; todo: not yet; unknown: the check could not be read. */
export type SetupStepState = 'done' | 'todo' | 'unknown';

export interface SetupStep {
  id: SetupStepId;
  state: SetupStepState;
  optional: boolean;
  /** What was found, e.g. "3 agents reporting". */
  detail: string;
}

interface SetupProgressState {
  steps: SetupStep[] | null;
  loading: boolean;
  loadedAt: number | null;
  load: () => Promise<void>;
  /** Loads only when the last result is older than maxAgeMs, so the sidebar badge does not refetch on every page. */
  loadIfStale: (maxAgeMs: number) => void;
  /** Forgets the last result, e.g. when another user signs in. */
  reset: () => void;
}

export const SETUP_STEP_ORDER: SetupStepId[] = ['agent', 'scan', 'triage', 'team', 'cluster'];

/** Setup is finished when every required step is done; the optional one never holds it open. */
export function setupComplete(steps: SetupStep[] | null): boolean {
  return steps !== null && steps.every((s) => s.optional || s.state === 'done');
}

export function setupDoneCount(steps: SetupStep[] | null): number {
  return steps?.filter((s) => s.state === 'done').length ?? 0;
}

function plural(n: number, word: string): string {
  return `${n.toLocaleString()} ${word}${n === 1 ? '' : 's'}`;
}

let inflight: Promise<void> | null = null;

/** First-run checklist for admins, read from data Fortuna already has; nothing is stored. */
export const useSetupProgressStore = create<SetupProgressState>()((set, get) => ({
  steps: null,
  loading: false,
  loadedAt: null,
  reset: () => set({ steps: null, loadedAt: null }),
  loadIfStale: (maxAgeMs) => {
    const { loadedAt, load } = get();
    if (loadedAt === null || Date.now() - loadedAt > maxAgeMs) void load();
  },
  load: () => {
    if (inflight) return inflight;
    set({ loading: true });
    inflight = (async () => {
      const [agents, sync, acknowledged, resolved, users, clusters] = await Promise.allSettled([
        api.getAgents(),
        api.getSyncStatus(),
        api.getRisks({ status: 'acknowledged', page: 1, pageSize: 1, withScores: 0 }),
        api.getRisks({ status: 'resolved', page: 1, pageSize: 1, withScores: 0 }),
        api.getUsers(),
        api.getClusters(),
      ]);
      const steps: SetupStep[] = [];

      if (agents.status === 'fulfilled') {
        const up = agents.value.filter((a) => a.status === 'up').length;
        steps.push({ id: 'agent', optional: false, state: up > 0 ? 'done' : 'todo', detail: up > 0 ? `${plural(up, 'agent')} reporting` : 'No agent is reporting yet' });
      } else steps.push({ id: 'agent', optional: false, state: 'unknown', detail: 'Agent status could not be read' });

      if (sync.status === 'fulfilled') {
        const last = sync.value.lastScan;
        steps.push({ id: 'scan', optional: false, state: last ? 'done' : 'todo', detail: last ? `${plural(sync.value.resources.pods, 'pod')} inventoried` : 'No scan has finished yet' });
      } else steps.push({ id: 'scan', optional: false, state: 'unknown', detail: 'Scan status could not be read' });

      if (acknowledged.status === 'fulfilled' && resolved.status === 'fulfilled') {
        const n = acknowledged.value.total + resolved.value.total;
        steps.push({ id: 'triage', optional: false, state: n > 0 ? 'done' : 'todo', detail: n > 0 ? `${plural(n, 'finding')} in review or resolved` : 'Every finding is still new' });
      } else steps.push({ id: 'triage', optional: false, state: 'unknown', detail: 'Findings could not be read' });

      if (users.status === 'fulfilled') {
        const n = users.value.length;
        steps.push({ id: 'team', optional: false, state: n > 1 ? 'done' : 'todo', detail: n > 1 ? `${plural(n, 'user')}` : 'Only one user so far' });
      } else steps.push({ id: 'team', optional: false, state: 'unknown', detail: 'Users could not be read' });

      if (clusters.status === 'fulfilled') {
        const n = clusters.value.length;
        steps.push({ id: 'cluster', optional: true, state: n > 1 ? 'done' : 'todo', detail: plural(n, 'cluster') });
      } else steps.push({ id: 'cluster', optional: true, state: 'unknown', detail: 'Clusters could not be read' });

      set({ steps, loading: false, loadedAt: Date.now() });
    })().finally(() => {
      inflight = null;
      set({ loading: false });
    });
    return inflight;
  },
}));
