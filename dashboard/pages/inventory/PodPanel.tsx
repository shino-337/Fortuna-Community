import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/Button';
import { SidePanel } from '../../components/SidePanel';
import { api } from '../../lib/api';
import { attackPathsForPodPath, findingsForResourcePath, identityDetailPath, networkForPodPath, podDetailPath } from '../../lib/entityLinks';
import { getPodStatusBadgeClass, getSeverityBadgeClass, pathRiskLevel as pathLevel } from '../../lib/severity';
import type { AttackPath, Insight, PodWithRisk } from '../../types';
import { PanelSection, RiskBadge } from './shared';


type Load<T> = { state: 'loading' } | { state: 'ready'; data: T } | { state: 'failed' };

const linkClass = 'text-caption font-semibold text-brand hover:underline';

/** Preview of one workload: its findings, the attack paths it starts, and where to go next. */
export const PodPanel: React.FC<{
  pod: PodWithRisk | null;
  onClose: () => void;
  allowedRoutes: string[];
}> = ({ pod, onClose, allowedRoutes }) => {
  const navigate = useNavigate();
  const [findings, setFindings] = useState<Load<{ items: Insight[]; total: number }>>({ state: 'loading' });
  const [paths, setPaths] = useState<Load<AttackPath[]>>({ state: 'loading' });
  const canFindings = allowedRoutes.includes('/risks');
  const canPaths = allowedRoutes.includes('/attack-paths');
  const canNetwork = allowedRoutes.includes('/network-activity');

  useEffect(() => {
    if (!pod) return;
    let cancelled = false;
    setFindings({ state: 'loading' });
    setPaths({ state: 'loading' });
    if (canFindings) {
      api
        .getRisks({ resourceUid: pod.uid, clusterId: pod.clusterId, status: 'open', page: 1, pageSize: 3, sort: 'score', order: 'desc' })
        .then((r) => !cancelled && setFindings({ state: 'ready', data: { items: r.insights, total: r.total } }))
        .catch(() => !cancelled && setFindings({ state: 'failed' }));
    }
    if (canPaths) {
      api
        .getAttackPathsForPodStrict(pod.uid, pod.clusterId)
        .then((p) => !cancelled && setPaths({ state: 'ready', data: [...p].sort((a, b) => b.total_risk - a.total_risk) }))
        .catch(() => !cancelled && setPaths({ state: 'failed' }));
    }
    return () => {
      cancelled = true;
    };
  }, [pod, canFindings, canPaths]);

  if (!pod) return null;
  const ref = { uid: pod.uid, clusterId: pod.clusterId };
  const signals = pod.riskSignals;
  const score = pod.unifiedScore ?? pod.totalScore;

  return (
    <SidePanel
      open
      onClose={onClose}
      title={pod.name}
      subtitle={[pod.namespace, pod.clusterId, pod.nodeName].filter(Boolean).join(' · ')}
    >
      <div className="flex flex-wrap items-center gap-2">
        <RiskBadge level={pod.finalLevel} score={score} />
        {pod.phase || pod.status ? (
          <span className={`rounded px-1.5 py-0.5 text-meta ${getPodStatusBadgeClass(pod.phase || pod.status)}`}>{pod.phase || pod.status}</span>
        ) : null}
        {signals?.isEntryPoint ? <span className="rounded border border-border px-1.5 py-0.5 text-meta text-text">Entry point</span> : null}
        {signals?.isPivot ? <span className="rounded border border-border px-1.5 py-0.5 text-meta text-text">Pivot</span> : null}
      </div>

      {canFindings ? (
        <PanelSection title="Open findings" link={<Link className={linkClass} to={findingsForResourcePath(ref)}>All findings</Link>}>
          {findings.state === 'loading' ? (
            <p className="text-caption text-muted">Loading…</p>
          ) : findings.state === 'failed' ? (
            <p className="text-caption text-muted">Findings could not be loaded.</p>
          ) : findings.data.total === 0 ? (
            <p className="text-body text-muted">No open findings.</p>
          ) : (
            <ul className="divide-y divide-border/60 rounded-lg border border-border">
              {findings.data.items.map((f) => (
                <li key={f.id}>
                  <Link to={`/risks/${encodeURIComponent(f.id)}`} className="flex items-center justify-between gap-3 px-3 py-2 hover:bg-surface-2">
                    <span className="min-w-0 truncate text-body text-text">{f.title}</span>
                    {f.finalLevel ? (
                      <span className={`shrink-0 rounded border px-1.5 py-0.5 text-meta font-semibold uppercase ${getSeverityBadgeClass(f.finalLevel)}`}>{f.finalLevel}</span>
                    ) : null}
                  </Link>
                </li>
              ))}
              {findings.data.total > findings.data.items.length ? (
                <li className="px-3 py-2 text-caption text-muted">{(findings.data.total - findings.data.items.length).toLocaleString()} more</li>
              ) : null}
            </ul>
          )}
        </PanelSection>
      ) : null}

      {canPaths ? (
        <PanelSection title="Attack paths from here" link={<Link className={linkClass} to={attackPathsForPodPath(ref)}>Open in Attack Paths</Link>}>
          {paths.state === 'loading' ? (
            <p className="text-caption text-muted">Loading…</p>
          ) : paths.state === 'failed' ? (
            <p className="text-caption text-muted">Attack paths could not be loaded.</p>
          ) : paths.data.length === 0 ? (
            <p className="text-body text-muted">No attack path starts at this workload.</p>
          ) : (
            <ul className="divide-y divide-border/60 rounded-lg border border-border">
              {paths.data.slice(0, 3).map((p, i) => (
                <li key={p.path_id ?? i}>
                  <Link
                    to={attackPathsForPodPath(ref, { pathId: p.path_id })}
                    className="flex items-center justify-between gap-3 px-3 py-2 hover:bg-surface-2"
                  >
                    <span className="min-w-0 truncate text-body text-text">{p.description || `Path ${p.path_id ?? i + 1}`}</span>
                    <span className={`shrink-0 rounded border px-1.5 py-0.5 text-meta font-semibold uppercase ${getSeverityBadgeClass(pathLevel(p.total_risk))}`}>
                      {pathLevel(p.total_risk)} {p.total_risk.toFixed(1)}
                    </span>
                  </Link>
                </li>
              ))}
              {paths.data.length > 3 ? <li className="px-3 py-2 text-caption text-muted">{(paths.data.length - 3).toLocaleString()} more</li> : null}
            </ul>
          )}
        </PanelSection>
      ) : null}

      <PanelSection title="Identity and traffic">
        <div className="flex flex-wrap gap-x-4 gap-y-2 text-body">
          {pod.serviceAccount ? (
            pod.serviceAccountUid ? (
              <Link className="text-brand hover:underline" to={identityDetailPath({ uid: pod.serviceAccountUid, clusterId: pod.clusterId })}>
                Runs as {pod.serviceAccount}
              </Link>
            ) : (
              <span className="text-muted">Runs as {pod.serviceAccount}</span>
            )
          ) : null}
          {canNetwork ? (
            <Link className="text-brand hover:underline" to={networkForPodPath(ref, pod.namespace)}>
              Network flows
            </Link>
          ) : null}
        </div>
      </PanelSection>

      <div className="mt-auto flex flex-wrap gap-2 border-t border-border pt-4">
        <Button data-autofocus onClick={() => navigate(podDetailPath(pod.uid, pod.clusterId))}>
          Open pod
        </Button>
      </div>
    </SidePanel>
  );
};
