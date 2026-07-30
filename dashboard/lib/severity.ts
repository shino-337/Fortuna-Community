export type SeverityLevel = 'critical' | 'high' | 'medium' | 'low';

/** ADR unified bands: same as backend pkg/risk.DeriveFinalLevelFromScore */
export const deriveUnifiedRiskLevelFromScore = (score?: number): SeverityLevel | undefined => {
  if (score == null || Number.isNaN(score)) return undefined;
  const s = Math.round(score);
  if (s >= 70) return 'critical';
  if (s >= 40) return 'high';
  if (s >= 20) return 'medium';
  return 'low';
};

const normalizeSeverity = (severity?: string): SeverityLevel | undefined => {
  if (!severity) return undefined;
  const s = severity.toLowerCase();
  return (s === 'very high' || s === 'veryhigh' ? 'critical' : s) as SeverityLevel;
};

// Risk level colors paired with shape cues: Critical=triangle, High=diamond, Medium=filled circle, Low=hollow circle.
const severityStyles: Record<SeverityLevel, { badge: string; text: string; border: string; bar: string; icon: string; shapeLabel: string }> = {
  critical: {
    badge: 'bg-error text-white border-error font-semibold',
    text: 'text-error',
    border: 'border-error/60',
    bar: 'bg-error',
    icon: '▲',
    shapeLabel: 'triangle',
  },
  high: {
    badge: 'bg-orange-500 text-white border-orange-500 font-semibold',
    text: 'text-orange-500',
    border: 'border-orange-500/60',
    bar: 'bg-orange-500',
    icon: '◆',
    shapeLabel: 'diamond',
  },
  medium: {
    badge: 'bg-yellow-400 text-yellow-950 border-yellow-400 font-semibold',
    text: 'text-yellow-400',
    border: 'border-yellow-400/60',
    bar: 'bg-yellow-400',
    icon: '●',
    shapeLabel: 'filled circle',
  },
  low: {
    badge: 'bg-info text-white border-info font-medium',
    text: 'text-info',
    border: 'border-info/60',
    bar: 'bg-info',
    icon: '○',
    shapeLabel: 'hollow circle',
  },
};

/** Get Tailwind badge class for a severity level. */
export const getSeverityBadgeClass = (severity?: string) => {
  const key = normalizeSeverity(severity);
  return key && severityStyles[key] ? severityStyles[key].badge : 'bg-muted-2/20 text-muted border-muted-2/30';
};

/** Get Tailwind text class for a severity level. */
export const getSeverityTextClass = (severity?: string) => {
  const key = normalizeSeverity(severity);
  return key && severityStyles[key] ? severityStyles[key].text : 'text-muted';
};

/** Get Tailwind border class for a severity level. */
export const getSeverityBorderClass = (severity?: string) => {
  const key = normalizeSeverity(severity);
  return key && severityStyles[key] ? severityStyles[key].border : 'border-border';
};

/** Get Tailwind bar background class for a severity level. */
export const getSeverityBarClass = (severity?: string) => {
  const key = normalizeSeverity(severity);
  return key && severityStyles[key] ? severityStyles[key].bar : 'bg-muted-2';
};

/** Get icon character for a severity level. */
export const getSeverityIcon = (severity?: string) => {
  const key = normalizeSeverity(severity);
  return key && severityStyles[key] ? severityStyles[key].icon : '·';
};

/** Get shape label text for a severity level. */
export const getSeverityShapeLabel = (severity?: string) => {
  const key = normalizeSeverity(severity);
  return key && severityStyles[key] ? severityStyles[key].shapeLabel : 'dot';
};

/** Pod phase (Kubernetes status) badge: Running=green, Pending=amber, Succeeded=slate, Failed=red, Unknown=grey */
export const getPodStatusBadgeClass = (phase?: string) => {
  if (!phase || !phase.trim()) return 'border border-border text-muted bg-surface-2/50';
  const p = phase.trim().toLowerCase();
  if (p === 'running') return 'text-success bg-success/10 border-success/20';
  if (p === 'pending') return 'text-amber-400 bg-amber-500/10 border-amber-500/20';
  if (p === 'succeeded') return 'text-text bg-muted/10 border-border/40';
  if (p === 'failed') return 'text-error bg-error/10 border-error/20';
  return 'border border-border text-muted bg-surface-2/50';
};
