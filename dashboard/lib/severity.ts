export type SeverityLevel = 'critical' | 'high' | 'medium' | 'low';

// Risk level colors (Very High=red, High=orange, Medium=yellow, Low=green)
const severityStyles: Record<SeverityLevel, { badge: string; text: string; border: string; bar: string }> = {
  critical: {
    badge: 'bg-red-600 text-white border-red-600 font-semibold',
    text: 'text-red-500',
    border: 'border-l-red-600',
    bar: 'bg-red-600',
  },
  high: {
    badge: 'bg-orange-500 text-white border-orange-500 font-semibold',
    text: 'text-orange-500',
    border: 'border-l-orange-500',
    bar: 'bg-orange-500',
  },
  medium: {
    badge: 'bg-yellow-500 text-slate-900 border-yellow-500 font-semibold',
    text: 'text-yellow-500',
    border: 'border-l-yellow-500',
    bar: 'bg-yellow-500',
  },
  low: {
    badge: 'bg-green-500 text-white border-green-500 font-medium',
    text: 'text-green-500',
    border: 'border-l-green-500',
    bar: 'bg-green-500',
  },
};

export const getSeverityBadgeClass = (severity?: string) => {
  if (!severity) return 'bg-muted-2/20 text-muted border-muted-2/30';
  const s = severity.toLowerCase();
  const key = (s === 'very high' || s === 'veryhigh' ? 'critical' : s) as SeverityLevel;
  return severityStyles[key]?.badge || 'bg-muted-2/20 text-muted border-muted-2/30';
};

export const getSeverityTextClass = (severity?: string) => {
  if (!severity) return 'text-muted';
  const s = severity.toLowerCase();
  const key = (s === 'very high' || s === 'veryhigh' ? 'critical' : s) as SeverityLevel;
  return severityStyles[key]?.text || 'text-muted';
};

export const getSeverityBorderClass = (severity?: string) => {
  if (!severity) return 'border-l-border';
  const s = severity.toLowerCase();
  const key = (s === 'very high' || s === 'veryhigh' ? 'critical' : s) as SeverityLevel;
  return severityStyles[key]?.border || 'border-l-border';
};

export const getSeverityBarClass = (severity?: string) => {
  if (!severity) return 'bg-slate-600';
  const s = severity.toLowerCase();
  const key = (s === 'very high' || s === 'veryhigh' ? 'critical' : s) as SeverityLevel;
  return severityStyles[key]?.bar || 'bg-slate-600';
};

/** Industry-standard severity icon (Very High=red, High=orange, Medium=yellow, Low=green) */
export const getSeverityIcon = (severity?: string) => {
  if (!severity) return '—';
  const s = severity.toLowerCase();
  const key = (s === 'very high' || s === 'veryhigh' ? 'critical' : s) as SeverityLevel;
  const icons: Record<SeverityLevel, string> = {
    critical: '🔴',
    high: '🟠',
    medium: '🟡',
    low: '🟢',
  };
  return icons[key] ?? '—';
};

/** Pod phase (Kubernetes status) badge: Running=green, Pending=amber, Succeeded=slate, Failed=red, Unknown=grey */
export const getPodStatusBadgeClass = (phase?: string) => {
  if (!phase || !phase.trim()) return 'border border-slate-700 text-slate-400 bg-slate-800/50';
  const p = phase.trim().toLowerCase();
  if (p === 'running') return 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20';
  if (p === 'pending') return 'text-amber-400 bg-amber-500/10 border-amber-500/20';
  if (p === 'succeeded') return 'text-slate-300 bg-slate-500/10 border-slate-500/20';
  if (p === 'failed') return 'text-red-400 bg-red-500/10 border-red-500/20';
  return 'border border-slate-700 text-slate-400 bg-slate-800/50';
};
