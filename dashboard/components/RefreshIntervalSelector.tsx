import React from 'react';
import { RefreshCw } from 'lucide-react';
import { useRefreshIntervalStore, REFRESH_OPTIONS } from '../store/refreshIntervalStore';

export const RefreshIntervalSelector: React.FC = () => {
  const { intervalMs, setIntervalMs } = useRefreshIntervalStore();

  return (
    <div className="flex items-center gap-2">
      <RefreshCw className="w-4 h-4 text-slate-500 shrink-0" />
      <select
        value={intervalMs}
        onChange={(e) => setIntervalMs(Number(e.target.value))}
        className="bg-slate-900/80 border border-slate-700 rounded-lg px-3 py-1.5 text-sm text-slate-200 focus:outline-none focus:ring-1 focus:ring-pink-500 focus:border-pink-500"
        title="Refresh data interval"
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
