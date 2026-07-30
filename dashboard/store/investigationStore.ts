import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type InvestigationEntityType =
  | 'finding'
  | 'pod'
  | 'attack_path'
  | 'identity'
  | 'resource'
  | 'link';

export type InvestigationStatus =
  | 'OPEN'
  | 'TRIAGED'
  | 'ACTIVE'
  | 'CONTAINED'
  | 'REMEDIATING'
  | 'RESOLVED'
  | 'ARCHIVED';

export interface InvestigationEntitySnapshot {
  capturedAt: string;
  entity?: Record<string, unknown>;
  evidence?: Record<string, unknown>;
  score?: Record<string, unknown>;
  graph?: Record<string, unknown>;
}

export interface InvestigationEntity {
  id: string;
  type: InvestigationEntityType;
  label: string;
  href?: string;
  meta?: Record<string, string>;
  pinnedAt: string;
  snapshot?: InvestigationEntitySnapshot;
}

export interface InvestigationNote {
  id: string;
  body: string;
  createdAt: string;
  author: string;
}

export type RemediationActionStatus = 'pending' | 'in_progress' | 'done' | 'blocked';

export interface RemediationAction {
  id: string;
  title: string;
  status: RemediationActionStatus;
  owner: string;
  dueAt?: string;
  notes?: string;
  createdAt: string;
  externalSystem?: string;
  externalTicketUrl?: string;
  externalTicketKey?: string;
}

export interface HandoffNote {
  id: string;
  body: string;
  author: string;
  createdAt: string;
  mentions?: string[];
}

export interface InvestigationCollaboration {
  assignees: string[];
  watchers: string[];
  teamId?: string;
  handoffNotes: HandoffNote[];
}

export interface InvestigationCase {
  id: string;
  title: string;
  status: InvestigationStatus;
  owner: string;
  clusterId: string | null;
  notes: InvestigationNote[];
  entities: InvestigationEntity[];
  remediationActions: RemediationAction[];
  collaboration: InvestigationCollaboration;
  createdAt: string;
  updatedAt: string;
  slaDueAt?: string;
  archivedAt?: string;
  retentionUntil?: string;
}

interface InvestigationState {
  cases: InvestigationCase[];
  activeCaseId: string | null;
  createCase: (partial?: { title?: string; owner?: string; clusterId?: string | null }) => string;
  setActiveCase: (id: string | null) => void;
  updateCase: (id: string, patch: Partial<Pick<InvestigationCase, 'title' | 'status' | 'owner' | 'clusterId' | 'slaDueAt'>>) => void;
  addEntity: (
    caseId: string,
    entity: Omit<InvestigationEntity, 'pinnedAt' | 'id'> & { id?: string },
  ) => boolean;
  removeEntity: (caseId: string, entityId: string) => void;
  addNote: (caseId: string, body: string, author: string) => void;
  addRemediation: (caseId: string, partial: { title: string; owner?: string; dueAt?: string }) => void;
  updateRemediation: (
    caseId: string,
    actionId: string,
    patch: Partial<Pick<RemediationAction, 'title' | 'status' | 'owner' | 'dueAt' | 'notes'>>,
  ) => void;
  removeRemediation: (caseId: string, actionId: string) => void;
  deleteCase: (id: string) => void;
}

function newId(prefix: string): string {
  return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

export const useInvestigationStore = create<InvestigationState>()(
  persist(
    (set, get) => ({
      cases: [],
      activeCaseId: null,

      /** Create a new investigation case. */
      createCase: (partial) => {
        const now = new Date().toISOString();
        const id = newId('case');
        const next: InvestigationCase = {
          id,
          title: partial?.title?.trim() || `Investigation ${new Date().toLocaleString()}`,
          status: 'OPEN',
          owner: partial?.owner?.trim() || '',
          clusterId: partial?.clusterId ?? null,
          notes: [],
          entities: [],
          remediationActions: [],
          collaboration: { assignees: [], watchers: [], handoffNotes: [] },
          createdAt: now,
          updatedAt: now,
        };
        set((s) => ({
          cases: [next, ...s.cases],
          activeCaseId: id,
        }));
        return id;
      },

      setActiveCase: (id) => set({ activeCaseId: id }),

      /** Update a case. */
      updateCase: (id, patch) => {
        set((s) => ({
          cases: s.cases.map((c) =>
            c.id === id ? { ...c, ...patch, updatedAt: new Date().toISOString() } : c,
          ),
        }));
      },

      /** Add an entity to a case. */
      addEntity: (caseId, entity) => {
        const c = get().cases.find((x) => x.id === caseId);
        if (!c) return false;
        const entityKey = entity.id ?? `${entity.type}:${entity.label}`;
        if (c.entities.some((e) => e.id === entityKey || (e.href && e.href === entity.href))) {
          return false;
        }
        const pinned: InvestigationEntity = {
          ...entity,
          id: entityKey,
          pinnedAt: new Date().toISOString(),
        };
        set((s) => ({
          cases: s.cases.map((x) =>
            x.id === caseId
              ? {
                  ...x,
                  entities: [pinned, ...x.entities],
                  updatedAt: new Date().toISOString(),
                }
              : x,
          ),
        }));
        return true;
      },

      removeEntity: (caseId, entityId) => {
        set((s) => ({
          cases: s.cases.map((c) =>
            c.id === caseId
              ? {
                  ...c,
                  entities: c.entities.filter((e) => e.id !== entityId),
                  updatedAt: new Date().toISOString(),
                }
              : c,
          ),
        }));
      },

      /** Add a note to a case. */
      addNote: (caseId, body, author) => {
        const trimmed = body.trim();
        if (!trimmed) return;
        const note: InvestigationNote = {
          id: newId('note'),
          body: trimmed,
          createdAt: new Date().toISOString(),
          author,
        };
        set((s) => ({
          cases: s.cases.map((c) =>
            c.id === caseId
              ? { ...c, notes: [note, ...c.notes], updatedAt: new Date().toISOString() }
              : c,
          ),
        }));
      },

      /** Add a remediation action to a case. */
      addRemediation: (caseId, partial) => {
        const action: RemediationAction = {
          id: newId('rem'),
          title: partial.title.trim(),
          status: 'pending',
          owner: partial.owner?.trim() || '',
          dueAt: partial.dueAt,
          createdAt: new Date().toISOString(),
        };
        if (!action.title) return;
        set((s) => ({
          cases: s.cases.map((c) =>
            c.id === caseId
              ? {
                  ...c,
                  remediationActions: [...(c.remediationActions ?? []), action],
                  updatedAt: new Date().toISOString(),
                }
              : c,
          ),
        }));
      },

      /** Update a remediation action. */
      updateRemediation: (caseId, actionId, patch) => {
        set((s) => ({
          cases: s.cases.map((c) =>
            c.id === caseId
              ? {
                  ...c,
                  remediationActions: (c.remediationActions ?? []).map((a) =>
                    a.id === actionId ? { ...a, ...patch } : a,
                  ),
                  updatedAt: new Date().toISOString(),
                }
              : c,
          ),
        }));
      },

      removeRemediation: (caseId, actionId) => {
        set((s) => ({
          cases: s.cases.map((c) =>
            c.id === caseId
              ? {
                  ...c,
                  remediationActions: (c.remediationActions ?? []).filter((a) => a.id !== actionId),
                  updatedAt: new Date().toISOString(),
                }
              : c,
          ),
        }));
      },

      deleteCase: (id) => {
        set((s) => {
          const cases = s.cases.filter((c) => c.id !== id);
          const activeCaseId =
            s.activeCaseId === id ? (cases[0]?.id ?? null) : s.activeCaseId;
          return { cases, activeCaseId };
        });
      },
    }),
    {
      name: 'fortuna-investigations-v2',
      partialize: (s) => ({ cases: s.cases, activeCaseId: s.activeCaseId }),
    },
  ),
);
