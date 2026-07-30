import { create } from 'zustand';

/** Manual refresh trigger: bump() to force all pages using usePolling(..., { refreshTrigger }) to refetch. */
interface RefreshTriggerState {
  trigger: number;
  bump: () => void;
}

/** Manual refresh trigger: bump() to force all pages using usePolling(..., { refreshTrigger }) to refetch. */
export const useRefreshTriggerStore = create<RefreshTriggerState>()((set) => ({
  /** Counter bumped on each manual refresh request. */
  trigger: 0,
  /** Bump the counter to invalidate cached data across tabs. */
  bump: () => set((s) => ({ trigger: s.trigger + 1 })),
}));
