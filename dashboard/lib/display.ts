/** Format a date/time value as locale string. */
export const formatDateTime = (value?: string): string => {
  if (!value) return '-';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleString();
};

/** 5-minute bucket clock (hour:minute, locale). Prefer formatDateTime in tooltips for full timestamps. */
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

/** Get human-readable label for a connection status. */
export const getConnectionStatusLabel = (status?: string): string => {
  const s = (status || '').toLowerCase();
  if (s === 'connected') return 'Connected';
  if (s === 'degraded') return 'Degraded';
  if (s === 'disconnected') return 'Disconnected';
  return 'Unknown';
};

/** Get Tailwind class string for a connection status. */
export const getConnectionStatusClass = (status?: string): string => {
  const s = (status || '').toLowerCase();
  if (s === 'connected') return 'bg-success/10 text-success border border-success/20';
  if (s === 'degraded') return 'bg-warning/10 text-warning border border-warning/20';
  if (s === 'disconnected') return 'bg-critical/10 text-critical border border-critical/20';
  return 'bg-surface-2 text-muted border border-border';
};

/** Get human-readable label for an agent status. */
export const getAgentStatusLabel = (status?: string): string => {
  const s = (status || '').toLowerCase();
  if (s === 'healthy') return 'Healthy';
  if (s === 'slow') return 'Slow';
  if (s === 'disconnected') return 'Disconnected';
  if (!s) return 'Unknown';
  return status as string;
};
