import { create } from 'zustand';

/** Manual refresh trigger: bump() to force all pages using usePolling(..., { refreshTrigger }) to refetch. */
interface RefreshTriggerState {
  trigger: number;
  bump: () => void;
}

export const useRefreshTriggerStore = create<RefreshTriggerState>()((set) => ({
  trigger: 0,
  bump: () => set((s) => ({ trigger: s.trigger + 1 })),
}));
