import React, { useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { useTimeWindowStore, TIME_WINDOW_OPTIONS } from '../store/timeWindowStore';
import { useRefreshIntervalStore, REFRESH_OPTIONS } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';

const DEFAULT_INTERVAL_MS = 5 * 60 * 1000;

/**
 * Control bar for time-based data: Time Range (what window), Auto Refresh (how often), Manual Refresh (fetch now).
 * Time range = phạm vi dữ liệu; Refresh interval = tần suất re-fetch; Refresh button = fetch ngay.
 */
export const DataControlBar: React.FC = () => {
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
      className="flex flex-wrap items-center gap-2 sm:gap-3 px-2 sm:px-3 py-2 bg-slate-900/80 border border-slate-800 rounded-lg min-w-0 max-w-full"
      title="Time range = phạm vi dữ liệu; Refresh interval = tần suất re-fetch; Nút Refresh = fetch ngay"
    >
      {/* 1. Time Range Selector – Last 1m / 5m / ... */}
      <div className="flex items-center gap-2">
        <label htmlFor="data-control-time-range" className="text-xs text-slate-500 whitespace-nowrap shrink-0">
          <span className="hidden min-[1100px]:inline">Time range:</span>
          <span className="min-[1100px]:hidden">Range:</span>
        </label>
        <select
          id="data-control-time-range"
          value={timeWindowMinutes}
          onChange={(e) => setTimeWindowMinutes(Number(e.target.value))}
          className="bg-slate-800 border border-slate-700 rounded-md px-2.5 py-1.5 text-sm text-slate-200 focus:outline-none focus:ring-1 focus:ring-pink-500 focus:border-pink-500 min-w-[100px]"
          title="Phạm vi dữ liệu (from = now - Xm, to = now). Không ảnh hưởng tần suất refresh."
        >
          {TIME_WINDOW_OPTIONS.map((opt) => (
            <option key={opt.valueMinutes} value={opt.valueMinutes}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>

      <div className="w-px h-5 bg-slate-700" aria-hidden />

      {/* 2. Auto Refresh Interval – Every 1m / 5m / ... */}
      <div className="flex items-center gap-2">
        <label htmlFor="data-control-refresh-interval" className="text-xs text-slate-500 whitespace-nowrap shrink-0">
          <span className="hidden min-[1100px]:inline">Auto refresh:</span>
          <span className="min-[1100px]:hidden">Auto:</span>
        </label>
        <select
          id="data-control-refresh-interval"
          value={displayIntervalMs}
          onChange={(e) => setIntervalMs(Number(e.target.value))}
          className="bg-slate-800 border border-slate-700 rounded-md px-2.5 py-1.5 text-sm text-slate-200 focus:outline-none focus:ring-1 focus:ring-pink-500 focus:border-pink-500 min-w-[100px]"
          title="Chu kỳ tự động re-fetch. Giữ nguyên time range, now cập nhật → dữ liệu trượt theo thời gian thực."
        >
          {REFRESH_OPTIONS.map((opt) => (
            <option key={opt.valueMs} value={opt.valueMs}>
              {intervalLabel(opt.valueMs)}
            </option>
          ))}
        </select>
      </div>

      <div className="w-px h-5 bg-slate-700" aria-hidden />

      {/* 3. Manual Refresh Button */}
      <button
        type="button"
        onClick={handleManualRefresh}
        disabled={refreshing}
        className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-sm font-medium bg-slate-800 border border-slate-700 text-slate-300 hover:bg-slate-700 hover:text-white hover:border-slate-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        title="Refresh ngay lập tức, không chờ auto refresh. Giữ nguyên time range và interval."
      >
        <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />
        <span className="hidden sm:inline">Refresh now</span>
      </button>
    </div>
  );
};
