export const formatDateTime = (value?: string): string => {
  if (!value) return '-';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleString();
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
