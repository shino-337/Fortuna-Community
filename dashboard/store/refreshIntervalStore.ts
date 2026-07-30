import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export const REFRESH_OPTIONS = [
  { label: '30s', valueMs: 30 * 1000 },
  { label: '1 min', valueMs: 60 * 1000 },
  { label: '5 min', valueMs: 5 * 60 * 1000 },
  { label: '10 min', valueMs: 10 * 60 * 1000 },
  { label: '15 min', valueMs: 15 * 60 * 1000 },
  { label: '30 min', valueMs: 30 * 60 * 1000 },
] as const;

export type RefreshOptionLabel = typeof REFRESH_OPTIONS[number]['label'];

interface RefreshIntervalState {
  /** Selected interval in ms (always > 0; no "Off" option) */
  intervalMs: number;
  setIntervalMs: (ms: number) => void;
  /** Get interval for a page; if stored value invalid/0, return fallback so initial load always runs */
  getIntervalMs: (fallbackMs: number) => number;
}

const DEFAULT_INTERVAL_MS = 5 * 60 * 1000; // 5 min

export const useRefreshIntervalStore = create<RefreshIntervalState>()(
  persist(
    (set, get) => ({
      /** Selected refresh interval in ms. */
      intervalMs: DEFAULT_INTERVAL_MS,
      setIntervalMs: (ms) => set({ intervalMs: ms > 0 ? ms : DEFAULT_INTERVAL_MS }),
      /** Get the stored interval; returns fallback if invalid. */
      getIntervalMs: (fallbackMs) => {
        const { intervalMs } = get();
        return intervalMs > 0 ? intervalMs : fallbackMs;
      },
    }),
    {
      name: 'fortuna-refresh-interval',
      partialize: (s) => ({ intervalMs: s.intervalMs }),
    }
  )
);
