import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Briefcase, FolderPlus, Loader2 } from 'lucide-react';
import { Button } from './ui/Button';
import { Dialog } from '../design-system/components/Dialog';
import { api, type InvestigationCaseApi } from '../lib/api';
import type { InvestigationEntity } from '../store/investigationStore';
import { useAuthStore } from '../store/authStore';
import { useCan } from '../hooks/usePermUser';
import { P } from '../lib/permissions';
import { caseStatusLabel, isCaseClosed } from '../lib/caseLifecycle';
import type { InvestigationStatus } from '../store/investigationStore';

const NEW_CASE = '__new__';

function caseHref(id: string): string {
  return `/investigation?case=${encodeURIComponent(id)}`;
}

function openCases(items: InvestigationCaseApi[]): InvestigationCaseApi[] {
  return items.filter((c) => !isCaseClosed(String(c.status).toUpperCase() as InvestigationStatus));
}

interface AddToCaseButtonProps {
  entity: Omit<InvestigationEntity, 'id' | 'pinnedAt'>;
  /** The cluster a new case is bound to; a case bound to a cluster only takes evidence from it. */
  clusterId?: string | null;
  /** For a finding: its id, so the button can say which cases already link it. */
  insightId?: string;
  size?: 'sm' | 'md';
  className?: string;
  /** Lets a surrounding dialog pause its own focus trap while the picker is open. */
  onOpenChange?: (open: boolean) => void;
}

/**
 * Adds a finding, pod or attack path to a case the user picks, or to a new case. For a finding it also
 * shows the cases that already link it, so the same work is not opened twice.
 */
export const AddToCaseButton: React.FC<AddToCaseButtonProps> = ({ entity, clusterId, insightId, size = 'sm', className, onOpenChange }) => {
  const canRead = useCan(P.investigationsRead);
  const canWrite = useCan(P.investigationsWrite);
  const { user } = useAuthStore();
  const [linked, setLinked] = useState<InvestigationCaseApi[]>([]);
  const [open, setOpen] = useState(false);
  const [cases, setCases] = useState<InvestigationCaseApi[] | null>(null);
  const [choice, setChoice] = useState<string>(NEW_CASE);
  const [title, setTitle] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [added, setAdded] = useState<{ id: string; title: string } | null>(null);

  useEffect(() => {
    setAdded(null);
    if (!insightId || !canRead) {
      setLinked([]);
      return undefined;
    }
    let cancelled = false;
    api
      .listInvestigationCasesForFinding(insightId)
      .then((items) => {
        if (!cancelled) setLinked(items);
      })
      .catch(() => {
        if (!cancelled) setLinked([]);
      });
    return () => {
      cancelled = true;
    };
  }, [insightId, canRead]);

  useEffect(() => {
    onOpenChange?.(open);
  }, [open, onOpenChange]);

  const openDialog = () => {
    setOpen(true);
    setError(null);
    setTitle(entity.label.slice(0, 120));
    setCases(null);
    api
      .listInvestigationCases()
      .then(({ items }) => {
        const usable = openCases(items).filter((c) => !linked.some((l) => l.id === c.id));
        setCases(usable);
        setChoice(usable[0]?.id ?? NEW_CASE);
      })
      .catch((e) => {
        setCases([]);
        setChoice(NEW_CASE);
        setError(e instanceof Error ? e.message : String(e));
      });
  };

  const close = () => {
    if (!busy) setOpen(false);
  };

  const submit = async () => {
    if (busy) return;
    if (choice === NEW_CASE && !title.trim()) return;
    setBusy(true);
    setError(null);
    try {
      let target: { id: string; title: string };
      if (choice === NEW_CASE) {
        const created = await api.createInvestigationCase({
          title: title.trim(),
          owner: user?.username ?? user?.email ?? '',
          clusterId: clusterId || null,
        });
        target = { id: created.id, title: created.title };
      } else {
        const picked = cases?.find((c) => c.id === choice);
        target = { id: choice, title: picked?.title ?? 'case' };
      }
      const res = await api.pinInvestigationEntity(target.id, {
        type: entity.type,
        label: entity.label,
        href: entity.href,
        meta: entity.meta,
      });
      setAdded(target);
      if (insightId) setLinked((prev) => (prev.some((c) => c.id === res.case.id) ? prev : [...prev, res.case]));
      setOpen(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const shown = added && !linked.some((c) => c.id === added.id) ? [...linked, { id: added.id, title: added.title } as InvestigationCaseApi] : linked;

  if (!canWrite && shown.length === 0) return null;

  return (
    <>
      {canWrite ? (
        <Button type="button" variant="secondary" size={size} className={className} onClick={openDialog}>
          <FolderPlus className="mr-1 h-4 w-4" aria-hidden />
          Add to case
        </Button>
      ) : null}
      {shown.length > 0 ? (
        <span className="inline-flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted">
          <Briefcase className="h-3.5 w-3.5" aria-hidden />
          In case
          {shown.map((c) => (
            <Link key={c.id} to={caseHref(c.id)} className="font-medium text-brand hover:underline">
              {c.title}
            </Link>
          ))}
        </span>
      ) : null}

      <Dialog
        open={open}
        onClose={close}
        closeDisabled={busy}
        size="sm"
        title="Add to case"
        description={entity.label}
        footer={
          <>
            <Button size="sm" variant="secondary" onClick={close} disabled={busy}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => void submit()} disabled={busy || cases === null || (choice === NEW_CASE && !title.trim())}>
              {busy ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden /> : null}
              {choice === NEW_CASE ? 'Create case' : 'Add'}
            </Button>
          </>
        }
      >
        {cases === null ? (
          <p className="text-body text-muted">Loading open cases…</p>
        ) : (
          <fieldset className="space-y-2">
            <legend className="sr-only">Case</legend>
            {cases.map((c) => (
              <label
                key={c.id}
                className="flex cursor-pointer items-center gap-3 rounded-lg border border-border px-3 py-2 text-body hover:border-muted has-[:checked]:border-brand has-[:checked]:bg-brand/10"
              >
                <input type="radio" name="add-to-case" value={c.id} checked={choice === c.id} onChange={() => setChoice(c.id)} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-text">{c.title}</span>
                  <span className="text-meta text-muted">
                    {caseStatusLabel(String(c.status).toUpperCase() as InvestigationStatus)}
                    {c.owner ? ` · ${c.owner}` : ''}
                  </span>
                </span>
              </label>
            ))}
            <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-border px-3 py-2 text-body hover:border-muted has-[:checked]:border-brand has-[:checked]:bg-brand/10">
              <input
                type="radio"
                name="add-to-case"
                value={NEW_CASE}
                checked={choice === NEW_CASE}
                onChange={() => setChoice(NEW_CASE)}
                className="mt-1"
              />
              <span className="min-w-0 flex-1 space-y-1.5">
                <span className="block font-medium text-text">New case</span>
                {choice === NEW_CASE ? (
                  <input
                    type="text"
                    aria-label="New case title"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    className="w-full rounded-lg border border-border bg-base px-3 py-1.5 text-body text-text outline-none focus:border-brand"
                  />
                ) : null}
              </span>
            </label>
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
