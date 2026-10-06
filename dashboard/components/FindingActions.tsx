import React, { useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { Button } from './ui/Button';
import { Dialog } from '../design-system/components/Dialog';
import { AddToCaseButton } from './AddToCaseButton';
import { FindingAssignee } from './FindingAssignee';
import { api, type InsightAssigneeCandidate } from '../lib/api';
import { summarizeBulkFindingResult } from '../lib/bulkFindingResult';
import { findingInvestigationEntity } from '../lib/investigationEntities';
import { ACTION_IDS, canRunAction } from '../lib/actionAccess';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import type { Insight } from '../types';

export type FindingAction = 'acknowledge' | 'resolve' | 'dismiss' | 'reopen';

const ACTION_LABEL: Record<FindingAction, string> = {
  acknowledge: 'Acknowledge',
  resolve: 'Resolve',
  dismiss: 'Dismiss',
  reopen: 'Reopen',
};

const MIN_REASON = 8;

function workflowStatus(insight: Insight): string {
  const s = String(insight.status ?? '').toLowerCase();
  return s === 'new' ? 'active' : s;
}

interface FindingActionsProps {
  insight: Insight;
  /** Called after the action succeeded so the caller can refresh. */
  onDone: (action: FindingAction) => void;
  /** Lets a surrounding dialog pause its own focus trap while the review dialog is open. */
  onReviewOpenChange?: (open: boolean) => void;
  /** Called after the owner changed; the finding stays open, so callers refresh in place. */
  onAssigned?: (assignee: InsightAssigneeCandidate | null) => void;
  className?: string;
}

/**
 * Every workflow action on one finding, in one place: the detail panel and the full detail page both
 * render this. Only the actions the finding's state and the user's role allow are shown.
 */
export const FindingActions: React.FC<FindingActionsProps> = ({ insight, onDone, onReviewOpenChange, onAssigned, className }) => {
  const permUser = usePermUser();
  const status = workflowStatus(insight);
  const closed = status === 'resolved' || status === 'dismissed';
  const allowed: FindingAction[] = [];
  if (status === 'active' && canRunAction(permUser, ACTION_IDS.findingAcknowledge)) allowed.push('acknowledge');
  if (!closed && canRunAction(permUser, ACTION_IDS.findingResolve)) allowed.push('resolve');
  if (!closed && canRunAction(permUser, ACTION_IDS.findingDismiss)) allowed.push('dismiss');
  if (closed && can(permUser, P.findingsReopen)) allowed.push('reopen');
  const canSeeCases = can(permUser, P.investigationsWrite) || can(permUser, P.investigationsRead);

  const [pending, setPending] = useState<FindingAction | null>(null);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [caseDialogOpen, setCaseDialogOpen] = useState(false);
  const [assignDialogOpen, setAssignDialogOpen] = useState(false);
  useEffect(() => {
    onReviewOpenChange?.(pending !== null || caseDialogOpen || assignDialogOpen);
  }, [pending, caseDialogOpen, assignDialogOpen, onReviewOpenChange]);

  const needsReason = pending === 'resolve' || pending === 'dismiss';
  const close = () => {
    if (busy) return;
    setPending(null);
    setReason('');
    setError(null);
  };

  const submit = async () => {
    if (!pending || busy) return;
    const note = reason.trim();
    if (needsReason && note.length < MIN_REASON) return;
    setBusy(true);
    setError(null);
    try {
      if (pending === 'reopen') {
        await api.reopenInsight(insight.id);
      } else {
        const result = await api.bulkInsightsAction({
          action: pending,
          insightIds: [insight.id],
          resolution: pending === 'resolve' ? note : undefined,
          reason: pending === 'dismiss' ? note : undefined,
        });
        const outcome = summarizeBulkFindingResult(result, [insight.id]);
        if (!outcome.complete) {
          setError(outcome.message);
          return;
        }
      }
      const done = pending;
      setPending(null);
      setReason('');
      onDone(done);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const showOwner = Boolean(insight.assignee) || (can(permUser, P.findingsAck) && !closed);
  if (allowed.length === 0 && !canSeeCases && !showOwner) return null;

  return (
    <div className={`flex flex-wrap items-center gap-2 ${className ?? ''}`}>
      {showOwner ? (
        <div className="flex flex-wrap items-center gap-2 border-border sm:border-r sm:pr-3">
          <FindingAssignee insight={insight} onChange={(a) => onAssigned?.(a)} onDialogOpenChange={setAssignDialogOpen} />
        </div>
      ) : null}
      {allowed.map((action) => (
        <Button
          key={action}
          size="sm"
          variant={action === 'resolve' || (action === 'acknowledge' && !allowed.includes('resolve')) ? 'primary' : 'secondary'}
          onClick={() => {
            setError(null);
            setReason('');
            setPending(action);
          }}
        >
          {action === 'dismiss' ? 'Dismiss…' : ACTION_LABEL[action]}
        </Button>
      ))}
      {canSeeCases ? (
        <AddToCaseButton
          entity={findingInvestigationEntity(insight)}
          insightId={String(insight.id)}
          clusterId={insight.clusterId || null}
          onOpenChange={setCaseDialogOpen}
        />
      ) : null}

      <Dialog
        open={pending !== null}
        onClose={close}
        closeDisabled={busy}
        size="sm"
        title={pending ? `${ACTION_LABEL[pending]} finding` : ''}
        description={insight.title}
        footer={
          <>
            <Button size="sm" variant="secondary" onClick={close} disabled={busy}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => void submit()} disabled={busy || (needsReason && reason.trim().length < MIN_REASON)}>
              {busy ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden /> : null}
              {pending ? ACTION_LABEL[pending] : ''}
            </Button>
          </>
        }
      >
        {needsReason ? (
          <>
            <label className="block text-caption font-semibold text-muted" htmlFor="finding-action-reason">
              {pending === 'resolve' ? 'How was it fixed?' : 'Why is it not actionable?'}
            </label>
            <textarea
              id="finding-action-reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              className="mt-1 min-h-24 w-full rounded-lg border border-border bg-base px-3 py-2 text-body text-text outline-none focus:border-brand"
              placeholder={pending === 'resolve' ? 'The remediation or compensating control.' : 'For example: accepted risk, false positive, test workload.'}
            />
            <p className="mt-1 text-meta text-muted">At least {MIN_REASON} characters. It is saved to the finding's audit trail.</p>
          </>
        ) : (
          <p className="text-body text-muted">
            {pending === 'acknowledge'
              ? 'The finding moves to In review. It stays open until it is resolved or dismissed.'
              : 'The finding returns to Needs triage.'}
          </p>
        )}
        {error ? (
          <p className="mt-3 rounded-lg border border-error-border bg-error-background px-3 py-2 text-caption text-error-foreground" role="alert">
            {error}
          </p>
        ) : null}
      </Dialog>
    </div>
  );
};
