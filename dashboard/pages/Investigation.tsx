import React, { useEffect, useMemo, useState } from 'react';
import { Link, Navigate, useSearchParams } from 'react-router-dom';
import {
  Briefcase,
  Plus,
  Trash2,
  ExternalLink,
  StickyNote,
  Pin,
  ListTodo,
} from 'lucide-react';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Button } from '../components/ui/Button';
import { Card } from '../design-system/components/Card';
import { useConfirm } from '../design-system/components/ConfirmDialog';
import type {
  InvestigationCase,
  InvestigationStatus,
  RemediationActionStatus,
} from '../store/investigationStore';
import { useAuthStore } from '../store/authStore';
import { formatDateTime } from '../lib/display';
import { useInvestigationCases } from '../hooks/useInvestigationCases';
import { InvestigationTimeline } from '../components/InvestigationTimeline';
import { PageContract } from '../components/PageContract';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { InvestigationPresenceStrip } from '../components/InvestigationPresenceStrip';
import { HandoffBanner } from '../components/HandoffBanner';
import { SharedCognitionStrip } from '../components/SharedCognitionStrip';
import { IncidentCommandPanel } from '../components/IncidentCommandPanel';
import { LiveAnnotationsPanel } from '../components/LiveAnnotationsPanel';
import { recordIncidentObservation } from '../lib/operationalMemory';
import { useInvestigationWorkspaceStore, type WorkspacePanel } from '../store/investigationWorkspaceStore';
import { DecisionLogPanel } from '../components/investigation/DecisionLogPanel';
import { WorkspaceGraphPanel } from '../components/investigation/WorkspaceGraphPanel';
import { PAGE_TITLES } from '../lib/pageTitles';
import clsx from 'clsx';

const STATUS_OPTIONS: { value: InvestigationStatus; label: string }[] = [
  { value: 'OPEN', label: 'Open' },
  { value: 'TRIAGED', label: 'Triaged' },
  { value: 'ACTIVE', label: 'Active' },
  { value: 'CONTAINED', label: 'Contained' },
  { value: 'REMEDIATING', label: 'Remediating' },
  { value: 'RESOLVED', label: 'Resolved' },
  { value: 'ARCHIVED', label: 'Archived' },
];

const EXTERNAL_SYSTEMS = ['', 'jira', 'servicenow', 'gitops'] as const;

function entityTypeLabel(type: string): string {
  return type.replace(/_/g, ' ');
}

export const Investigation: React.FC = () => {
  const { user } = useAuthStore();
  const confirm = useConfirm();
  const {
    cases,
    loading,
    error,
    canRead,
    canWrite,
    canDelete,
    activeCaseId,
    setActiveCase,
    refresh,
    createCase,
    updateCase,
    deleteCase,
  } = useInvestigationCases();
  const [searchParams, setSearchParams] = useSearchParams();

  const [noteDraft, setNoteDraft] = useState('');
  const [remediationDraft, setRemediationDraft] = useState('');
  const [assigneesDraft, setAssigneesDraft] = useState('');
  const [watchersDraft, setWatchersDraft] = useState('');
  const [handoffDraft, setHandoffDraft] = useState('');

  useEffect(() => {
    const caseId = searchParams.get('case');
    if (caseId && cases.some((c) => c.id === caseId)) {
      setActiveCase(caseId);
    }
  }, [searchParams, cases, setActiveCase]);

  useEffect(() => {
    if (!activeCaseId) return;
    const current = searchParams.get('case');
    if (current === activeCaseId) return;
    setSearchParams({ case: activeCaseId }, { replace: true });
  }, [activeCaseId, searchParams, setSearchParams]);

  const activeCase = useMemo(
    () => cases.find((c) => c.id === activeCaseId) ?? cases[0] ?? null,
    [cases, activeCaseId],
  );

  const workspacePanel = useInvestigationWorkspaceStore((s) =>
    activeCase ? s.getWorkspace(activeCase.id).activePanel : 'overview',
  );
  const setWorkspacePanel = useInvestigationWorkspaceStore((s) => s.setActivePanel);
  const touchWorkspace = useInvestigationWorkspaceStore((s) => s.touchCase);

  useEffect(() => {
    if (!activeCase) return;
    touchWorkspace(activeCase.id);
    recordIncidentObservation({
      clusterId: activeCase.clusterId,
      owner: activeCase.owner,
      hadRemediation: (activeCase.remediationActions?.length ?? 0) > 0,
      resolved: activeCase.status === 'RESOLVED' || activeCase.status === 'ARCHIVED',
      entityCount: activeCase.entities.length,
    });
  }, [activeCase?.id, activeCase?.status, activeCase?.entities.length, touchWorkspace]);

  const author = user?.username ?? user?.email ?? 'analyst';

  if (!canRead) {
    return <Navigate to="/" replace />;
  }

  const casesEmpty = !loading && cases.length === 0;

  const patchActive = (patch: Partial<InvestigationCase>) => {
    if (!activeCase || !canWrite) return;
    void updateCase(activeCase.id, { ...activeCase, ...patch });
  };

  const exportSummary = (c: InvestigationCase) => {
    const lines = [
      `# ${c.title}`,
      `Status: ${c.status}`,
      `Owner: ${c.owner || '—'}`,
      `Cluster: ${c.clusterId ?? 'all'}`,
      `Updated: ${c.updatedAt}`,
      '',
      '## Pinned entities',
      ...c.entities.map(
        (e) =>
          `- [${e.type}] ${e.label}${e.href ? ` (${e.href.replace(/^#/, window.location.origin + window.location.pathname)})` : ''}`,
      ),
      '',
      '## Remediation',
      ...(c.remediationActions ?? []).map(
        (a) => `- [${a.status}] ${a.title}${a.owner ? ` (@${a.owner})` : ''}${a.dueAt ? ` due ${a.dueAt}` : ''}`,
      ),
      '',
      '## Notes',
      ...c.notes.map((n) => `- ${n.createdAt} (${n.author}): ${n.body}`),
    ];
    const blob = new Blob([lines.join('\n')], { type: 'text/markdown;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `investigation-${c.id}.md`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <PageContract
      feature="investigation"
      loading={loading}
    >
    <PageLayout
      title={PAGE_TITLES.investigations}
      description="Server-persisted incident context scoped to your role and cluster access. Viewers can read shared cases; operators can edit cases they own."
    >
      {error ? (
        <div className="mb-4 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-caption text-amber-100 flex justify-between gap-2">
          <span>{error}</span>
          <Button type="button" size="sm" variant="secondary" onClick={() => void refresh()}>
            Retry
          </Button>
        </div>
      ) : null}
      <InvestigationPresenceStrip />
      <SharedCognitionStrip />
      <HandoffBanner />
      {loading ? <p className="text-caption text-muted mb-4">Loading cases…</p> : null}
      {!canWrite ? (
        <p className="text-caption text-muted mb-4 rounded-lg border border-border bg-surface/40 px-3 py-2">
          Read-only: your role has <code className="text-brand">investigations.read</code> but not write. Pin and edit actions are hidden.
        </p>
      ) : null}
      <div className="grid gap-4 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)]">
        <Card variant="secondary" className="p-3 space-y-2 h-fit">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-caption font-semibold text-muted uppercase tracking-wider flex items-center gap-1.5">
              <Briefcase className="w-4 h-4" aria-hidden />
              Cases
            </h2>
            <Button
              type="button"
              size="sm"
              variant="secondary"
              disabled={!canWrite}
              onClick={() => void createCase({ owner: author })}
            >
              <Plus className="w-4 h-4 mr-1" />
              New
            </Button>
          </div>
          {cases.length === 0 ? (
            <SemanticEmptyState
              state="no_data"
              compact
              title="No cases in this workspace"
              reason="Pin a finding or attack path from Risk Operations or Attack Analysis, or create a case here."
            />
          ) : (
            <ul className="space-y-1 max-h-[min(70vh,32rem)] overflow-y-auto">
              {cases.map((c) => (
                <li key={c.id}>
                  <button
                    type="button"
                    onClick={() => setActiveCase(c.id)}
                    className={`w-full text-left rounded-md border px-2 py-2 text-caption transition-colors ${
                      activeCase?.id === c.id
                        ? 'border-brand bg-brand/10 text-text'
                        : 'border-border text-muted hover:border-muted hover:text-text'
                    }`}
                  >
                    <div className="font-semibold truncate">{c.title}</div>
                    <div className="text-meta mt-0.5 flex justify-between gap-1">
                      <span>{c.status.replace(/_/g, ' ')}</span>
                      <span>{c.entities.length} pins</span>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Card>

        {activeCase ? (
          <div className="space-y-4 min-w-0">
            <Card variant="primary" className="p-4 space-y-4">
              <div className="flex flex-wrap gap-3 items-start justify-between">
                <div className="min-w-0 flex-1 space-y-2">
                  <label className="block text-meta text-muted uppercase tracking-wide">Title</label>
                  <input
                    type="text"
                    value={activeCase.title}
                    onChange={(e) => patchActive({ title: e.target.value })}
                    disabled={!canWrite}
                    className="w-full rounded-md border border-border bg-base px-3 py-2 text-body text-text disabled:opacity-60"
                  />
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button type="button" variant="secondary" size="sm" onClick={() => exportSummary(activeCase)}>
                    Export handoff
                  </Button>
                  {canDelete ? (
                    <Button
                      type="button"
                      variant="secondary"
                      size="sm"
                      onClick={() => {
                        void confirm({
                          title: 'Archive investigation case',
                          description: 'This case will be soft-deleted with retention, not permanently removed.',
                          confirmLabel: 'Archive case',
                          variant: 'danger',
                        }).then((confirmed) => {
                          if (confirmed) void deleteCase(activeCase.id);
                        });
                      }}
                    >
                      Archive
                      <Trash2 className="w-4 h-4 ml-1" />
                    </Button>
                  ) : null}
                </div>
              </div>

              <div className="grid gap-3 sm:grid-cols-3">
                <div>
                  <label className="text-meta text-muted uppercase tracking-wide">Status</label>
                  <select
                    value={activeCase.status}
                    onChange={(e) => patchActive({ status: e.target.value as InvestigationStatus })}
                    disabled={!canWrite}
                    className="mt-1 w-full rounded-md border border-border bg-base px-2 py-1.5 text-caption disabled:opacity-60"
                  >
                    {STATUS_OPTIONS.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="text-meta text-muted uppercase tracking-wide">Owner</label>
                  <input
                    type="text"
                    value={activeCase.owner}
                    onChange={(e) => patchActive({ owner: e.target.value })}
                    disabled={!canWrite}
                    className="mt-1 w-full rounded-md border border-border bg-base px-2 py-1.5 text-caption disabled:opacity-60"
                    placeholder="Assignee"
                  />
                </div>
                <div>
                  <label className="text-meta text-muted uppercase tracking-wide">Case SLA due</label>
                  <input
                    type="datetime-local"
                    value={activeCase.slaDueAt ? activeCase.slaDueAt.slice(0, 16) : ''}
                    onChange={(e) =>
                      patchActive({
                        slaDueAt: e.target.value
                          ? new Date(e.target.value).toISOString()
                          : undefined,
                      })
                    }
                    disabled={!canWrite}
                    className="mt-1 w-full rounded-md border border-border bg-base px-2 py-1.5 text-caption disabled:opacity-60"
                  />
                </div>
              </div>
              <p className="text-caption text-muted">
                Scope cluster: <span className="text-text">{activeCase.clusterId ?? 'All clusters'}</span>
              </p>
              <p className="text-meta text-muted">
                Created {formatDateTime(activeCase.createdAt)} · Updated {formatDateTime(activeCase.updatedAt)}
              </p>
            </Card>

            <nav className="flex gap-1 overflow-x-auto rounded-lg border border-border/80 bg-surface/40 p-1" aria-label="Workspace panels">
              {(
                [
                  ['overview', 'Overview'],
                  ['evidence', 'Evidence'],
                  ['graph', 'Graph'],
                  ['decisions', 'Decisions'],
                  ['remediation', 'Remediation'],
                  ['timeline', 'Timeline'],
                ] as [WorkspacePanel, string][]
              ).map(([id, label]) => (
                <button
                  key={id}
                  type="button"
                  onClick={() => setWorkspacePanel(activeCase.id, id)}
                  className={clsx(
                    'shrink-0 rounded-md px-3 py-1.5 text-caption font-semibold',
                    workspacePanel === id ? 'bg-brand/15 text-brand' : 'text-muted hover:text-text',
                  )}
                >
                  {label}
                </button>
              ))}
            </nav>

            {(workspacePanel === 'overview' || workspacePanel === 'evidence') && (
            <Card variant="secondary" className="p-4">
              <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-3 flex items-center gap-2">
                <Pin className="w-4 h-4" />
                Pinned entities ({activeCase.entities.length})
              </h3>
              {activeCase.entities.length === 0 ? (
                <p className="text-caption text-muted">
                  Pin findings from Risk Operations, attack scenarios from Attack Analysis, or pods from Resources.
                </p>
              ) : (
                <ul className="divide-y divide-border rounded-md border border-border overflow-hidden">
                  {activeCase.entities.map((e) => (
                    <li
                      key={e.id}
                      className="flex flex-wrap items-center justify-between gap-2 px-3 py-2 bg-base/40 text-caption"
                    >
                      <div className="min-w-0">
                        <span className="text-meta text-muted uppercase mr-2">{entityTypeLabel(e.type)}</span>
                        <span className="font-medium text-text">{e.label}</span>
                        {e.meta && Object.keys(e.meta).length > 0 ? (
                          <span className="block text-meta text-muted mt-0.5 truncate">
                            {Object.entries(e.meta)
                              .map(([k, v]) => `${k}: ${v}`)
                              .join(' · ')}
                          </span>
                        ) : null}
                        {e.snapshot?.capturedAt ? (
                          <span className="block text-meta text-emerald-400/90 mt-0.5">
                            Snapshot {formatDateTime(e.snapshot.capturedAt)}
                          </span>
                        ) : null}
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        {e.href ? (
                          <Link
                            to={e.href.replace(/^#/, '')}
                            className="text-brand hover:underline inline-flex items-center gap-1"
                          >
                            Open <ExternalLink className="w-3.5 h-3.5" />
                          </Link>
                        ) : null}
                        <button
                          type="button"
                          className="inline-flex min-h-10 min-w-10 items-center justify-center rounded-lg text-muted transition-colors hover:bg-critical/10 hover:text-critical focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-8 sm:min-w-8"
                          aria-label={`Remove ${e.label || e.id} from case`}
                          onClick={() =>
                            patchActive({ entities: activeCase.entities.filter((x) => x.id !== e.id) })
                          }
                          disabled={!canWrite}
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
              <div className="mt-3 flex flex-wrap gap-2 text-caption">
                <Link to="/risks/findings" className="text-brand hover:underline">
                  Triage queue
                </Link>
                <span className="text-muted">·</span>
                <Link to="/attack-paths" className="text-brand hover:underline">
                  Attack paths
                </Link>
                <span className="text-muted">·</span>
                <Link to="/resources" className="text-brand hover:underline">
                  Resources
                </Link>
              </div>
            </Card>
            )}

            {workspacePanel === 'graph' && (
            <Card variant="secondary" className="p-4">
              <WorkspaceGraphPanel activeCase={activeCase} />
            </Card>
            )}

            {workspacePanel === 'decisions' && (
            <Card variant="secondary" className="p-4">
              <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-3">Decision log</h3>
              <DecisionLogPanel caseId={activeCase.id} author={author} canWrite={canWrite} />
            </Card>
            )}

            {(workspacePanel === 'overview' || workspacePanel === 'remediation') && (
            <Card variant="secondary" className="p-4">
              <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-3 flex items-center gap-2">
                <ListTodo className="w-4 h-4" />
                Remediation tracker
              </h3>
              <div className="flex flex-wrap gap-2 mb-3">
                <input
                  type="text"
                  value={remediationDraft}
                  onChange={(e) => setRemediationDraft(e.target.value)}
                  placeholder="Containment or fix action…"
                  className="flex-1 min-w-[12rem] rounded-md border border-border bg-base px-3 py-2 text-caption"
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && remediationDraft.trim()) {
                      const action = {
                        id: `rem_${Date.now()}`,
                        title: remediationDraft.trim(),
                        status: 'pending' as RemediationActionStatus,
                        owner: author,
                        createdAt: new Date().toISOString(),
                      };
                      patchActive({
                        remediationActions: [action, ...(activeCase.remediationActions ?? [])],
                      });
                      setRemediationDraft('');
                    }
                  }}
                />
                <Button
                  type="button"
                  size="sm"
                  disabled={!remediationDraft.trim()}
                  onClick={() => {
                    const action = {
                      id: `rem_${Date.now()}`,
                      title: remediationDraft.trim(),
                      status: 'pending' as RemediationActionStatus,
                      owner: author,
                      createdAt: new Date().toISOString(),
                    };
                    patchActive({
                      remediationActions: [action, ...(activeCase.remediationActions ?? [])],
                    });
                    setRemediationDraft('');
                  }}
                >
                  Add action
                </Button>
              </div>
              {(activeCase.remediationActions ?? []).length === 0 ? (
                <p className="text-caption text-muted">No remediation steps tracked yet.</p>
              ) : (
                <ul className="space-y-2">
                  {(activeCase.remediationActions ?? []).map((action) => (
                    <li
                      key={action.id}
                      className="rounded-md border border-border bg-base/30 px-3 py-2 flex flex-wrap gap-2 items-center"
                    >
                      <input
                        type="text"
                        value={action.title}
                        onChange={(e) =>
                          patchActive({
                            remediationActions: (activeCase.remediationActions ?? []).map((a) =>
                              a.id === action.id ? { ...a, title: e.target.value } : a,
                            ),
                          })
                        }
                        disabled={!canWrite}
                        className="flex-1 min-w-[10rem] rounded border border-border bg-base px-2 py-1 text-caption disabled:opacity-60"
                      />
                      <select
                        value={action.status}
                        onChange={(e) =>
                          patchActive({
                            remediationActions: (activeCase.remediationActions ?? []).map((a) =>
                              a.id === action.id
                                ? { ...a, status: e.target.value as RemediationActionStatus }
                                : a,
                            ),
                          })
                        }
                        disabled={!canWrite}
                        className="rounded border border-border bg-base px-2 py-1 text-caption disabled:opacity-60"
                      >
                        <option value="pending">Pending</option>
                        <option value="in_progress">In progress</option>
                        <option value="done">Done</option>
                        <option value="blocked">Blocked</option>
                      </select>
                      <select
                        value={action.externalSystem ?? ''}
                        onChange={(e) =>
                          patchActive({
                            remediationActions: (activeCase.remediationActions ?? []).map((a) =>
                              a.id === action.id
                                ? { ...a, externalSystem: e.target.value || undefined }
                                : a,
                            ),
                          })
                        }
                        disabled={!canWrite}
                        className="rounded border border-border bg-base px-2 py-1 text-caption disabled:opacity-60"
                        title="External ticketing"
                      >
                        {EXTERNAL_SYSTEMS.map((s) => (
                          <option key={s || 'none'} value={s}>
                            {s ? s.toUpperCase() : 'No ticket'}
                          </option>
                        ))}
                      </select>
                      <input
                        type="url"
                        value={action.externalTicketUrl ?? ''}
                        onChange={(e) =>
                          patchActive({
                            remediationActions: (activeCase.remediationActions ?? []).map((a) =>
                              a.id === action.id
                                ? { ...a, externalTicketUrl: e.target.value || undefined }
                                : a,
                            ),
                          })
                        }
                        disabled={!canWrite}
                        placeholder="Ticket URL"
                        className="min-w-[8rem] rounded border border-border bg-base px-2 py-1 text-caption disabled:opacity-60"
                      />
                      <button
                        type="button"
                        className="inline-flex min-h-10 min-w-10 items-center justify-center rounded-lg text-muted transition-colors hover:bg-critical/10 hover:text-critical focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-8 sm:min-w-8"
                        aria-label={`Remove remediation action ${action.title || action.id}`}
                        onClick={() =>
                          patchActive({
                            remediationActions: (activeCase.remediationActions ?? []).filter(
                              (a) => a.id !== action.id,
                            ),
                          })
                        }
                        disabled={!canWrite}
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </Card>
            )}

            {workspacePanel === 'overview' && (
            <>
            <IncidentCommandPanel activeCase={activeCase} canWrite={canWrite} />
            <Card variant="secondary" className="p-4">
              <LiveAnnotationsPanel caseId={activeCase.id} author={author} canWrite={canWrite} />
            </Card>
            <Card variant="secondary" className="p-4 space-y-4">
              <h3 className="text-caption font-semibold text-muted uppercase tracking-wider">
                Collaboration
              </h3>
              <div className="grid gap-3 sm:grid-cols-2">
                <div>
                  <label className="text-meta text-muted uppercase tracking-wide">Assignees</label>
                  <input
                    type="text"
                    value={assigneesDraft || (activeCase.collaboration?.assignees ?? []).join(', ')}
                    onChange={(e) => setAssigneesDraft(e.target.value)}
                    onBlur={() => {
                      const list = (assigneesDraft || '')
                        .split(',')
                        .map((s) => s.trim())
                        .filter(Boolean);
                      patchActive({
                        collaboration: {
                          ...activeCase.collaboration,
                          assignees: list,
                          watchers: activeCase.collaboration?.watchers ?? [],
                          handoffNotes: activeCase.collaboration?.handoffNotes ?? [],
                        },
                      });
                      setAssigneesDraft('');
                    }}
                    disabled={!canWrite}
                    placeholder="user1, user2"
                    className="mt-1 w-full rounded-md border border-border bg-base px-2 py-1.5 text-caption disabled:opacity-60"
                  />
                </div>
                <div>
                  <label className="text-meta text-muted uppercase tracking-wide">Watchers</label>
                  <input
                    type="text"
                    value={watchersDraft || (activeCase.collaboration?.watchers ?? []).join(', ')}
                    onChange={(e) => setWatchersDraft(e.target.value)}
                    onBlur={() => {
                      const list = (watchersDraft || '')
                        .split(',')
                        .map((s) => s.trim())
                        .filter(Boolean);
                      patchActive({
                        collaboration: {
                          ...activeCase.collaboration,
                          assignees: activeCase.collaboration?.assignees ?? [],
                          watchers: list,
                          handoffNotes: activeCase.collaboration?.handoffNotes ?? [],
                        },
                      });
                      setWatchersDraft('');
                    }}
                    disabled={!canWrite}
                    placeholder="soc-lead, on-call"
                    className="mt-1 w-full rounded-md border border-border bg-base px-2 py-1.5 text-caption disabled:opacity-60"
                  />
                </div>
              </div>
              {canWrite ? (
                <div className="flex flex-col sm:flex-row gap-2">
                  <textarea
                    value={handoffDraft}
                    onChange={(e) => setHandoffDraft(e.target.value)}
                    rows={2}
                    placeholder="Handoff note with @mentions…"
                    className="flex-1 rounded-md border border-border bg-base px-3 py-2 text-caption resize-y min-h-[3rem]"
                  />
                  <Button
                    type="button"
                    size="sm"
                    className="self-end"
                    disabled={!handoffDraft.trim()}
                    onClick={() => {
                      const mentions = [...handoffDraft.matchAll(/@([\w.-]+)/g)].map((m) => m[1]);
                      const note = {
                        id: `handoff_${Date.now()}`,
                        body: handoffDraft.trim(),
                        author,
                        createdAt: new Date().toISOString(),
                        mentions,
                      };
                      patchActive({
                        collaboration: {
                          ...activeCase.collaboration,
                          assignees: activeCase.collaboration?.assignees ?? [],
                          watchers: activeCase.collaboration?.watchers ?? [],
                          handoffNotes: [note, ...(activeCase.collaboration?.handoffNotes ?? [])],
                        },
                      });
                      setHandoffDraft('');
                    }}
                  >
                    Add handoff
                  </Button>
                </div>
              ) : null}
              {(activeCase.collaboration?.handoffNotes ?? []).length > 0 ? (
                <ul className="space-y-2">
                  {(activeCase.collaboration?.handoffNotes ?? []).map((n) => (
                    <li key={n.id} className="rounded-md border border-border bg-base/30 px-3 py-2 text-caption">
                      <div className="text-meta text-muted mb-1">
                        {formatDateTime(n.createdAt)} · {n.author}
                        {n.mentions?.length ? ` · @${n.mentions.join(', @')}` : ''}
                      </div>
                      <p className="text-text whitespace-pre-wrap">{n.body}</p>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-caption text-muted">No handoff notes yet.</p>
              )}
            </Card>
            </>
            )}

            {(workspacePanel === 'overview' || workspacePanel === 'timeline') && (
            <Card variant="secondary" className="p-4">
              <InvestigationTimeline caseId={activeCase.id} />
            </Card>
            )}

            {workspacePanel === 'overview' && (
            <Card variant="secondary" className="p-4">
              <h3 className="text-caption font-semibold text-muted uppercase tracking-wider mb-3 flex items-center gap-2">
                <StickyNote className="w-4 h-4" />
                Notes
              </h3>
              {canWrite ? (
              <div className="flex flex-col sm:flex-row gap-2 mb-3">
                <textarea
                  value={noteDraft}
                  onChange={(e) => setNoteDraft(e.target.value)}
                  rows={2}
                  placeholder="Handoff note, hypothesis, or next action…"
                  className="flex-1 rounded-md border border-border bg-base px-3 py-2 text-caption text-text resize-y min-h-[4rem]"
                />
                <Button
                  type="button"
                  size="sm"
                  className="self-end sm:self-auto"
                  onClick={() => {
                    const note = {
                      id: `note_${Date.now()}`,
                      body: noteDraft.trim(),
                      createdAt: new Date().toISOString(),
                      author,
                    };
                    patchActive({ notes: [note, ...activeCase.notes] });
                    setNoteDraft('');
                  }}
                  disabled={!noteDraft.trim()}
                >
                  Add note
                </Button>
              </div>
              ) : null}
              {activeCase.notes.length === 0 ? (
                <p className="text-caption text-muted">No notes yet.</p>
              ) : (
                <ul className="space-y-2">
                  {activeCase.notes.map((n) => (
                    <li key={n.id} className="rounded-md border border-border bg-base/30 px-3 py-2 text-caption">
                      <div className="text-meta text-muted mb-1">
                        {formatDateTime(n.createdAt)} · {n.author}
                      </div>
                      <p className="text-text whitespace-pre-wrap">{n.body}</p>
                    </li>
                  ))}
                </ul>
              )}
            </Card>
            )}
          </div>
        ) : (
          <Card variant="secondary" className="p-8 text-center text-muted">
            <Briefcase className="w-10 h-10 mx-auto mb-3 opacity-50" />
            <p className="text-body">Create a case to start an investigation workspace.</p>
          </Card>
        )}
      </div>
    </PageLayout>
    </PageContract>
  );
};
