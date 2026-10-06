import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/Button';
import { SidePanel } from '../../components/SidePanel';
import { AddToCaseButton } from '../../components/AddToCaseButton';
import { api } from '../../lib/api';
import { attackPathsForPodPath, findingsForResourcePath, podDetailPath } from '../../lib/entityLinks';
import type { NetworkActivityConnectionRow, NetworkActivityDestinationRow } from '../../types';
import { PanelSection, RiskBadge } from '../inventory/shared';
import { destKind, destLabel, flagFlow, relativeTime, type FlowRow, type PodFacts } from './flows';

export type FlowSelection = { kind: 'flow'; row: FlowRow } | { kind: 'dest'; row: NetworkActivityDestinationRow };

type Load<T> = { state: 'loading' } | { state: 'ready'; data: T } | { state: 'failed' };

const KIND_TAG: Record<string, string> = {
  service: 'Service',
  pod: 'Pod',
  internal: 'Internal IP',
  external: 'External',
};

const linkClass = 'text-caption font-semibold text-brand hover:underline';

function KindTag({ kind }: { kind: string }) {
  return (
    <span
      className={`rounded border px-1.5 py-0.5 text-meta ${kind === 'external' ? 'border-amber-500/50 text-amber-300' : 'border-border text-muted'}`}
    >
      {KIND_TAG[kind] ?? kind}
    </span>
  );
}

function Fact({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-3 py-1 text-body">
      <dt className="text-muted">{label}</dt>
      <dd className="min-w-0 truncate text-right text-text">{children}</dd>
    </div>
  );
}

/** A flow or a destination: who is on each end, how often, and where to go next. */
export const FlowPanel: React.FC<{
  selection: FlowSelection | null;
  clusterId: string;
  sinceMinutes: number;
  pods: Record<string, PodFacts>;
  allowedRoutes: string[];
  onClose: () => void;
  onShowDestination: (destIp: string) => void;
}> = ({ selection, clusterId, sinceMinutes, pods, allowedRoutes, onClose, onShowDestination }) => {
  const navigate = useNavigate();
  const [sockets, setSockets] = useState<Load<NetworkActivityConnectionRow[]>>({ state: 'loading' });
  const [sources, setSources] = useState<Load<FlowRow[]>>({ state: 'loading' });

  useEffect(() => {
    if (!selection) return;
    let cancelled = false;
    const destIp = selection.row.destIp ?? '';
    if (selection.kind === 'flow') {
      setSockets({ state: 'loading' });
      api
        .getNetworkActivity({ cluster: clusterId, view: 'connections', podUid: selection.row.podUid, q: destIp, sinceMinutes, page: 1, pageSize: 10 })
        .then((r) => {
          if (cancelled) return;
          const port = Number(selection.row.destPort);
          const rows = (r.items as NetworkActivityConnectionRow[]).filter((x) => x.destIp === destIp && Number(x.destPort) === port);
          setSockets({ state: 'ready', data: rows });
        })
        .catch(() => !cancelled && setSockets({ state: 'failed' }));
    } else {
      setSources({ state: 'loading' });
      api
        .getNetworkActivity({ cluster: clusterId, view: 'edges', q: destIp, sinceMinutes, page: 1, pageSize: 20 })
        .then((r) => {
          if (cancelled) return;
          const port = Number(selection.row.destPort);
          setSources({ state: 'ready', data: (r.items as FlowRow[]).filter((x) => x.destIp === destIp && Number(x.destPort) === port) });
        })
        .catch(() => !cancelled && setSources({ state: 'failed' }));
    }
    return () => {
      cancelled = true;
    };
  }, [selection, clusterId, sinceMinutes]);

  if (!selection) return null;
  const d = selection.row;
  const dest = destLabel({ destIp: d.destIp ?? '', destServiceName: d.destServiceName, destServiceNamespace: d.destServiceNamespace, destWorkloadName: d.destWorkloadName, destWorkloadNamespace: d.destWorkloadNamespace });
  const kind = destKind({ destIp: d.destIp ?? '', destServiceName: d.destServiceName, destWorkloadName: d.destWorkloadName });
  const portLine = `${d.destPort ?? '—'}/${String(d.protocol ?? '').toUpperCase() || '—'}`;
  const canFindings = allowedRoutes.includes('/risks');
  const canPaths = allowedRoutes.includes('/attack-paths');

  if (selection.kind === 'dest') {
    const row = selection.row;
    return (
      <SidePanel open onClose={onClose} title={`${dest.name}:${row.destPort}`} subtitle={[KIND_TAG[kind], dest.detail].filter(Boolean).join(' · ')}>
        <dl className="divide-y divide-border/60">
          <Fact label="Pods talking to it">{row.distinctPodCount.toLocaleString()}</Fact>
          <Fact label="Observations">{row.observationCount.toLocaleString()}</Fact>
          <Fact label="Last seen">{relativeTime(row.lastObservedAt)}</Fact>
          <Fact label="Protocol">{portLine}</Fact>
        </dl>
        <PanelSection title="Talked to by">
          {sources.state === 'loading' ? (
            <p className="text-caption text-muted">Loading…</p>
          ) : sources.state === 'failed' ? (
            <p className="text-caption text-muted">Sources could not be loaded.</p>
          ) : sources.data.length === 0 ? (
            <p className="text-body text-muted">No pod in this time range.</p>
          ) : (
            <ul className="divide-y divide-border/60 rounded-lg border border-border">
              {sources.data.map((s) => {
                const facts = s.podUid ? pods[s.podUid] : undefined;
                return (
                  <li key={s.podUid}>
                    <Link to={podDetailPath(s.podUid ?? '', clusterId)} className="flex items-center justify-between gap-3 px-3 py-2 hover:bg-surface-2">
                      <span className="min-w-0 truncate text-body text-text">
                        {s.podName || facts?.name || s.podUid}
                        <span className="ml-1 text-caption text-muted">{s.namespace}</span>
                      </span>
                      <span className="shrink-0 text-caption tabular-nums text-muted">{Number(s.observationCount ?? 0).toLocaleString()}×</span>
                    </Link>
                  </li>
                );
              })}
            </ul>
          )}
        </PanelSection>
        <div className="mt-auto flex flex-wrap gap-2 border-t border-border pt-4">
          <Button data-autofocus onClick={() => onShowDestination(row.destIp)}>
            Show these flows
          </Button>
        </div>
      </SidePanel>
    );
  }

  const row = selection.row;
  const facts = row.podUid ? pods[row.podUid] : undefined;
  const podName = row.podName || facts?.name || row.podUid || 'pod';
  const ref = { uid: row.podUid ?? '', clusterId };
  const flag = flagFlow(row, facts);

  return (
    <SidePanel open onClose={onClose} title={`${podName} → ${dest.name}:${row.destPort ?? ''}`} subtitle={[row.namespace, portLine].filter(Boolean).join(' · ')}>
      {flag ? (
        <p className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-body text-amber-200">Flagged: {flag.reasons.join(', ')}.</p>
      ) : null}

      <PanelSection title="Source">
        <div className="flex flex-wrap items-center gap-2">
          <Link to={podDetailPath(ref.uid, clusterId)} className="text-body font-medium text-text hover:text-brand hover:underline">
            {podName}
          </Link>
          {facts ? <RiskBadge level={facts.finalLevel} score={facts.score} /> : null}
        </div>
        <p className="text-caption text-muted">{[row.namespace, row.ownerKind && row.ownerName ? `${row.ownerKind} ${row.ownerName}` : '', row.nodeName].filter(Boolean).join(' · ')}</p>
        <div className="flex flex-wrap gap-4">
          {canFindings ? (
            <Link className={linkClass} to={findingsForResourcePath(ref)}>
              {facts && facts.riskCount > 0 ? `${facts.riskCount.toLocaleString()} open ${facts.riskCount === 1 ? 'finding' : 'findings'}` : 'Findings'}
            </Link>
          ) : null}
          {canPaths ? (
            <Link className={linkClass} to={attackPathsForPodPath(ref)}>
              Attack paths
            </Link>
          ) : null}
        </div>
      </PanelSection>

      <PanelSection title="Destination">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-body font-medium text-text">{dest.name}</span>
          <KindTag kind={kind} />
        </div>
        {dest.detail ? <p className="text-caption text-muted">{dest.detail}</p> : null}
        <button type="button" className={`${linkClass} w-fit`} onClick={() => onShowDestination(row.destIp ?? '')}>
          Every pod that talks to it
        </button>
      </PanelSection>

      <PanelSection title="Evidence">
        <dl className="divide-y divide-border/60">
          <Fact label="Observations">{Number(row.observationCount ?? 0).toLocaleString()}</Fact>
          <Fact label="Last seen">{relativeTime(row.lastObservedAt ?? row.observedAt)}</Fact>
        </dl>
        {sockets.state === 'loading' ? (
          <p className="text-caption text-muted">Loading sockets…</p>
        ) : sockets.state === 'failed' ? (
          <p className="text-caption text-muted">Sockets could not be loaded.</p>
        ) : sockets.data.length === 0 ? null : (
          <details className="rounded-lg border border-border">
            <summary className="cursor-pointer select-none px-3 py-2 text-caption font-semibold text-muted hover:text-text">
              {sockets.data.length === 1 ? 'Latest socket' : `Latest ${sockets.data.length} sockets`}
            </summary>
            <ul className="divide-y divide-border/60 border-t border-border font-mono text-caption">
              {sockets.data.map((s, i) => (
                <li key={s.id ?? i} className="flex flex-wrap justify-between gap-x-3 px-3 py-1.5">
                  <span className="text-text">
                    {s.sourceIp}:{s.sourcePort} → {s.destIp}:{s.destPort}
                  </span>
                  <span className="text-muted">
                    {s.state} · {relativeTime(s.observedAt)}
                  </span>
                </li>
              ))}
            </ul>
          </details>
        )}
      </PanelSection>

      <div className="mt-auto flex flex-wrap gap-2 border-t border-border pt-4">
        <Button data-autofocus onClick={() => navigate(podDetailPath(ref.uid, clusterId))}>
          Open pod
        </Button>
        <AddToCaseButton
          entity={{
            type: 'link',
            label: `${podName} → ${dest.name}:${row.destPort ?? ''}`,
            href: `#/network-activity?${new URLSearchParams({ clusterId, podUid: ref.uid, q: row.destIp ?? '' }).toString()}`,
            meta: { observations: String(row.observationCount ?? 0), protocol: portLine },
          }}
          clusterId={clusterId}
        />
      </div>
    </SidePanel>
  );
};
