import React from 'react';
import { Link } from 'react-router-dom';
import { AlertTriangle, ArrowRight, Radio, Users } from 'lucide-react';
import { useIncidentMode } from '../../hooks/useIncidentMode';
import { useOperationalContextGraph } from '../../hooks/useOperationalContextGraph';

type BannerTone = 'critical' | 'warning';

interface BannerContent {
  tone: BannerTone;
  icon: React.ReactNode;
  title: string;
  detail?: string;
  action?: { label: string; to: string };
}

const TONE_CLASS: Record<BannerTone, string> = {
  critical: 'border-red-500/35 bg-red-500/10 text-red-100',
  warning: 'border-amber-500/30 bg-amber-500/10 text-amber-100',
};

/**
 * The shell's single system banner. Shows only the most important condition:
 * an active incident, then several incidents competing for attention, then
 * telemetry that leaves data incomplete. Pages never render their own copy.
 */
export const ShellBanner: React.FC<{ allowedRoutes: string[] }> = ({ allowedRoutes }) => {
  const incident = useIncidentMode();
  const { multiIncident, telemetryHealth } = useOperationalContextGraph();
  const canOpen = (path: string) => allowedRoutes.includes(path);

  let content: BannerContent | null = null;
  if (incident.active) {
    const caseTitle = incident.context.activeCaseTitle;
    content = {
      tone: 'critical',
      icon: <AlertTriangle className="h-4 w-4 shrink-0" aria-hidden />,
      title: incident.context.escalated ? `${incident.label} (escalated)` : incident.label,
      detail: caseTitle || undefined,
      action: canOpen('/investigation')
        ? {
            label: 'Open case',
            to: incident.context.activeCaseId ? `/investigation?case=${incident.context.activeCaseId}` : '/investigation',
          }
        : undefined,
    };
  } else if (multiIncident.level === 'high' || multiIncident.level === 'critical') {
    content = {
      tone: multiIncident.level === 'critical' ? 'critical' : 'warning',
      icon: <Users className="h-4 w-4 shrink-0" aria-hidden />,
      title: `${multiIncident.competingCases.length || 'Several'} investigations need attention at the same time`,
      detail: multiIncident.resourceContentionMessage || undefined,
      action: canOpen('/investigation') ? { label: 'Open investigations', to: '/investigation' } : undefined,
    };
  } else if (telemetryHealth.overallDegraded) {
    const lagging = telemetryHealth.pipelines.filter((p) => p.healthy === false).map((p) => p.label);
    content = {
      tone: 'warning',
      icon: <Radio className="h-4 w-4 shrink-0" aria-hidden />,
      title: 'Some data may be incomplete',
      detail: lagging.length > 0 ? `Behind: ${lagging.join(', ')}.` : undefined,
      action: canOpen('/monitoring') ? { label: 'Open Platform', to: '/monitoring' } : undefined,
    };
  }

  if (!content) return null;

  return (
    <div
      className={`mx-4 mt-4 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border px-3 py-2 text-caption sm:mx-6 ${TONE_CLASS[content.tone]}`}
      role="status"
      aria-live="polite"
    >
      {content.icon}
      <span className="font-semibold">{content.title}</span>
      {content.detail ? <span className="min-w-0 flex-1 truncate text-muted">{content.detail}</span> : <span className="flex-1" />}
      {content.action ? (
        <Link to={content.action.to} className="inline-flex shrink-0 items-center gap-1 font-semibold text-brand hover:text-brand/90">
          {content.action.label} <ArrowRight className="h-3.5 w-3.5" aria-hidden />
        </Link>
      ) : null}
    </div>
  );
};
