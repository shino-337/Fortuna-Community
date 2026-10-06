import React, { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Check, Circle, HelpCircle, RefreshCw } from 'lucide-react';
import { PageLayout } from '../../design-system/layouts/PageLayout';
import { Button } from '../../components/ui/Button';
import { PAGE_TITLES } from '../../lib/pageTitles';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';
import {
  SETUP_STEP_ORDER,
  setupComplete,
  setupDoneCount,
  useSetupProgressStore,
  type SetupStep,
  type SetupStepId,
} from '../../store/setupProgressStore';

interface StepCopy {
  title: string;
  why: string;
  how: React.ReactNode;
  action?: { label: string; to: string };
}

const STEP_COPY: Record<SetupStepId, StepCopy> = {
  agent: {
    title: 'Get an agent reporting',
    why: 'Everything Fortuna shows comes from the agent running in your cluster.',
    how: (
      <>
        The Helm chart installs an agent in the cluster it runs in. If none reports, check the agent DaemonSet with{' '}
        <code className="rounded bg-base px-1 font-mono text-meta">kubectl -n fortuna get ds</code>.
      </>
    ),
    action: { label: 'Open Platform', to: '/monitoring' },
  },
  scan: {
    title: 'Finish the first scan',
    why: 'The first inventory scan lists workloads and service accounts and produces the first findings.',
    how: 'It starts by itself a few minutes after the agent reports. Platform shows each stage as it completes.',
    action: { label: 'Open Platform', to: '/monitoring' },
  },
  triage: {
    title: 'Triage your first finding',
    why: 'Triage is the daily loop: take the top finding, decide, and record what you decided.',
    how: 'Open Findings, pick the top row, then Acknowledge, Resolve or Add to case.',
    action: { label: 'Open Findings', to: '/risks/findings' },
  },
  team: {
    title: 'Invite your team',
    why: 'Give each person their own account and the narrowest role that fits, so the audit log says who did what.',
    how: 'Add users in Users & Access and assign a role and, where needed, a cluster scope.',
    action: { label: 'Add users', to: '/settings' },
  },
  cluster: {
    title: 'Add another cluster',
    why: 'One Fortuna installation can watch several clusters. Each remote cluster runs only the agent.',
    how: (
      <>
        Expose Core from this cluster, then run{' '}
        <code className="rounded bg-base px-1 font-mono text-meta">scripts/deploy/sync-remote-agent.sh</code> with the remote kubeconfig. See
        "Add a remote cluster" in the install guide.
      </>
    ),
  },
};

function StepIcon({ state }: { state: SetupStep['state'] }) {
  if (state === 'done')
    return (
      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-emerald-500/20 text-emerald-300">
        <Check className="h-4 w-4" aria-hidden />
      </span>
    );
  if (state === 'unknown')
    return (
      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-surface-2 text-muted">
        <HelpCircle className="h-4 w-4" aria-hidden />
      </span>
    );
  return (
    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-border text-muted">
      <Circle className="h-3 w-3" aria-hidden />
    </span>
  );
}

const STATE_LABEL: Record<SetupStep['state'], string> = { done: 'Done', todo: 'To do', unknown: 'Unknown' };

/** First-run checklist: the steps that take a new installation to a team using it daily. */
export const Setup: React.FC = () => {
  const { steps, loading, load } = useSetupProgressStore();
  const { allowedRoutes } = useOperationalMaterialization();

  useEffect(() => {
    void load();
  }, [load]);

  const ordered = SETUP_STEP_ORDER.map((id) => steps?.find((s) => s.id === id)).filter((s): s is SetupStep => Boolean(s));
  const next = ordered.find((s) => s.state !== 'done' && !s.optional);
  const complete = setupComplete(steps);

  return (
    <PageLayout
      title={PAGE_TITLES.setup}
      description="The steps that take a new installation to a team using Fortuna every day. Each step checks itself from live data."
      actions={
        <Button size="sm" variant="secondary" onClick={() => void load()} isLoading={loading}>
          <RefreshCw className="mr-1 h-4 w-4" aria-hidden />
          Check again
        </Button>
      }
    >
      <div className="flex max-w-3xl flex-col gap-4">
        <section className="rounded-lg border border-border bg-surface px-4 py-3" aria-label="Setup progress">
          <p className="text-section-title text-text">
            {steps === null ? 'Checking…' : complete ? 'Setup is complete' : `${setupDoneCount(steps)} of ${ordered.length} done`}
          </p>
          <p className="mt-1 text-body text-muted">
            {steps === null
              ? ''
              : complete
                ? 'This page leaves the sidebar now. Start each day on Home.'
                : next
                  ? `Next: ${STEP_COPY[next.id].title.toLowerCase()}.`
                  : ''}
          </p>
        </section>

        <ol className="flex flex-col gap-3">
          {ordered.map((s, i) => {
            const copy = STEP_COPY[s.id];
            const isNext = next?.id === s.id;
            const action = copy.action && allowedRoutes.includes(copy.action.to) ? copy.action : undefined;
            return (
              <li
                key={s.id}
                className={`flex gap-4 rounded-lg border bg-surface p-4 ${isNext ? 'border-brand/60' : 'border-border'}`}
                aria-current={isNext ? 'step' : undefined}
              >
                <StepIcon state={s.state} />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                    <h2 className={`text-card-title ${s.state === 'done' ? 'text-muted' : 'text-text'}`}>
                      {i + 1}. {copy.title}
                    </h2>
                    {s.optional ? <span className="text-meta text-muted">Optional</span> : null}
                    <span className="text-meta text-muted">
                      {STATE_LABEL[s.state]} · {s.detail}
                    </span>
                  </div>
                  {s.state !== 'done' ? (
                    <>
                      <p className="mt-1.5 text-body text-muted">{copy.why}</p>
                      <p className="mt-1 text-body text-text">{copy.how}</p>
                    </>
                  ) : null}
                </div>
                {action && s.state !== 'done' ? (
                  <Link
                    to={action.to}
                    className={`inline-flex min-h-9 shrink-0 items-center self-start rounded-lg px-3 text-caption font-semibold ${
                      isNext ? 'bg-brand text-white hover:bg-brand/90' : 'border border-border text-text hover:border-muted'
                    }`}
                  >
                    {action.label}
                  </Link>
                ) : null}
              </li>
            );
          })}
        </ol>
      </div>
    </PageLayout>
  );
};
