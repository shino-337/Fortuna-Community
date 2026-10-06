import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ExternalLink, Loader2, Trash2, X } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../design-system/components/Dialog';
import { PageContainer } from '../../design-system/layouts/PageContainer';
import { useConfirm } from '../../design-system/components/ConfirmDialog';
import { InvestigationTimeline } from '../../components/InvestigationTimeline';
import { api, type InvestigationLinkedFindingApi } from '../../lib/api';
import { summarizeBulkFindingResult } from '../../lib/bulkFindingResult';
import { ACTION_IDS, canRunAction } from '../../lib/actionAccess';
import { usePermUser } from '../../hooks/usePermUser';
import { useAuthStore } from '../../store/authStore';
import { formatDateTime } from '../../lib/display';
import { downloadText } from '../../lib/download';
import { getSeverityBadgeClass } from '../../lib/severity';
import { CASE_STEPS, canMoveCase, caseStatusLabel, isCaseClosed, isFindingOpen } from '../../lib/caseLifecycle';
import type {
  InvestigationCase,
  InvestigationEntity,
  InvestigationStatus,
  RemediationAction,
  RemediationActionStatus,
} from '../../store/investigationStore';
import { CaseStatusBadge } from './CaseStatusBadge';

const MIN_REASON = 8;

const FINDING_STATUS_LABEL: Record<string, string> = {
  active: 'Needs triage',
  new: 'Needs triage',
  acknowledged: 'In review',
  resolved: 'Resolved',
  dismissed: 'Dismissed',
};

const ENTITY_TYPE_LABEL: Record<string, string> = {
  pod: 'Pod',
  attack_path: 'Attack path',
  identity: 'Identity',
  resource: 'Resource',
  link: 'Link',
};

function routeFromHref(href?: string): string | null {
  if (!href) return null;
  return href.startsWith('#') ? href.slice(1) : href.startsWith('/') ? href : null;
}

interface Asset {
  key: string;
  name: string;
  detail: string;
  links: { label: string; to: string }[];
}

/** Workloads named by the linked findings, then any pod, path or identity added to the case directly. */
function affectedAssets(findings: InvestigationLinkedFindingApi[], entities: InvestigationEntity[]): Asset[] {
  const out = new Map<string, Asset>();
  for (const f of findings) {
    if (f.missing || !f.resourceName) continue;
    const key = `${f.clusterId ?? ''}/${f.resourceUid || f.resourceName}`;
    if (out.has(key)) continue;
    const links: Asset['links'] = [];
    if (f.resourceType === 'Pod' && f.resourceUid) {
      const cluster = f.clusterId ? `?clusterId=${encodeURIComponent(f.clusterId)}` : '';
      links.push({ label: 'Pod', to: `/resources/pods/uid/${encodeURIComponent(f.resourceUid)}${cluster}` });
      links.push({ label: 'Attack paths', to: `/attack-paths?podUid=${encodeURIComponent(f.resourceUid)}` });
    }
    out.set(key, {
      key,
      name: f.resourceName,
      detail: [f.resourceType, f.resourceNamespace].filter(Boolean).join(' · '),
      links,
    });
  }
  for (const e of entities) {
    if (e.type === 'finding') continue;
    const to = routeFromHref(e.href);
    out.set(`entity:${e.id}`, {
      key: `entity:${e.id}`,
      name: e.label,
      detail: [ENTITY_TYPE_LABEL[e.type] ?? e.type, e.meta?.namespace].filter(Boolean).join(' · '),
      links: to ? [{ label: 'Open', to }] : [],
    });
  }
  return [...out.values()];
}

interface CaseDetailProps {
  activeCase: InvestigationCase;
  canWrite: boolean;
  canDelete: boolean;
  onUpdate: (patch: Partial<InvestigationCase>) => Promise<void>;
  onArchive: () => Promise<void>;
}

/** One case: where it is in its lifecycle, the findings and assets it covers, and what happened. */
export const CaseDetail: React.FC<CaseDetailProps> = ({ activeCase, canWrite, canDelete, onUpdate, onArchive }) => {
  const confirm = useConfirm();
  const permUser = usePermUser();
  const { user } = useAuthStore();
  const author = user?.username ?? user?.email ?? 'analyst';
  const isAdmin = String(user?.role ?? '').toLowerCase() === 'admin';
  const canResolveFindings = canRunAction(permUser, ACTION_IDS.findingResolve);
  const closed = isCaseClosed(activeCase.status);
  const editable = canWrite && activeCase.status !== 'ARCHIVED';

  const [findings, setFindings] = useState<InvestigationLinkedFindingApi[]>([]);
  const [hiddenFindings, setHiddenFindings] = useState(0);
  const [findingsError, setFindingsError] = useState<string | null>(null);
  const [findingsLoading, setFindingsLoading] = useState(true);
  const [timelineKey, setTimelineKey] = useState(0);
  const [saveError, setSaveError] = useState<string | null>(null);

  const [titleDraft, setTitleDraft] = useState(activeCase.title);
  const [ownerDraft, setOwnerDraft] = useState(activeCase.owner);
  const [noteOpen, setNoteOpen] = useState(false);
  const [noteDraft, setNoteDraft] = useState('');
  const [taskDraft, setTaskDraft] = useState('');

  const [closing, setClosing] = useState(false);
  const [closeReason, setCloseReason] = useState('');
  const [resolveLinked, setResolveLinked] = useState(true);
  const [closeBusy, setCloseBusy] = useState(false);
  const [closeError, setCloseError] = useState<string | null>(null);

  useEffect(() => setTitleDraft(activeCase.title), [activeCase.title]);
  useEffect(() => setOwnerDraft(activeCase.owner), [activeCase.owner]);

  const loadFindings = useCallback(async () => {
    setFindingsLoading(true);
    try {
      const res = await api.listInvestigationFindings(activeCase.id);
      setFindings(res.items);
      setHiddenFindings(res.hidden);
      setFindingsError(null);
    } catch (e) {
      setFindingsError(e instanceof Error ? e.message : String(e));
    } finally {
      setFindingsLoading(false);
    }
  }, [activeCase.id]);

  const linkedFindingKeys = activeCase.entities
    .filter((e) => e.type === 'finding')
    .map((e) => e.id)
    .join('|');
  useEffect(() => {
    void loadFindings();
  }, [loadFindings, linkedFindingKeys]);

  /** Saves a change and reports whether it was saved, so drafts are only cleared on success. */
  const save = async (patch: Partial<InvestigationCase>): Promise<boolean> => {
    setSaveError(null);
    try {
      await onUpdate(patch);
      setTimelineKey((k) => k + 1);
      return true;
    } catch (e) {
      setSaveError(e instanceof Error ? e.message : String(e));
      return false;
    }
  };

  const openFindings = findings.filter((f) => !f.missing && isFindingOpen(f.status));
  const assets = useMemo(() => affectedAssets(findings, activeCase.entities), [findings, activeCase.entities]);
  const canClose = editable && !closed && canMoveCase(activeCase.status, 'RESOLVED', isAdmin);
  const nextSteps = CASE_STEPS.filter(
    (s) => s.status !== 'RESOLVED' && canMoveCase(activeCase.status, s.status, isAdmin),
  );
  const currentIndex = CASE_STEPS.findIndex((s) => s.status === activeCase.status);

  const commitTitle = () => {
    const next = titleDraft.trim();
    if (!next || next === activeCase.title) {
      setTitleDraft(activeCase.title);
      return;
    }
    void save({ title: next });
  };

  const commitOwner = () => {
    const next = ownerDraft.trim();
    if (next === activeCase.owner) return;
    void save({ owner: next });
  };

  const addNote = async () => {
    const body = noteDraft.trim();
    if (!body) return;
    const saved = await save({
      notes: [{ id: `note_${Date.now()}`, body, createdAt: new Date().toISOString(), author }, ...activeCase.notes],
    });
    if (!saved) return;
    setNoteDraft('');
    setNoteOpen(false);
  };

  const unlink = async (entityId: string, label: string) => {
    const ok = await confirm({
      title: 'Remove from case',
      description: `"${label}" will no longer be part of this case. The finding itself does not change.`,
      confirmLabel: 'Remove',
    });
    if (ok) await save({ entities: activeCase.entities.filter((e) => e.id !== entityId) });
  };

  const tasks = activeCase.remediationActions ?? [];
  const patchTask = (id: string, patch: Partial<RemediationAction>) =>
    save({ remediationActions: tasks.map((t) => (t.id === id ? { ...t, ...patch } : t)) });
  const addTask = async () => {
    const title = taskDraft.trim();
    if (!title) return;
    const saved = await save({
      remediationActions: [
        { id: `rem_${Date.now()}`, title, status: 'pending', owner: author, createdAt: new Date().toISOString() },
        ...tasks,
      ],
    });
    if (saved) setTaskDraft('');
  };

  const openClose = () => {
    setCloseReason('');
    setCloseError(null);
    setResolveLinked(canResolveFindings && openFindings.length > 0);
    setClosing(true);
  };

  const submitClose = async () => {
    const reason = closeReason.trim();
    if (reason.length < MIN_REASON || closeBusy) return;
    setCloseBusy(true);
    setCloseError(null);
    try {
      if (resolveLinked && openFindings.length > 0) {
        const ids = openFindings.map((f) => f.insightId);
        const result = await api.bulkInsightsAction({
          action: 'resolve',
          insightIds: ids,
          resolution: `Closed with case "${activeCase.title}": ${reason}`,
        });
        const outcome = summarizeBulkFindingResult(result, ids);
        if (!outcome.complete) {
          // Keep the case open so nothing is closed while findings are still open.
          setCloseError(`${outcome.message} The case is still open.`);
          void loadFindings();
          return;
        }
      }
      await onUpdate({
        status: 'RESOLVED',
        notes: [{ id: `note_${Date.now()}`, body: `Closed: ${reason}`, createdAt: new Date().toISOString(), author }, ...activeCase.notes],
      });
      setClosing(false);
      setTimelineKey((k) => k + 1);
      void loadFindings();
    } catch (e) {
      setCloseError(e instanceof Error ? e.message : String(e));
    } finally {
      setCloseBusy(false);
    }
  };

  const exportHandoff = () => {
    const lines = [
      `# ${activeCase.title}`,
      `Status: ${caseStatusLabel(activeCase.status)}`,
      `Owner: ${activeCase.owner || 'Unassigned'}`,
      `Cluster: ${activeCase.clusterId ?? 'All clusters'}`,
      `Updated: ${activeCase.updatedAt}`,
      '',
      '## Findings',
      ...findings.map((f) => `- [${FINDING_STATUS_LABEL[f.status] ?? (f.missing ? 'Deleted' : f.status)}] ${f.title} (${f.resourceName || 'unknown resource'})`),
      '',
      '## Affected assets',
      ...assets.map((a) => `- ${a.name} (${a.detail})`),
      '',
      '## Remediation',
      ...tasks.map((a) => `- [${a.status}] ${a.title}${a.owner ? ` (@${a.owner})` : ''}${a.externalTicketUrl ? ` ${a.externalTicketUrl}` : ''}`),
      '',
      '## Notes',
      ...activeCase.notes.map((n) => `- ${n.createdAt} (${n.author}): ${n.body}`),
    ];
    downloadText(lines.join('\n'), `case-${activeCase.id}.md`, 'text/markdown;charset=utf-8');
  };

  return (
    <PageContainer className="gap-5">
      <nav aria-label="Breadcrumb" className="text-caption text-muted">
        <Link to="/investigation" className="text-brand hover:underline">
          Cases
        </Link>{' '}
        / {activeCase.title}
      </nav>

      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          {editable ? (
            <input
              aria-label="Case title"
              value={titleDraft}
              onChange={(e) => setTitleDraft(e.target.value)}
              onBlur={commitTitle}
              onKeyDown={(e) => {
                if (e.key === 'Enter') (e.target as HTMLInputElement).blur();
                if (e.key === 'Escape') setTitleDraft(activeCase.title);
              }}
              className="w-full rounded-md border border-transparent bg-transparent px-1 -mx-1 text-page-title text-text outline-none hover:border-border focus:border-brand"
            />
          ) : (
            <h1 className="text-page-title text-text">{activeCase.title}</h1>
          )}
          <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted">
            <CaseStatusBadge status={activeCase.status} />
            <span>Owner {activeCase.owner || 'unassigned'}</span>
            <span aria-hidden>·</span>
            <span>Opened {formatDateTime(activeCase.createdAt)}</span>
            <span aria-hidden>·</span>
            <span>{activeCase.clusterId ?? 'All clusters'}</span>
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {editable ? (
            <Button size="sm" variant="secondary" onClick={() => setNoteOpen((v) => !v)}>
              Add note
            </Button>
          ) : null}
          <Button size="sm" variant="secondary" onClick={exportHandoff}>
            Export handoff
          </Button>
          {editable && activeCase.status === 'RESOLVED' && canMoveCase('RESOLVED', 'ACTIVE', isAdmin) ? (
            <Button size="sm" variant="secondary" onClick={() => void save({ status: 'ACTIVE' })}>
              Reopen
            </Button>
          ) : null}
          {editable && !closed ? (
            <Button
              size="sm"
              onClick={openClose}
              disabled={!canClose}
              title={canClose ? undefined : 'Move the case to Contained or Remediating before closing it.'}
            >
              Close case…
            </Button>
          ) : null}
        </div>
      </header>

      {saveError ? (
        <p className="rounded-lg border border-error-border bg-error-background px-3 py-2 text-caption text-error-foreground" role="alert">
          The change was not saved. {saveError}
        </p>
      ) : null}

      {noteOpen ? (
        <section className="rounded-lg border border-border bg-surface p-4" aria-label="New note">
          <label htmlFor="case-note" className="block text-caption font-semibold text-muted">
            Note
          </label>
          <textarea
            id="case-note"
            value={noteDraft}
            onChange={(e) => setNoteDraft(e.target.value)}
            className="mt-1 min-h-20 w-full rounded-lg border border-border bg-base px-3 py-2 text-body text-text outline-none focus:border-brand"
            placeholder="What you found, decided or handed over. It goes to the timeline."
          />
          <div className="mt-2 flex justify-end gap-2">
            <Button size="sm" variant="secondary" onClick={() => setNoteOpen(false)}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => void addNote()} disabled={!noteDraft.trim()}>
              Save note
            </Button>
          </div>
        </section>
      ) : null}

      <ol className="flex flex-wrap gap-2 rounded-lg border border-border bg-surface p-4" aria-label="Case lifecycle">
        {CASE_STEPS.map((step, i) => {
          const done = currentIndex >= 0 && i <= currentIndex;
          const current = step.status === activeCase.status;
          const reachable = editable && nextSteps.some((s) => s.status === step.status);
          return (
            <li key={step.status} className="flex min-w-[6.5rem] flex-1 flex-col gap-1.5" aria-current={current ? 'step' : undefined}>
              <span className={`h-1 rounded-full ${done ? 'bg-brand' : 'bg-surface-2'}`} aria-hidden />
              {reachable ? (
                <button
                  type="button"
                  onClick={() => void save({ status: step.status as InvestigationStatus })}
                  className="w-fit rounded text-left text-caption text-brand hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                >
                  Move to {step.label}
                </button>
              ) : (
                <span className={`text-caption ${current ? 'font-semibold text-text' : 'text-muted'}`}>
                  {step.label}
                  {current ? ' · now' : ''}
                </span>
              )}
            </li>
          );
        })}
      </ol>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <div className="flex min-w-0 flex-col gap-5">
          <section className="rounded-lg border border-border bg-surface" aria-labelledby="case-findings">
            <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
              <h2 id="case-findings" className="text-body font-semibold text-text">
                Linked findings
                {openFindings.length > 0 ? <span className="ml-2 text-caption font-normal text-muted">{openFindings.length} open</span> : null}
              </h2>
              {editable ? (
                <Link to="/risks/findings" className="text-caption font-semibold text-brand hover:underline">
                  Link a finding
                </Link>
              ) : null}
            </div>
            {findingsLoading && findings.length === 0 ? (
              <p className="px-4 py-3 text-caption text-muted">Loading findings…</p>
            ) : findingsError ? (
              <p className="px-4 py-3 text-caption text-error-foreground" role="alert">
                Linked findings could not be loaded. {findingsError}
              </p>
            ) : findings.length === 0 ? (
              <p className="px-4 py-3 text-caption text-muted">
                No findings yet. Open a finding and use Add to case.
              </p>
            ) : (
              <ul>
                {findings.map((f) => {
                  const entity = activeCase.entities.find((e) => e.type === 'finding' && (e.meta?.insightId === f.insightId || e.href === `#/risks/${f.insightId}`));
                  return (
                    <li key={f.insightId} className="flex items-center gap-3 border-b border-border/60 px-4 py-3 last:border-b-0">
                      <span
                        className={`shrink-0 rounded px-1.5 py-0.5 text-meta font-semibold capitalize ${f.finalLevel ? getSeverityBadgeClass(f.finalLevel) : 'border border-border text-muted'}`}
                      >
                        {f.missing ? 'deleted' : f.finalLevel ?? 'no score'}
                      </span>
                      <span className="min-w-0 flex-1">
                        {f.missing ? (
                          <span className="block truncate font-medium text-muted">{f.title}</span>
                        ) : (
                          <Link to={`/risks/${encodeURIComponent(f.insightId)}`} className="block truncate font-medium text-text hover:text-brand">
                            {f.title}
                          </Link>
                        )}
                        <span className="block truncate font-mono text-meta text-muted">
                          {[f.resourceNamespace, f.resourceName].filter(Boolean).join('/')}
                        </span>
                      </span>
                      <span className="shrink-0 text-caption text-muted">
                        {f.missing ? 'No longer exists' : FINDING_STATUS_LABEL[f.status] ?? f.status}
                      </span>
                      {editable && entity ? (
                        <button
                          type="button"
                          aria-label={`Remove ${f.title} from case`}
                          onClick={() => void unlink(entity.id, f.title)}
                          className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-muted hover:bg-surface-2 hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                        >
                          <X className="h-4 w-4" aria-hidden />
                        </button>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
            )}
            {hiddenFindings > 0 ? (
              <p className="border-t border-border px-4 py-2 text-meta text-muted">
                {hiddenFindings} more linked {hiddenFindings === 1 ? 'finding is' : 'findings are'} in clusters you cannot see.
              </p>
            ) : null}
          </section>

          <section className="rounded-lg border border-border bg-surface p-4">
            <InvestigationTimeline caseId={activeCase.id} refreshKey={timelineKey} />
          </section>
        </div>

        <div className="flex min-w-0 flex-col gap-5">
          <section className="rounded-lg border border-border bg-surface" aria-labelledby="case-assets">
            <h2 id="case-assets" className="border-b border-border px-4 py-3 text-body font-semibold text-text">
              Affected assets
            </h2>
            {assets.length === 0 ? (
              <p className="px-4 py-3 text-caption text-muted">Assets appear here from linked findings, or from pods and attack paths added to the case.</p>
            ) : (
              <ul>
                {assets.map((a) => {
                  const entityId = a.key.startsWith('entity:') ? a.key.slice('entity:'.length) : null;
                  return (
                    <li key={a.key} className="flex flex-wrap items-center gap-3 border-b border-border/60 px-4 py-3 last:border-b-0">
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-medium text-text">{a.name}</span>
                        <span className="block text-meta text-muted">{a.detail}</span>
                      </span>
                      <span className="flex gap-3 text-caption">
                        {a.links.map((l) => (
                          <Link key={l.to} to={l.to} className="text-brand hover:underline">
                            {l.label}
                          </Link>
                        ))}
                      </span>
                      {editable && entityId ? (
                        <button
                          type="button"
                          aria-label={`Remove ${a.name} from case`}
                          onClick={() => void unlink(entityId, a.name)}
                          className="inline-flex h-8 w-8 items-center justify-center rounded-lg text-muted hover:bg-surface-2 hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                        >
                          <X className="h-4 w-4" aria-hidden />
                        </button>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          <section className="rounded-lg border border-border bg-surface p-4" aria-labelledby="case-details">
            <h2 id="case-details" className="mb-3 text-body font-semibold text-text">
              Details
            </h2>
            <dl className="grid grid-cols-[6.5rem_minmax(0,1fr)] items-center gap-x-3 gap-y-2.5 text-caption">
              <dt className="text-muted">Owner</dt>
              <dd>
                {editable ? (
                  <input
                    aria-label="Owner"
                    value={ownerDraft}
                    onChange={(e) => setOwnerDraft(e.target.value)}
                    onBlur={commitOwner}
                    onKeyDown={(e) => e.key === 'Enter' && (e.target as HTMLInputElement).blur()}
                    placeholder="Username"
                    className="w-full rounded-md border border-border bg-base px-2 py-1 text-caption text-text outline-none focus:border-brand"
                  />
                ) : (
                  activeCase.owner || 'Unassigned'
                )}
              </dd>
              <dt className="text-muted">Due</dt>
              <dd>
                {editable ? (
                  <input
                    type="datetime-local"
                    aria-label="Due"
                    defaultValue={activeCase.slaDueAt ? activeCase.slaDueAt.slice(0, 16) : ''}
                    key={activeCase.slaDueAt ?? 'none'}
                    onBlur={(e) => {
                      const next = e.target.value ? new Date(e.target.value).toISOString() : '';
                      if (next.slice(0, 16) !== (activeCase.slaDueAt ?? '').slice(0, 16)) void save({ slaDueAt: next });
                    }}
                    className="w-full rounded-md border border-border bg-base px-2 py-1 text-caption text-text outline-none focus:border-brand"
                  />
                ) : activeCase.slaDueAt ? (
                  formatDateTime(activeCase.slaDueAt)
                ) : (
                  'Not set'
                )}
              </dd>
              <dt className="text-muted">Cluster</dt>
              <dd>{activeCase.clusterId ?? 'All clusters'}</dd>
              <dt className="text-muted">Updated</dt>
              <dd>{formatDateTime(activeCase.updatedAt)}</dd>
            </dl>
          </section>

          <section className="rounded-lg border border-border bg-surface p-4" aria-labelledby="case-tasks">
            <h2 id="case-tasks" className="mb-3 text-body font-semibold text-text">
              Remediation tasks
            </h2>
            {editable ? (
              <div className="mb-3 flex gap-2">
                <input
                  aria-label="New task"
                  value={taskDraft}
                  onChange={(e) => setTaskDraft(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && void addTask()}
                  placeholder="Containment or fix step"
                  className="min-w-0 flex-1 rounded-md border border-border bg-base px-2 py-1.5 text-caption text-text outline-none focus:border-brand"
                />
                <Button size="sm" variant="secondary" onClick={() => void addTask()} disabled={!taskDraft.trim()}>
                  Add
                </Button>
              </div>
            ) : null}
            {tasks.length === 0 ? (
              <p className="text-caption text-muted">No tasks yet.</p>
            ) : (
              <ul className="space-y-2">
                {tasks.map((t) => (
                  <li key={t.id} className="rounded-md border border-border/70 px-3 py-2 text-caption">
                    <div className="flex items-start gap-2">
                      <span className={`min-w-0 flex-1 ${t.status === 'done' ? 'text-muted line-through' : 'text-text'}`}>{t.title}</span>
                      {editable ? (
                        <button
                          type="button"
                          aria-label={`Remove task ${t.title}`}
                          onClick={() => void save({ remediationActions: tasks.filter((x) => x.id !== t.id) })}
                          className="inline-flex h-7 w-7 items-center justify-center rounded text-muted hover:bg-surface-2 hover:text-text"
                        >
                          <Trash2 className="h-3.5 w-3.5" aria-hidden />
                        </button>
                      ) : null}
                    </div>
                    <div className="mt-1.5 flex flex-wrap items-center gap-2">
                      {editable ? (
                        <select
                          aria-label={`Status of ${t.title}`}
                          value={t.status}
                          onChange={(e) => void patchTask(t.id, { status: e.target.value as RemediationActionStatus })}
                          className="rounded border border-border bg-base px-1.5 py-0.5 text-meta"
                        >
                          <option value="pending">To do</option>
                          <option value="in_progress">In progress</option>
                          <option value="blocked">Blocked</option>
                          <option value="done">Done</option>
                        </select>
                      ) : (
                        <span className="text-meta text-muted">{t.status.replace('_', ' ')}</span>
                      )}
                      {t.externalTicketUrl ? (
                        <a href={t.externalTicketUrl} target="_blank" rel="noreferrer noopener" className="inline-flex items-center gap-1 text-meta text-brand hover:underline">
                          Ticket <ExternalLink className="h-3 w-3" aria-hidden />
                        </a>
                      ) : editable ? (
                        <input
                          type="url"
                          aria-label={`Ticket link for ${t.title}`}
                          placeholder="Ticket link"
                          onBlur={(e) => {
                            const url = e.target.value.trim();
                            if (/^https?:\/\//i.test(url)) void patchTask(t.id, { externalTicketUrl: url });
                          }}
                          className="min-w-0 flex-1 rounded border border-border bg-base px-1.5 py-0.5 text-meta"
                        />
                      ) : null}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          {canDelete && activeCase.status !== 'ARCHIVED' ? (
            <Button
              size="sm"
              variant="secondary"
              className="self-start"
              onClick={() => {
                void confirm({
                  title: 'Archive case',
                  description: 'The case leaves every list. It is kept for the retention period, not deleted.',
                  confirmLabel: 'Archive case',
                  variant: 'danger',
                }).then((ok) => {
                  if (ok) void onArchive();
                });
              }}
            >
              <Trash2 className="mr-1 h-4 w-4" aria-hidden />
              Archive case
            </Button>
          ) : null}
        </div>
      </div>

      <Dialog
        open={closing}
        onClose={() => !closeBusy && setClosing(false)}
        closeDisabled={closeBusy}
        size="sm"
        title="Close case"
        description={activeCase.title}
        footer={
          <>
            <Button size="sm" variant="secondary" onClick={() => setClosing(false)} disabled={closeBusy}>
              Cancel
            </Button>
            <Button size="sm" onClick={() => void submitClose()} disabled={closeBusy || closeReason.trim().length < MIN_REASON}>
              {closeBusy ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden /> : null}
              {resolveLinked && openFindings.length > 0 ? `Close and resolve ${openFindings.length}` : 'Close case'}
            </Button>
          </>
        }
      >
        <label className="block text-caption font-semibold text-muted" htmlFor="case-close-reason">
          How was it resolved?
        </label>
        <textarea
          id="case-close-reason"
          value={closeReason}
          onChange={(e) => setCloseReason(e.target.value)}
          className="mt-1 min-h-24 w-full rounded-lg border border-border bg-base px-3 py-2 text-body text-text outline-none focus:border-brand"
          placeholder="The fix or control that ended it."
        />
        <p className="mt-1 text-meta text-muted">At least {MIN_REASON} characters. It is saved to the case and to each finding it resolves.</p>
        {openFindings.length > 0 ? (
          canResolveFindings ? (
            <label className="mt-3 flex items-start gap-2 text-body text-text">
              <input type="checkbox" checked={resolveLinked} onChange={(e) => setResolveLinked(e.target.checked)} className="mt-1 h-4 w-4" />
              <span>
                Resolve the {openFindings.length} linked {openFindings.length === 1 ? 'finding that is' : 'findings that are'} still open
              </span>
            </label>
          ) : (
            <p className="mt-3 text-caption text-muted">
              {openFindings.length} linked {openFindings.length === 1 ? 'finding stays' : 'findings stay'} open: your role cannot resolve findings.
            </p>
          )
        ) : null}
        {closeError ? (
          <p className="mt-3 rounded-lg border border-error-border bg-error-background px-3 py-2 text-caption text-error-foreground" role="alert">
            {closeError}
          </p>
        ) : null}
      </Dialog>
    </PageContainer>
  );
};
