import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { api, type ServiceAccountMutationAction, type ServiceAccountMutationPreview, type ServiceAccountMutationStep } from '../lib/api';
import { canAll, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { Card } from '../design-system/components/Card';
import { Button } from './ui/Button';

function subjectText(subject: { kind: string; name: string; namespace?: string }): string {
  return `${subject.kind} ${subject.namespace ? `${subject.namespace}/` : ''}${subject.name}`;
}

function StepEffect({ step }: { step: ServiceAccountMutationStep }) {
  return (
    <li className="rounded-lg border border-border bg-base/40 px-3 py-2 text-caption">
      <p className="font-semibold text-text">{step.kind} {step.namespace ? `${step.namespace}/` : ''}{step.name}</p>
      <p className="break-all font-mono text-muted">UID: {step.uid}{step.resourceVersion ? ` · resourceVersion: ${step.resourceVersion}` : ''}</p>
      {step.kind === 'RoleBinding' || step.kind === 'ClusterRoleBinding' ? (
        <div className="mt-1 text-muted">
          <p>Before: {(step.before ?? []).map(subjectText).join(', ') || 'no subjects'}</p>
          <p>After: {(step.after ?? []).map(subjectText).join(', ') || 'no subjects'}</p>
        </div>
      ) : null}
    </li>
  );
}

export const ServiceAccountMutationPanel: React.FC<{
  uid: string;
  clusterId: string;
  name: string;
  namespace: string;
}> = ({ uid, clusterId, name, namespace }) => {
  const user = usePermUser();
  const canDelete = canAll(user, [P.inventoryRead, P.inventoryDelete]);
  const canRevoke = canAll(user, [P.inventoryRead, P.inventoryModify, P.inventoryDelete]);
  const location = useLocation();
  const navigate = useNavigate();
  const operationId = new URLSearchParams(location.search).get('operationId')?.trim() ?? '';
  const sequence = useRef(0);
  const [current, setCurrent] = useState<ServiceAccountMutationPreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [reviewed, setReviewed] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const requirePlanIdentity = useCallback((result: ServiceAccountMutationPreview) => {
    if (result.plan.name !== name || result.plan.namespace !== namespace) {
      throw new Error('Mutation plan does not match this ServiceAccount identity');
    }
    return result;
  }, [name, namespace]);

  const setOperationId = useCallback((id: string) => {
    const query = new URLSearchParams(location.search);
    if (id) query.set('operationId', id);
    else query.delete('operationId');
    navigate({ pathname: location.pathname, search: query.toString() }, { replace: true });
  }, [location.pathname, location.search, navigate]);

  const refresh = useCallback(async () => {
    if (!operationId) return;
    const request = ++sequence.current;
    try {
      const result = requirePlanIdentity(await api.getServiceAccountMutation(operationId, uid, clusterId));
      if (request !== sequence.current) return;
      setCurrent(result);
      setError(null);
    } catch (cause) {
      if (request !== sequence.current) return;
      setError(cause instanceof Error ? cause.message : 'Unable to load mutation status');
    }
  }, [clusterId, operationId, requirePlanIdentity, uid]);

  useEffect(() => {
    if (operationId) void refresh();
    return () => { sequence.current++; };
  }, [operationId, refresh]);

  useEffect(() => {
    if (busy || !operationId || !current || !['queued', 'running', 'retry'].includes(current.operation.status)) return;
    const timer = window.setInterval(() => { void refresh(); }, 3000);
    return () => window.clearInterval(timer);
  }, [busy, current, operationId, refresh]);

  const preview = async (action: ServiceAccountMutationAction) => {
    const request = ++sequence.current;
    setBusy(true);
    setError(null);
    setCurrent(null);
    setReviewed(false);
    try {
      const result = requirePlanIdentity(await api.previewServiceAccountMutation(uid, clusterId, action));
      if (request !== sequence.current) return;
      setCurrent(result);
      setOperationId(result.operation.id);
    } catch (cause) {
      if (request !== sequence.current) return;
      setError(cause instanceof Error ? cause.message : 'Unable to create mutation preview');
    } finally {
      if (request === sequence.current) setBusy(false);
    }
  };

  const execute = async () => {
    if (!current || current.operation.status !== 'preview' || !reviewed || current.plan.steps.length === 0) return;
    if (Date.now() >= Date.parse(current.operation.expiresAt)) {
      setError('Preview expired. Create a fresh preview before executing.');
      return;
    }
    const request = ++sequence.current;
    setBusy(true);
    setError(null);
    try {
      await api.executeServiceAccountMutation(current);
      if (request !== sequence.current) return;
      setCurrent({ ...current, operation: { ...current.operation, status: 'queued' } });
      setReviewed(false);
    } catch (cause) {
      if (request !== sequence.current) return;
      setError(cause instanceof Error ? cause.message : 'Unable to queue mutation');
    } finally {
      if (request === sequence.current) setBusy(false);
    }
  };

  if (!canDelete && !canRevoke) return null;
  const operation = current?.operation;
  const active = operation && ['queued', 'running', 'retry'].includes(operation.status);
  return (
    <Card className="p-6 mt-6">
      <h3 className="text-section-title text-text mb-2">Kubernetes identity action</h3>
      <p className="text-caption text-muted mb-4">
        Preview the exact Kubernetes objects before changing {namespace}/{name}. An operation continues on the server after this page closes.
      </p>
      <div className="flex flex-wrap gap-2">
        {canRevoke ? <Button variant="secondary" disabled={busy || Boolean(active)} onClick={() => void preview('revoke')}>Preview direct grant revocation</Button> : null}
        {canDelete ? <Button variant="danger" disabled={busy || Boolean(active)} onClick={() => void preview('delete')}>Preview ServiceAccount deletion</Button> : null}
        {operationId ? <Button variant="secondary" disabled={busy} onClick={() => void refresh()}>Refresh operation</Button> : null}
      </div>
      {error ? <p role="alert" className="mt-3 text-caption text-danger">{error}</p> : null}
      {current ? (
        <div className="mt-5 space-y-4">
          <p role="status" className="text-body text-text">
            <strong>{operation?.action === 'revoke' ? 'Revoke direct grants' : 'Delete ServiceAccount'}</strong> · Status: <strong>{operation?.status}</strong>
          </p>
          <p className="text-caption text-muted break-all">Operation: {operation?.id} · Cluster: {clusterId} · UID: {uid}</p>
          {operation?.status === 'preview' ? <p className="text-caption text-muted">Preview expires: {new Date(operation.expiresAt).toLocaleString()}</p> : null}
          <div>
            <h4 className="font-semibold text-text mb-2">Exact planned effects ({current.plan.steps.length})</h4>
            {current.plan.steps.length ? (
              <ol className="max-h-72 space-y-2 overflow-y-auto">{current.plan.steps.map((step, index) => <StepEffect key={`${step.kind}/${step.uid}/${index}`} step={step} />)}</ol>
            ) : <p className="text-caption text-muted">No matching Kubernetes objects would change.</p>}
          </div>
          <div>
            <h4 className="font-semibold text-text mb-2">Limitations</h4>
            <ul className="list-disc pl-5 text-caption text-muted space-y-1">{current.plan.limitations.map((text, index) => <li key={index}>{text}</li>)}</ul>
          </div>
          {operation?.status === 'preview' && current.plan.steps.length > 0 ? (
            <div className="space-y-3">
              <label className="flex items-start gap-2 text-caption text-text">
                <input type="checkbox" checked={reviewed} onChange={(event) => setReviewed(event.target.checked)} />
                I reviewed every listed object and limitation for this {operation.action} operation.
              </label>
              <Button variant="danger" disabled={busy || !reviewed || Date.now() >= Date.parse(operation.expiresAt)} onClick={() => void execute()}>
                Execute reviewed {operation.action}
              </Button>
            </div>
          ) : null}
          {operation?.status !== 'preview' ? (
            <p className="text-caption text-muted">
              Completed steps: {operation?.completedSteps} / {current.plan.steps.length} · Attempts: {operation?.attempts}
              {operation?.lastError ? ` · Last error: ${operation.lastError}` : ''}
              {operation?.status === 'blocked' ? ' · Create a new preview after resolving the conflict.' : ''}
            </p>
          ) : null}
        </div>
      ) : null}
    </Card>
  );
};
