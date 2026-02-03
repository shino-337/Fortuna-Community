import React, { useEffect } from 'react';
import { RefreshCw } from 'lucide-react';
import { useRefreshIntervalStore, REFRESH_OPTIONS } from '../store/refreshIntervalStore';

const DEFAULT_INTERVAL_MS = 5 * 60 * 1000;

export const RefreshIntervalSelector: React.FC = () => {
  const { intervalMs, setIntervalMs } = useRefreshIntervalStore();
  useEffect(() => {
    if (intervalMs <= 0) setIntervalMs(DEFAULT_INTERVAL_MS);
  }, [intervalMs, setIntervalMs]);
  const displayMs = intervalMs > 0 && REFRESH_OPTIONS.some((o) => o.valueMs === intervalMs)
    ? intervalMs
    : DEFAULT_INTERVAL_MS;

  return (
    <div className="flex items-center gap-2" title="Auto-refresh interval for data on this page. Use filters to narrow results.">
      <RefreshCw className="w-4 h-4 text-slate-500 shrink-0" />
      <label htmlFor="refresh-interval" className="text-xs text-slate-500 whitespace-nowrap sr-only sm:not-sr-only">
        Refresh:
      </label>
      <select
        id="refresh-interval"
        value={displayMs}
        onChange={(e) => setIntervalMs(Number(e.target.value))}
        className="bg-slate-900/80 border border-slate-700 rounded-lg px-3 py-1.5 text-sm text-slate-200 focus:outline-none focus:ring-1 focus:ring-pink-500 focus:border-pink-500"
        title="Data refreshes every selected interval. Use page filters to see more specific data."
      >
        {REFRESH_OPTIONS.map((opt) => (
          <option key={opt.valueMs} value={opt.valueMs}>
            {opt.label}
          </option>
        ))}
      </select>
    </div>
  );
};
