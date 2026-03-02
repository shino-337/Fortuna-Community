export type SeverityLevel = 'critical' | 'high' | 'medium' | 'low';

// Enterprise severity palette (hex-true):
// Critical: #B42318, High: #F79009, Medium: #FDB022, Low: #667085
const severityStyles: Record<SeverityLevel, { badge: string; text: string; border: string; bar: string }> = {
  critical: {
    badge: 'bg-[#B42318] text-white border-[#B42318] font-semibold',
    text: 'text-[#B42318]',
    border: 'border-l-[#B42318]',
    bar: 'bg-[#B42318]',
  },
  high: {
    badge: 'bg-[#F79009] text-white border-[#F79009] font-semibold',
    text: 'text-[#F79009]',
    border: 'border-l-[#F79009]',
    bar: 'bg-[#F79009]',
  },
  medium: {
    badge: 'bg-[#FDB022] text-slate-900 border-[#FDB022] font-semibold',
    text: 'text-[#FDB022]',
    border: 'border-l-[#FDB022]',
    bar: 'bg-[#FDB022]',
  },
  low: {
    badge: 'bg-[#667085] text-white border-[#667085] font-medium',
    text: 'text-[#667085]',
    border: 'border-l-[#667085]',
    bar: 'bg-[#667085]',
  },
};

export const getSeverityBadgeClass = (severity?: string) => {
  if (!severity) return 'bg-muted-2/20 text-muted border-muted-2/30';
  const key = severity.toLowerCase() as SeverityLevel;
  return severityStyles[key]?.badge || 'bg-muted-2/20 text-muted border-muted-2/30';
};

export const getSeverityTextClass = (severity?: string) => {
  if (!severity) return 'text-muted';
  const key = severity.toLowerCase() as SeverityLevel;
  return severityStyles[key]?.text || 'text-muted';
};

export const getSeverityBorderClass = (severity?: string) => {
  if (!severity) return 'border-l-border';
  const key = severity.toLowerCase() as SeverityLevel;
  return severityStyles[key]?.border || 'border-l-border';
};

export const getSeverityBarClass = (severity?: string) => {
  if (!severity) return 'bg-slate-600';
  const key = severity.toLowerCase() as SeverityLevel;
  return severityStyles[key]?.bar || 'bg-slate-600';
};

/** Industry-standard severity icon for quick scan (Critical=red dot, etc.) */
export const getSeverityIcon = (severity?: string) => {
  if (!severity) return '—';
  const key = severity.toLowerCase() as SeverityLevel;
  const icons: Record<SeverityLevel, string> = {
    critical: '🔴',
    high: '🟠',
    medium: '🟡',
    low: '⚪',
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
