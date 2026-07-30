import React from 'react';
import { getSeverityBadgeClass, getSeverityIcon, getSeverityShapeLabel } from '../../lib/severity';
import { TYPOGRAPHY } from '../tokens/typography';

export type BadgeVariant = 'critical' | 'high' | 'medium' | 'low' | 'info' | 'default';

const variantToSeverity: Record<BadgeVariant, string> = {
  critical: 'critical',
  high: 'high',
  medium: 'medium',
  low: 'low',
  info: 'low',
  default: 'medium',
};

interface BadgeProps {
  children: React.ReactNode;
  /** Severity variant (maps to design-system colors). Use severity for risk/CVE. */
  variant?: BadgeVariant;
  /** If set, overrides variant and uses severity utility class from lib/severity (e.g. "critical", "high"). */
  severity?: string;
  className?: string;
  /** Uppercase label; default true for severity badges. */
  uppercase?: boolean;
  /** Severity badges show a non-color shape cue by default. */
  showShape?: boolean;
}

/**
 * Design-system Badge: severity and status labels.
 * Typography: text-caption font-semibold (legibility on dark UI).
 */
export const Badge: React.FC<BadgeProps> = ({
  children,
  variant = 'default',
  severity,
  className = '',
  uppercase = true,
  showShape = true,
}) => {
  const severityKey = severity ?? variantToSeverity[variant];
  const baseClass = getSeverityBadgeClass(severityKey);
  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-0.5 rounded border ${TYPOGRAPHY.badge} ${baseClass} ${uppercase ? 'uppercase' : ''} ${className}`}
      title={showShape ? `${severityKey}: ${getSeverityShapeLabel(severityKey)} marker` : undefined}
    >
      {showShape ? <span aria-hidden="true">{getSeverityIcon(severityKey)}</span> : null}
      {children}
    </span>
  );
};
