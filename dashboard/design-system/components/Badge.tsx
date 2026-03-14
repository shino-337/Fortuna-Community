import React from 'react';
import { getSeverityBadgeClass } from '../../lib/severity';
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
}

/**
 * Design-system Badge: severity and status labels.
 * Typography: text-[11px] font-semibold per FORTUNA_UI_DESIGN_SYSTEM.
 */
export const Badge: React.FC<BadgeProps> = ({
  children,
  variant = 'default',
  severity,
  className = '',
  uppercase = true,
}) => {
  const severityKey = severity ?? variantToSeverity[variant];
  const baseClass = getSeverityBadgeClass(severityKey);
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded border ${TYPOGRAPHY.badge} ${baseClass} ${uppercase ? 'uppercase' : ''} ${className}`}
    >
      {children}
    </span>
  );
};
