import React, { useEffect, useState } from 'react';
import { Loader2, UserRound } from 'lucide-react';
import { Button } from './ui/Button';
import { Dialog } from '../design-system/components/Dialog';
import { api, type InsightAssigneeCandidate } from '../lib/api';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import type { Insight } from '../types';

interface FindingAssigneeProps {
  insight: Insight;
  /** Called with the new owner (null when unassigned) after the change is saved. */
  onChange: (assignee: InsightAssigneeCandidate | null) => void;
  onDialogOpenChange?: (open: boolean) => void;
}

function isClosed(insight: Insight): boolean {
  const s = String(insight.status ?? '').toLowerCase();
  return s === 'resolved' || s === 'dismissed';
}

/**
 * Who owns the finding. People who can triage take it in one click or hand it to a teammate; the server
 * only offers teammates who can triage this finding themselves.
 */
export const FindingAssignee: React.FC<FindingAssigneeProps> = ({ insight, onChange, onDialogOpenChange }) => {
  const permUser = usePermUser();
  const canAssign = can(permUser, P.findingsAck) && !isClosed(insight);
  const myId = permUser?.id != null ? Number(permUser.id) : NaN;
  const owner = insight.assignee ?? '';
  const ownedByMe = owner !== '' && insight.assigneeUserId != null && insight.assigneeUserId === myId;

  const [open, setOpen] = useState(false);
  const [candidates, setCandidates] = useState<InsightAssigneeCandidate[] | null>(null);
  const [choice, setChoice] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    onDialogOpenChange?.(open);
  }, [open, onDialogOpenChange]);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setCandidates(null);
    api
      .listInsightAssignees(insight.id)
      .then((items) => {
        if (!cancelled) setCandidates(items);
      })
      .catch((e) => {
        if (!cancelled) {
          setCandidates([]);
          setError(e instanceof Error ? e.message : String(e));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [open, insight.id]);

  const save = async (userId: number | null) => {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      const res = await api.assignInsight(insight.id, userId);
      setOpen(false);
      onChange(res.assignee ?? null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  if (!canAssign) {
    if (!owner) return null;
    return (
      <span className="inline-flex items-center gap-1.5 text-caption text-muted" data-testid="finding-owner">
        <UserRound className="h-3.5 w-3.5" aria-hidden />
        Owner: <span className="font-semibold text-text">{owner}</span>
      </span>
    );
  }

  return (
    <>
      <span className="inline-flex items-center gap-1.5 text-caption text-muted" data-testid="finding-owner">
        <UserRound className="h-3.5 w-3.5" aria-hidden />
        Owner: <span className={owner ? 'font-semibold text-text' : ''}>{owner ? (ownedByMe ? `${owner} (you)` : owner) : 'nobody'}</span>
      </span>
      {!ownedByMe && Number.isFinite(myId) ? (
        <Button size="sm" variant="secondary" onClick={() => void save(myId)} disabled={busy}>
          Take it
        </Button>
      ) : null}
      <Button
        size="sm"
        variant="secondary"
        onClick={() => {
          setError(null);
          setChoice(insight.assigneeUserId ?? null);
          setOpen(true);
        }}
      >
        Assign…
      </Button>
      {!open && error ? (
        <span className="text-caption text-error-foreground" role="alert">
          {error}
        </span>
      ) : null}

      <Dialog
        open={open}
        onClose={() => !busy && setOpen(false)}
        closeDisabled={busy}
        size="sm"
        title="Assign finding"
        description={insight.title}
        footer={
          <>
            <Button size="sm" variant="secondary" onClick={() => setOpen(false)} disabled={busy}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => void save(choice)} disabled={busy || candidates === null || choice === (insight.assigneeUserId ?? null)}>
              {busy ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden /> : null}
              Save
            </Button>
          </>
        }
      >
        <p className="text-body text-muted">Only people who can triage this finding are listed.</p>
        {candidates === null ? (
          <p className="mt-3 flex items-center gap-2 text-caption text-muted">
            <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden /> Loading people…
          </p>
        ) : (
          <fieldset className="mt-3 max-h-64 overflow-y-auto rounded-lg border border-border">
            <legend className="sr-only">Owner</legend>
            {[{ id: null as number | null, username: 'Nobody' }, ...candidates].map((c) => (
              <label
                key={c.id ?? 'none'}
                className="flex min-h-10 cursor-pointer items-center gap-2 border-b border-border px-3 text-body text-text last:border-b-0 hover:bg-surface-2"
              >
                <input type="radio" name="finding-assignee" checked={choice === c.id} onChange={() => setChoice(c.id)} className="accent-brand" />
                <span className={c.id === null ? 'text-muted' : ''}>{c.username}</span>
                {c.id !== null && c.id === myId ? <span className="text-meta text-muted">you</span> : null}
              </label>
            ))}
          </fieldset>
        )}
        {error ? (
          <p className="mt-3 rounded-lg border border-error-border bg-error-background px-3 py-2 text-caption text-error-foreground" role="alert">
            {error}
          </p>
        ) : null}
      </Dialog>
    </>
  );
};
