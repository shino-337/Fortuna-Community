import React, { useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { Loader2, Plus } from 'lucide-react';
import { PageLayout } from '../../design-system/layouts/PageLayout';
import { Dialog } from '../../design-system/components/Dialog';
import { SemanticEmptyState } from '../../design-system/components/SemanticEmptyState';
import { Button } from '../../components/ui/Button';
import { useAuthStore } from '../../store/authStore';
import { formatDateTime } from '../../lib/display';
import { PAGE_TITLES } from '../../lib/pageTitles';
import { caseStatusLabel, isCaseClosed } from '../../lib/caseLifecycle';
import type { InvestigationCase } from '../../store/investigationStore';
import { CaseStatusBadge } from './CaseStatusBadge';

type CaseView = 'open' | 'mine' | 'closed' | 'all';

const VIEWS: { id: CaseView; label: string }[] = [
  { id: 'open', label: 'Open' },
  { id: 'mine', label: 'Mine' },
  { id: 'closed', label: 'Closed' },
  { id: 'all', label: 'All' },
];

export function caseHref(id: string): string {
  return `/investigation?case=${encodeURIComponent(id)}`;
}

function linkedFindingCount(c: InvestigationCase): number {
  return c.entities.filter((e) => e.type === 'finding').length;
}

interface CaseListProps {
  cases: InvestigationCase[];
  error: string | null;
  canWrite: boolean;
  onRetry: () => void;
  onCreate: (title: string) => Promise<string>;
}

/** Every case the user can see, split into open and closed work. */
export const CaseList: React.FC<CaseListProps> = ({ cases, error, canWrite, onRetry, onCreate }) => {
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const me = (user?.username ?? '').toLowerCase();
  const [searchParams, setSearchParams] = useSearchParams();
  const viewParam = searchParams.get('view') as CaseView | null;
  const view: CaseView = VIEWS.some((v) => v.id === viewParam) ? (viewParam as CaseView) : 'open';
  const [creating, setCreating] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [busy, setBusy] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const byView = useMemo(() => {
    const open = cases.filter((c) => !isCaseClosed(c.status));
    return {
      open,
      mine: open.filter((c) => c.owner.toLowerCase() === me),
      closed: cases.filter((c) => isCaseClosed(c.status)),
      all: cases,
    } satisfies Record<CaseView, InvestigationCase[]>;
  }, [cases, me]);
  const rows = byView[view];

  const setView = (next: CaseView) => {
    setSearchParams(next === 'open' ? {} : { view: next }, { replace: true });
  };

  const submitNew = async () => {
    if (!newTitle.trim() || busy) return;
    setBusy(true);
    setCreateError(null);
    try {
      const id = await onCreate(newTitle.trim());
      setCreating(false);
      setNewTitle('');
      navigate(caseHref(id));
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <PageLayout
      title={PAGE_TITLES.investigations}
      description="Work that spans several findings: what is affected, what was done, and who owns it. Closing a case can resolve its findings in the same step."
      actions={
        canWrite ? (
          <Button size="sm" onClick={() => setCreating(true)}>
            <Plus className="mr-1 h-4 w-4" aria-hidden />
            New case
          </Button>
        ) : undefined
      }
    >
      {error ? (
        <div className="mb-4 flex items-center justify-between gap-2 rounded-lg border border-error-border bg-error-background px-3 py-2 text-caption text-error-foreground" role="alert">
          <span>Cases could not be loaded. {error}</span>
          <Button size="sm" variant="secondary" onClick={onRetry}>
            Retry
          </Button>
        </div>
      ) : null}

      <nav aria-label="Case views" className="mb-4 flex flex-wrap gap-1 rounded-lg border border-border bg-surface/40 p-1">
        {VIEWS.map((v) => (
          <button
            key={v.id}
            type="button"
            aria-current={view === v.id ? 'page' : undefined}
            onClick={() => setView(v.id)}
            className={`inline-flex min-h-9 items-center gap-2 rounded-md px-3 text-body transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 ${
              view === v.id ? 'bg-surface-2 font-semibold text-text' : 'text-muted hover:text-text'
            }`}
          >
            {v.label}
            <span className="rounded-full bg-surface-2 px-1.5 text-meta text-muted">{byView[v.id].length}</span>
          </button>
        ))}
      </nav>

      {rows.length === 0 ? (
        <SemanticEmptyState
          state="no_data"
          title={view === 'closed' ? 'No closed cases' : view === 'mine' ? 'No open cases owned by you' : 'No open cases'}
          reason={
            canWrite
              ? 'Use Add to case on a finding, pod or attack path, or create a case here.'
              : 'Cases you own or created appear here.'
          }
        />
      ) : (
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="w-full min-w-[40rem] text-left text-body">
            <thead className="border-b border-border bg-surface/60 text-meta uppercase tracking-wider text-muted">
              <tr>
                <th scope="col" className="px-4 py-2.5 font-semibold">Case</th>
                <th scope="col" className="px-4 py-2.5 font-semibold">Status</th>
                <th scope="col" className="px-4 py-2.5 font-semibold">Owner</th>
                <th scope="col" className="px-4 py-2.5 text-right font-semibold">Findings</th>
                <th scope="col" className="px-4 py-2.5 font-semibold">Updated</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((c) => (
                <tr
                  key={c.id}
                  className="cursor-pointer border-b border-border/60 last:border-b-0 hover:bg-surface-2/40"
                  onClick={() => navigate(caseHref(c.id))}
                >
                  <td className="px-4 py-3">
                    <Link
                      to={caseHref(c.id)}
                      className="font-semibold text-text hover:text-brand focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {c.title}
                    </Link>
                    <div className="text-meta text-muted">{c.clusterId ?? 'All clusters'}</div>
                  </td>
                  <td className="px-4 py-3">
                    <CaseStatusBadge status={c.status} />
                  </td>
                  <td className="px-4 py-3 text-muted">{c.owner || 'Unassigned'}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{linkedFindingCount(c)}</td>
                  <td className="px-4 py-3 text-muted" title={caseStatusLabel(c.status)}>
                    {formatDateTime(c.updatedAt)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Dialog
        open={creating}
        onClose={() => !busy && setCreating(false)}
        closeDisabled={busy}
        size="sm"
        title="New case"
        description="You own the case. It is bound to the cluster selected in the header, if any."
        footer={
          <>
            <Button size="sm" variant="secondary" onClick={() => setCreating(false)} disabled={busy}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => void submitNew()} disabled={busy || !newTitle.trim()}>
              {busy ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden /> : null}
              Create case
            </Button>
          </>
        }
      >
        <label className="block text-caption font-semibold text-muted" htmlFor="new-case-title">
          Title
        </label>
        <input
          id="new-case-title"
          type="text"
          value={newTitle}
          onChange={(e) => setNewTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void submitNew();
          }}
          placeholder="For example: Crypto-miner on payments-api"
          className="mt-1 w-full rounded-lg border border-border bg-base px-3 py-2 text-body text-text outline-none focus:border-brand"
        />
        {createError ? (
          <p className="mt-3 rounded-lg border border-error-border bg-error-background px-3 py-2 text-caption text-error-foreground" role="alert">
            {createError}
          </p>
        ) : null}
      </Dialog>
    </PageLayout>
  );
};
