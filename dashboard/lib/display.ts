export const formatDateTime = (value?: string): string => {
  if (!value) return '-';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleString();
};

/** Hiển thị mốc bucket 5 phút (giờ:phút, theo locale). Tooltip nên dùng formatDateTime đầy đủ. */
export const formatBucketClock = (value?: string): string => {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hour12: false });
};

/** Format startTime as relative uptime (e.g. "2h 15m" or "3d 1h"). */
export const formatUptime = (startTime?: string | null): string => {
  if (!startTime) return '—';
  const start = new Date(startTime);
  if (Number.isNaN(start.getTime())) return '—';
  const now = Date.now();
  const ms = now - start.getTime();
  if (ms < 0) return '—';
  const s = Math.floor(ms / 1000);
  const m = Math.floor(s / 60);
  const h = Math.floor(m / 60);
  const d = Math.floor(h / 24);
  if (d > 0) return `${d}d ${h % 24}h`;
  if (h > 0) return `${h}h ${m % 60}m`;
  if (m > 0) return `${m}m ${s % 60}s`;
  return `${s}s`;
};

export const getConnectionStatusLabel = (status?: string): string => {
  const s = (status || '').toLowerCase();
  if (s === 'connected') return 'Connected';
  if (s === 'degraded') return 'Degraded';
  if (s === 'disconnected') return 'Disconnected';
  return 'Unknown';
};

export const getConnectionStatusClass = (status?: string): string => {
  const s = (status || '').toLowerCase();
  if (s === 'connected') return 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20';
  if (s === 'degraded') return 'bg-yellow-500/10 text-yellow-400 border border-yellow-500/20';
  if (s === 'disconnected') return 'bg-red-500/10 text-red-400 border border-red-500/20';
  return 'bg-slate-800 text-slate-400 border border-slate-700';
};

export const getAgentStatusLabel = (status?: string): string => {
  const s = (status || '').toLowerCase();
  if (s === 'healthy') return 'Healthy';
  if (s === 'slow') return 'Slow';
  if (s === 'disconnected') return 'Disconnected';
  if (!s) return 'Unknown';
  return status as string;
};
