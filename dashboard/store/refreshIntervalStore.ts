import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export const REFRESH_OPTIONS = [
  { label: 'Off', valueMs: 0 },
  { label: '30s', valueMs: 30 * 1000 },
  { label: '1 min', valueMs: 60 * 1000 },
  { label: '5 min', valueMs: 5 * 60 * 1000 },
  { label: '30 min', valueMs: 30 * 60 * 1000 },
] as const;

export type RefreshOptionLabel = typeof REFRESH_OPTIONS[number]['label'];

interface RefreshIntervalState {
  /** Selected interval in ms; 0 = off */
  intervalMs: number;
  setIntervalMs: (ms: number) => void;
  /** Get interval for a page; if global is Off, return 0; else return global or fallback */
  getIntervalMs: (fallbackMs: number) => number;
}

export const useRefreshIntervalStore = create<RefreshIntervalState>()(
  persist(
    (set, get) => ({
      intervalMs: 30 * 1000, // default 30s
      setIntervalMs: (ms) => set({ intervalMs: ms }),
      getIntervalMs: (fallbackMs) => {
        const { intervalMs } = get();
        return intervalMs <= 0 ? 0 : intervalMs;
      },
    }),
    {
      name: 'fortuna-refresh-interval',
      partialize: (s) => ({ intervalMs: s.intervalMs }),
    }
  )
);
