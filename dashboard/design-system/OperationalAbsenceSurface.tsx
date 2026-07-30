import React from 'react';
import { ShieldOff, Radar, MapPin, Ban, EyeOff, Layers } from 'lucide-react';

export type OperationalAbsence =
  | 'healthy_empty'
  | 'out_of_scope'
  | 'telemetry_unavailable'
  | 'not_operationally_relevant'
  | 'hidden_by_policy'
  | 'sampling_incomplete';

const COPY: Record<
  OperationalAbsence,
  { title: string; icon: React.ReactNode; tone: string }
> = {
  healthy_empty: {
    title: 'No records in scope',
    icon: <Radar className="w-8 h-8 text-emerald-400/80" />,
    tone: 'border-emerald-500/25 bg-emerald-500/5',
  },
  out_of_scope: {
    title: 'Outside operational scope',
    icon: <MapPin className="w-8 h-8 text-amber-400/80" />,
    tone: 'border-amber-500/25 bg-amber-500/5',
  },
  telemetry_unavailable: {
    title: 'Telemetry unavailable',
    icon: <Radar className="w-8 h-8 text-amber-400/80" />,
    tone: 'border-amber-500/25 bg-amber-500/5',
  },
  not_operationally_relevant: {
    title: 'Not part of your operational workspace',
    icon: <Ban className="w-8 h-8 text-muted" />,
    tone: 'border-border bg-surface/30',
  },
  hidden_by_policy: {
    title: 'Restricted by policy',
    icon: <EyeOff className="w-8 h-8 text-muted" />,
    tone: 'border-border bg-surface/30',
  },
  sampling_incomplete: {
    title: 'Sampled or incomplete view',
    icon: <Layers className="w-8 h-8 text-amber-400/80" />,
    tone: 'border-amber-500/25 bg-amber-500/5',
  },
};

/**
 * Render ONLY when a surface was materialized but data/state is absent.
 * NEVER use for not_operationally_relevant — those surfaces must not mount.
 */
export const OperationalAbsenceSurface: React.FC<{
  state: OperationalAbsence;
  reason?: string;
  description?: string;
  className?: string;
}> = ({ state, reason, description, className = '' }) => {
  if (state === 'not_operationally_relevant') return null;

  const meta = COPY[state];
  const message = reason ?? description;
  return (
    <div
      role="status"
      className={`rounded-xl border px-6 py-10 text-center ${meta.tone} ${className}`}
    >
      <div className="flex justify-center mb-3">{meta.icon}</div>
      <h2 className="text-section-title font-semibold text-text">{meta.title}</h2>
      {message ? <p className="mt-2 text-caption text-muted max-w-md mx-auto">{message}</p> : null}
      {state === 'hidden_by_policy' ? (
        <p className="mt-3 text-meta text-muted flex items-center justify-center gap-1">
          <ShieldOff className="w-3.5 h-3.5" />
          Server RBAC remains authoritative.
        </p>
      ) : null}
    </div>
  );
};
