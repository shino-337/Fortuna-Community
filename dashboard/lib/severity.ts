export type SeverityLevel = 'critical' | 'high' | 'medium' | 'low';

const severityStyles: Record<SeverityLevel, { badge: string; text: string; border: string }> = {
  critical: {
    badge: 'bg-red-500/10 text-red-400 border-red-500/20',
    text: 'text-red-500',
    border: 'border-l-red-500',
  },
  high: {
    badge: 'bg-orange-500/10 text-orange-400 border-orange-500/20',
    text: 'text-orange-500',
    border: 'border-l-orange-500',
  },
  medium: {
    badge: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20',
    text: 'text-yellow-500',
    border: 'border-l-yellow-500',
  },
  low: {
    badge: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
    text: 'text-blue-500',
    border: 'border-l-blue-500',
  },
};

export const getSeverityBadgeClass = (severity?: string) => {
  if (!severity) return 'bg-slate-500/10 text-slate-400 border-slate-500/20';
  const key = severity.toLowerCase() as SeverityLevel;
  return severityStyles[key]?.badge || 'bg-slate-500/10 text-slate-400 border-slate-500/20';
};

export const getSeverityTextClass = (severity?: string) => {
  if (!severity) return 'text-slate-400';
  const key = severity.toLowerCase() as SeverityLevel;
  return severityStyles[key]?.text || 'text-slate-400';
};

export const getSeverityBorderClass = (severity?: string) => {
  if (!severity) return 'border-l-slate-700';
  const key = severity.toLowerCase() as SeverityLevel;
  return severityStyles[key]?.border || 'border-l-slate-700';
};
