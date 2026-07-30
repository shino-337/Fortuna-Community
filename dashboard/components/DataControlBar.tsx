import React, { useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { useTimeWindowStore, TIME_WINDOW_OPTIONS } from '../store/timeWindowStore';
import { useRefreshIntervalStore, REFRESH_OPTIONS } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';

const DEFAULT_INTERVAL_MS = 5 * 60 * 1000;

/**
 * Control bar for time-based data: Time Range (what window), Auto Refresh (how often), Manual Refresh (fetch now).
 * Time range = data window; refresh interval = how often to re-fetch; Refresh now = immediate fetch.
 */
export const DataControlBar: React.FC<{ className?: string; compact?: boolean }> = ({ className = '', compact = false }) => {
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);
  const setTimeWindowMinutes = useTimeWindowStore((s) => s.setValueMinutes);
  const { intervalMs, setIntervalMs } = useRefreshIntervalStore();
  const bump = useRefreshTriggerStore((s) => s.bump);
  const [refreshing, setRefreshing] = useState(false);

  const displayIntervalMs =
    intervalMs > 0 && REFRESH_OPTIONS.some((o) => o.valueMs === intervalMs) ? intervalMs : DEFAULT_INTERVAL_MS;

  const intervalLabel = (ms: number) => {
    if (ms >= 60000) return `Every ${ms / 60000}m`;
    return `Every ${ms / 1000}s`;
  };

  const handleManualRefresh = () => {
    if (refreshing) return;
    setRefreshing(true);
    bump();
    setTimeout(() => setRefreshing(false), 500);
  };

  return (
    <div
      className={`flex max-w-full min-w-0 flex-wrap items-center gap-2 rounded-lg border border-border bg-surface/80 px-2 py-2 sm:gap-3 sm:px-3 ${className}`.trim()}
      title="Time range = data window; refresh interval = re-fetch cadence; Refresh now = fetch immediately"
    >
      {/* 1. Time Range Selector – Last 1m / 5m / ... */}
      <div className="flex items-center gap-2">
        <label htmlFor="data-control-time-range" className="text-caption font-medium text-muted whitespace-nowrap shrink-0">
          <span className={compact ? 'inline' : 'hidden min-[1100px]:inline'}>{compact ? 'Range' : 'Time range'}</span>
          {!compact ? <span className="min-[1100px]:hidden">Range</span> : null}
        </label>
        <select
          id="data-control-time-range"
          value={timeWindowMinutes}
          onChange={(e) => setTimeWindowMinutes(Number(e.target.value))}
          className={`${compact ? 'min-w-[86px]' : 'min-w-[100px]'} h-10 rounded-md border border-border bg-surface-2 px-2.5 text-body text-text transition-colors focus:outline-none focus-visible:border-brand focus-visible:ring-2 focus-visible:ring-brand/50 lg:h-9`}
          title="Data window (from = now − Xm, to = now). Does not change refresh cadence."
        >
          {TIME_WINDOW_OPTIONS.map((opt) => (
            <option key={opt.valueMinutes} value={opt.valueMinutes}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>

      <div className="w-px h-5 bg-border" aria-hidden />

      {/* 2. Auto Refresh Interval – Every 1m / 5m / ... */}
      <div className="flex items-center gap-2">
        <label htmlFor="data-control-refresh-interval" className="text-caption font-medium text-muted whitespace-nowrap shrink-0">
          <span className={compact ? 'inline' : 'hidden min-[1100px]:inline'}>{compact ? 'Auto' : 'Auto refresh'}</span>
          {!compact ? <span className="min-[1100px]:hidden">Auto</span> : null}
        </label>
        <select
          id="data-control-refresh-interval"
          value={displayIntervalMs}
          onChange={(e) => setIntervalMs(Number(e.target.value))}
          className={`${compact ? 'min-w-[94px]' : 'min-w-[100px]'} h-10 rounded-md border border-border bg-surface-2 px-2.5 text-body text-text transition-colors focus:outline-none focus-visible:border-brand focus-visible:ring-2 focus-visible:ring-brand/50 lg:h-9`}
          title="How often to re-fetch automatically. Time window stays the same while &quot;now&quot; advances, so data slides with real time."
        >
          {REFRESH_OPTIONS.map((opt) => (
            <option key={opt.valueMs} value={opt.valueMs}>
              {intervalLabel(opt.valueMs)}
            </option>
          ))}
        </select>
      </div>

      <div className="w-px h-5 bg-border" aria-hidden />

      {/* 3. Manual Refresh Button */}
      <button
        type="button"
        onClick={handleManualRefresh}
        disabled={refreshing}
        className="flex h-10 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-md border border-border bg-surface-2 px-2.5 text-body font-medium text-text transition-colors hover:border-muted hover:bg-surface-2/80 hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:cursor-not-allowed disabled:opacity-50 lg:h-9"
        title="Fetch immediately without waiting for the next auto refresh. Keeps the same time range and interval."
      >
        <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />
        {!compact ? <span className="hidden sm:inline">Refresh now</span> : null}
      </button>
    </div>
  );
};
