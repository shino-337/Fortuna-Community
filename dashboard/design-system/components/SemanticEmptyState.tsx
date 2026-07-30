import React from 'react';
import clsx from 'clsx';
import {
  Inbox,
  Lock,
  MapPinOff,
  Radio,
  ShieldAlert,
  Clock,
  EyeOff,
  BarChart2,
} from 'lucide-react';
import type { SemanticVisibilityState } from '../../lib/visibilityEngine';

const COPY: Record<
  Exclude<SemanticVisibilityState, 'visible'>,
  { title: string; description: string; icon: React.ReactNode }
> = {
  no_permission: {
    title: 'Not available for your role',
    description:
      'Your account does not include permission to use this workspace. Contact a platform administrator if you need access.',
    icon: <Lock className="w-10 h-10 text-muted-2" aria-hidden />,
  },
  no_scope: {
    title: 'Nothing in your operational scope',
    description:
      'You may not be assigned to monitored clusters, or the selected scope is outside your ownership. Adjust the cluster selector or request scope from an admin.',
    icon: <MapPinOff className="w-10 h-10 text-amber-400/80" aria-hidden />,
  },
  no_telemetry: {
    title: 'Telemetry not available',
    description:
      'Agents are not reporting for this scope. Runtime and network views require healthy ingestion before they can be trusted.',
    icon: <Radio className="w-10 h-10 text-sky-400/80" aria-hidden />,
  },
  no_data: {
    title: 'No records in this scope',
    description:
      'There are genuinely no matching items for your current filters and time window — not a visibility restriction.',
    icon: <Inbox className="w-10 h-10 text-muted-2" aria-hidden />,
  },
  sampled: {
    title: 'Sampled view',
    description:
      'This visualization shows a sampled or summarized subset. Some relationships may be omitted for performance — not because they do not exist.',
    icon: <BarChart2 className="w-10 h-10 text-brand/80" aria-hidden />,
  },
  policy_hidden: {
    title: 'Partial visibility',
    description:
      'Some data or graph relationships are hidden by organizational policy or query permissions. Do not treat this view as complete blast radius.',
    icon: <EyeOff className="w-10 h-10 text-violet-400/80" aria-hidden />,
  },
  stale: {
    title: 'Data may be stale',
    description:
      'Ingestion or scoring freshness is degraded. Metrics and paths may be incomplete until telemetry recovers.',
    icon: <Clock className="w-10 h-10 text-amber-400/80" aria-hidden />,
  },
};

export interface SemanticEmptyStateProps {
  state: SemanticVisibilityState;
  /** Override registry copy when engine provides a specific reason. */
  reason?: string;
  title?: string;
  className?: string;
  action?: React.ReactNode;
  compact?: boolean;
}

export const SemanticEmptyState: React.FC<SemanticEmptyStateProps> = ({
  state,
  reason,
  title,
  className = '',
  action,
  compact = false,
}) => {
  if (state === 'visible') return null;

  const preset = COPY[state];
  const displayTitle = title ?? preset.title;
  const description = reason ?? preset.description;

  return (
    <div
      className={clsx(
        'flex flex-col items-center justify-center text-center px-4',
        compact ? 'py-6 gap-2' : 'py-10 gap-3',
        className,
      )}
      role="status"
      aria-live="polite"
    >
      {preset.icon}
      <p className={clsx('font-semibold text-text', compact ? 'text-caption' : 'text-body')}>{displayTitle}</p>
      <p className={clsx('max-w-md text-muted', compact ? 'text-meta' : 'text-caption')}>{description}</p>
      {state === 'policy_hidden' || state === 'sampled' ? (
        <p className="text-meta text-muted-2 flex items-center gap-1">
          <ShieldAlert className="w-3.5 h-3.5 shrink-0" aria-hidden />
          Graph and metrics may not reflect full operational truth.
        </p>
      ) : null}
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
};
