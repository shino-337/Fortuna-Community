import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronRight, Network, Route, ShieldAlert, UserRound } from 'lucide-react';
import clsx from 'clsx';
import { api } from '../lib/api';
import { attackPathsForPodPath, findingsForResourcePath, networkForPodPath } from '../lib/entityLinks';
import { getSeverityBadgeClass, pathRiskLevel } from '../lib/severity';
import type { AttackPath, PodWithRisk } from '../types';

interface StepProps {
  icon: React.ReactNode;
  label: string;
  value: React.ReactNode;
  to?: string | null;
  onClick?: () => void;
}

function Step({ icon, label, value, to, onClick }: StepProps) {
  const body = (
    <>
      <span className="shrink-0 text-muted" aria-hidden>
        {icon}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-meta font-semibold uppercase tracking-wider text-muted">{label}</span>
        <span className="block truncate text-body font-medium text-text">{value}</span>
      </span>
      {to || onClick ? <ChevronRight className="h-4 w-4 shrink-0 text-muted" aria-hidden /> : null}
    </>
  );
  const cls = 'flex min-h-14 min-w-0 items-center gap-3 rounded-lg border border-border bg-surface/40 px-3 py-2';
  if (to) {
    return (
      <Link to={to} className={clsx(cls, 'hover:border-brand')}>
        {body}
      </Link>
    );
  }
  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={clsx(cls, 'text-left hover:border-brand')}>
        {body}
      </button>
    );
  }
  return <div className={cls}>{body}</div>;
}

/** Where to go from a pod: its findings, the attack paths it starts, its traffic and its identity. */
export const PodNextSteps: React.FC<{
  pod: PodWithRisk;
  allowedRoutes: string[];
  serviceAccountName: string;
  onOpenIdentity?: () => void;
}> = ({ pod, allowedRoutes, serviceAccountName, onOpenIdentity }) => {
  const ref = { uid: pod.uid, clusterId: pod.clusterId ? String(pod.clusterId) : null };
  const paths = pod.riskSignals?.pathCount ?? 0;
  const role = pod.riskSignals?.isEntryPoint ? 'entry point' : pod.riskSignals?.isPivot ? 'pivot' : '';
  return (
    <nav aria-label="Next steps" className="mb-6 grid min-w-0 grid-cols-1 gap-2 min-[480px]:grid-cols-2 xl:grid-cols-4">
      <Step
        icon={<ShieldAlert className="h-5 w-5" />}
        label="Findings"
        value={pod.riskCount > 0 ? `${pod.riskCount.toLocaleString()} open` : 'None open'}
        to={allowedRoutes.includes('/risks') ? findingsForResourcePath(ref) : null}
      />
      <Step
        icon={<Route className="h-5 w-5" />}
        label="Attack paths"
        value={pod.riskSignals ? `${paths.toLocaleString()} from here${role ? ` · ${role}` : ''}` : 'Not analysed'}
        to={allowedRoutes.includes('/attack-paths') && paths > 0 ? attackPathsForPodPath(ref) : null}
      />
      <Step
        icon={<Network className="h-5 w-5" />}
        label="Network"
        value="Flows to and from this pod"
        to={allowedRoutes.includes('/network-activity') ? networkForPodPath(ref, pod.namespace || undefined) : null}
      />
      <Step icon={<UserRound className="h-5 w-5" />} label="Runs as" value={serviceAccountName || '—'} onClick={onOpenIdentity} />
    </nav>
  );
};

/** The attack paths that start from or pass through this pod, strongest first. */
export const PodAttackPaths: React.FC<{ pod: PodWithRisk; canOpen: boolean }> = ({ pod, canOpen }) => {
  const [state, setState] = useState<{ status: 'loading' } | { status: 'ready'; paths: AttackPath[] } | { status: 'failed' }>({ status: 'loading' });
  const clusterId = pod.clusterId ? String(pod.clusterId) : undefined;

  useEffect(() => {
    let cancelled = false;
    setState({ status: 'loading' });
    api
      .getAttackPathsForPodStrict(pod.uid, clusterId)
      .then((p) => !cancelled && setState({ status: 'ready', paths: [...p].sort((a, b) => b.total_risk - a.total_risk) }))
      .catch(() => !cancelled && setState({ status: 'failed' }));
    return () => {
      cancelled = true;
    };
  }, [pod.uid, clusterId]);

  if (state.status === 'loading') return <p className="text-body text-muted">Loading…</p>;
  if (state.status === 'failed') return <p className="text-body text-amber-300">Attack paths could not be loaded.</p>;
  if (state.paths.length === 0) return <p className="text-body text-muted">No attack path starts from or passes through this pod.</p>;

  const ref = { uid: pod.uid, clusterId: clusterId ?? null };
  return (
    <ul className="divide-y divide-border/60 rounded-lg border border-border">
      {state.paths.map((p) => {
        const level = pathRiskLevel(p.total_risk);
        const row = (
          <>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-body text-text">{p.description || p.path_id}</span>
              <span className="text-caption text-muted">{p.length ? `${p.length} steps` : null}</span>
            </span>
            <span className={clsx('shrink-0 rounded px-1.5 py-0.5 text-meta font-semibold uppercase', getSeverityBadgeClass(level))}>
              {level} {p.total_risk.toFixed(1)}
            </span>
          </>
        );
        return (
          <li key={p.path_id}>
            {canOpen ? (
              <Link to={attackPathsForPodPath(ref, { pathId: p.path_id })} className="flex items-center gap-3 px-3 py-2.5 hover:bg-surface-2">
                {row}
              </Link>
            ) : (
              <div className="flex items-center gap-3 px-3 py-2.5">{row}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
};
