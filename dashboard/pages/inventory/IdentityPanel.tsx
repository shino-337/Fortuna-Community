import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Button } from '../../components/ui/Button';
import { SidePanel } from '../../components/SidePanel';
import { api } from '../../lib/api';
import { identityDetailPath, podDetailPath } from '../../lib/entityLinks';
import type { K8sEffectiveRule, K8sRbacResourceDetail, K8sResource, PodWithRisk, ServiceAccountK8sPermissions } from '../../types';
import { KIND_LABEL } from './IdentitiesView';
import { PanelSection, RiskBadge } from './shared';

type Load<T> = { state: 'loading' } | { state: 'ready'; data: T } | { state: 'failed' };

const RULES_SHOWN = 8;

function isWildcard(rule: K8sEffectiveRule): boolean {
  return Boolean(rule.verbs?.includes('*') || rule.resources?.includes('*'));
}

function RuleList({ rules }: { rules: K8sEffectiveRule[] }) {
  if (rules.length === 0) return <p className="text-body text-muted">No rules.</p>;
  return (
    <ul className="divide-y divide-border/60 rounded-lg border border-border">
      {rules.slice(0, RULES_SHOWN).map((r, i) => (
        <li key={i} className="flex items-start justify-between gap-3 px-3 py-2">
          <span className="min-w-0 font-mono text-caption text-text">
            {(r.verbs ?? []).join(', ') || '—'} <span className="text-muted">on</span>{' '}
            {(r.resources?.length ? r.resources : r.nonResourceURLs ?? []).join(', ') || '—'}
            {r.namespace ? <span className="text-muted"> in {r.namespace}</span> : null}
          </span>
          {isWildcard(r) ? <span className="shrink-0 rounded border border-red-500/50 px-1 text-meta font-semibold text-red-300">wildcard</span> : null}
        </li>
      ))}
      {rules.length > RULES_SHOWN ? <li className="px-3 py-2 text-caption text-muted">{(rules.length - RULES_SHOWN).toLocaleString()} more</li> : null}
    </ul>
  );
}

function names(list: Array<Record<string, unknown>> | undefined): string[] {
  return (list ?? []).map((x) => String(x.name ?? '')).filter(Boolean);
}

/** What an identity can do and who uses it. */
export const IdentityPanel: React.FC<{ resource: K8sResource | null; onClose: () => void }> = ({ resource, onClose }) => {
  const navigate = useNavigate();
  const [perms, setPerms] = useState<Load<ServiceAccountK8sPermissions>>({ state: 'loading' });
  const [usedBy, setUsedBy] = useState<Load<PodWithRisk[]>>({ state: 'loading' });
  const [detail, setDetail] = useState<Load<K8sRbacResourceDetail>>({ state: 'loading' });
  const isSA = resource?.kind === 'ServiceAccount';

  useEffect(() => {
    if (!resource) return;
    let cancelled = false;
    setPerms({ state: 'loading' });
    setUsedBy({ state: 'loading' });
    setDetail({ state: 'loading' });
    if (resource.kind === 'ServiceAccount') {
      api
        .getServiceAccountPermissions(resource.id, resource.clusterId)
        .then((p) => !cancelled && setPerms({ state: 'ready', data: p }))
        .catch(() => !cancelled && setPerms({ state: 'failed' }));
      api
        .getPods({ cluster: resource.clusterId, namespace: resource.namespace, serviceAccount: resource.name, sortBy: 'risk_desc', pageSize: 5 })
        .then((r) => !cancelled && setUsedBy({ state: 'ready', data: r.pods }))
        .catch(() => !cancelled && setUsedBy({ state: 'failed' }));
    } else {
      api
        .getResourceDetail(resource.kind, resource.id)
        .then((d) => !cancelled && setDetail(d ? { state: 'ready', data: d } : { state: 'failed' }))
        .catch(() => !cancelled && setDetail({ state: 'failed' }));
    }
    return () => {
      cancelled = true;
    };
  }, [resource]);

  if (!resource) return null;
  const scope = resource.kind === 'ClusterRole' || resource.kind === 'ClusterRoleBinding' ? 'cluster-wide' : resource.namespace;

  return (
    <SidePanel open onClose={onClose} title={resource.name} subtitle={[KIND_LABEL[resource.kind] ?? resource.kind, scope, resource.clusterId].filter(Boolean).join(' · ')}>
      {isSA ? (
        <>
          <PanelSection title="Can do">
            {perms.state === 'loading' ? (
              <p className="text-caption text-muted">Loading…</p>
            ) : perms.state === 'failed' ? (
              <p className="text-caption text-muted">Permissions could not be loaded.</p>
            ) : (
              <RuleList rules={perms.data.effectiveRules ?? []} />
            )}
          </PanelSection>
          {perms.state === 'ready' ? (
            <PanelSection title="Granted by">
              <p className="text-body text-text">
                {[
                  ...(perms.data.roleBindings ?? []).map((b) => String(b.roleBinding?.name ?? '')),
                  ...(perms.data.clusterRoleBindings ?? []).map((b) => String(b.clusterRoleBinding?.name ?? '')),
                ]
                  .filter(Boolean)
                  .join(', ') || <span className="text-muted">No bindings.</span>}
              </p>
            </PanelSection>
          ) : null}
          <PanelSection title="Used by">
            {usedBy.state === 'loading' ? (
              <p className="text-caption text-muted">Loading…</p>
            ) : usedBy.state === 'failed' ? (
              <p className="text-caption text-muted">Workloads could not be loaded.</p>
            ) : usedBy.data.length === 0 ? (
              <p className="text-body text-muted">No running workload uses it.</p>
            ) : (
              <ul className="divide-y divide-border/60 rounded-lg border border-border">
                {usedBy.data.map((p) => (
                  <li key={p.uid}>
                    <Link to={podDetailPath(p.uid, p.clusterId)} className="flex items-center justify-between gap-3 px-3 py-2 hover:bg-surface-2">
                      <span className="min-w-0 truncate text-body text-text">{p.name}</span>
                      <RiskBadge level={p.finalLevel} score={p.unifiedScore ?? p.totalScore} />
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </PanelSection>
          <div className="mt-auto flex flex-wrap gap-2 border-t border-border pt-4">
            <Button data-autofocus onClick={() => navigate(identityDetailPath({ uid: resource.id, clusterId: resource.clusterId }))}>
              Open identity
            </Button>
          </div>
        </>
      ) : detail.state === 'loading' ? (
        <p className="text-caption text-muted">Loading…</p>
      ) : detail.state === 'failed' ? (
        <p className="text-caption text-muted">Details could not be loaded.</p>
      ) : (
        <>
          {detail.data.roleRef ? (
            <PanelSection title="Grants role">
              <p className="text-body text-text">
                {String(detail.data.roleRef.kind ?? '')} {String(detail.data.roleRef.name ?? '')}
              </p>
            </PanelSection>
          ) : null}
          {detail.data.subjects?.length ? (
            <PanelSection title="To">
              <ul className="flex flex-col gap-1 text-body text-text">
                {detail.data.subjects.map((s, i) => (
                  <li key={i}>
                    <span className="text-muted">{String(s.kind ?? '')}</span> {String(s.name ?? '')}
                    {s.namespace ? <span className="text-muted"> in {String(s.namespace)}</span> : null}
                  </li>
                ))}
              </ul>
            </PanelSection>
          ) : null}
          {detail.data.rules ? (
            <PanelSection title="Can do">
              <RuleList rules={detail.data.rules} />
            </PanelSection>
          ) : null}
          {names(detail.data.referencedBy).length ? (
            <PanelSection title="Bound by">
              <p className="text-body text-text">{names(detail.data.referencedBy).join(', ')}</p>
            </PanelSection>
          ) : null}
        </>
      )}
    </SidePanel>
  );
};
