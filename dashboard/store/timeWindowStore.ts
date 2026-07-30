import { create } from 'zustand';
import { persist } from 'zustand/middleware';

/** Time range options: data from the last N minutes. "All" = no time filter. */
export const TIME_WINDOW_OPTIONS = [
  { label: 'All', valueMinutes: 0 },
  { label: 'Last 1m', valueMinutes: 1 },
  { label: 'Last 5m', valueMinutes: 5 },
  { label: 'Last 10m', valueMinutes: 10 },
  { label: 'Last 15m', valueMinutes: 15 },
  { label: 'Last 30m', valueMinutes: 30 },
] as const;

export type TimeWindowOption = (typeof TIME_WINDOW_OPTIONS)[number];

interface TimeWindowState {
  /** Selected "last N minutes"; 0 = show all (no time filter). */
  valueMinutes: number;
  setValueMinutes: (minutes: number) => void;
}

/** Get the selected time window in milliseconds. */
export const useTimeWindowStore = create<TimeWindowState>()(
  persist(
    (set) => ({
      /** Selected "last N minutes"; 0 = show all (no time filter). */
      valueMinutes: 15,
      setValueMinutes: (minutes) => set({ valueMinutes: minutes }),
    }),
    {
      name: 'fortuna-time-window',
      partialize: (s) => ({ valueMinutes: s.valueMinutes }),
    }
  )
);
