export type SeverityLevel = 'critical' | 'high' | 'medium' | 'low';

const severityStyles: Record<SeverityLevel, { badge: string; text: string; border: string }> = {
  critical: {
    badge: 'bg-critical/10 text-critical border-critical/20',
    text: 'text-critical',
    border: 'border-l-critical',
  },
  high: {
    badge: 'bg-high/10 text-high border-high/20',
    text: 'text-high',
    border: 'border-l-high',
  },
  medium: {
    badge: 'bg-medium/10 text-medium border-medium/20',
    text: 'text-medium',
    border: 'border-l-medium',
  },
  low: {
    badge: 'bg-low/10 text-low border-low/20',
    text: 'text-low',
    border: 'border-l-low',
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
