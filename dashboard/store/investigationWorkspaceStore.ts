/**
 * Persistent investigation workspace state — operational continuity across sessions.
 */
import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { emitOperationalEvent } from '../lib/operationalEvents';

export type WorkspacePanel = 'overview' | 'evidence' | 'graph' | 'decisions' | 'remediation' | 'timeline';

export interface GraphScopeState {
  clusterId: string | null;
  focusedEntityId: string | null;
  focusedPathId: string | null;
  highlightedNodeIds: string[];
}

export interface DecisionLogEntry {
  id: string;
  at: string;
  author: string;
  decision: string;
  rationale: string;
  confidence: number;
  factors?: string[];
}

interface CaseWorkspace {
  activePanel: WorkspacePanel;
  graphScope: GraphScopeState;
  decisionLog: DecisionLogEntry[];
  lastVisitedAt: string;
}

interface InvestigationWorkspaceState {
  byCaseId: Record<string, CaseWorkspace>;
  setActivePanel: (caseId: string, panel: WorkspacePanel) => void;
  setGraphScope: (caseId: string, patch: Partial<GraphScopeState>) => void;
  appendDecision: (
    caseId: string,
    entry: Omit<DecisionLogEntry, 'id' | 'at'> & { id?: string; at?: string },
  ) => void;
  touchCase: (caseId: string) => void;
  getWorkspace: (caseId: string) => CaseWorkspace;
}

const DEFAULT_GRAPH_SCOPE: GraphScopeState = {
  clusterId: null,
  focusedEntityId: null,
  focusedPathId: null,
  highlightedNodeIds: [],
};

export const EMPTY_WORKSPACE: CaseWorkspace = {
  activePanel: 'overview',
  graphScope: DEFAULT_GRAPH_SCOPE,
  decisionLog: [],
  lastVisitedAt: '',
};

function defaultWorkspace(): CaseWorkspace {
  return {
    activePanel: 'overview',
    graphScope: { ...DEFAULT_GRAPH_SCOPE },
    decisionLog: [],
    lastVisitedAt: new Date().toISOString(),
  };
}

function newId(): string {
  return `dec_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`;
}

export const useInvestigationWorkspaceStore = create<InvestigationWorkspaceState>()(
  persist(
    (set, get) => ({
      byCaseId: {},
      /** Get the workspace for a case. */
      getWorkspace: (caseId) => get().byCaseId[caseId] ?? EMPTY_WORKSPACE,
      setActivePanel: (caseId, panel) => {
        emitOperationalEvent('workspace:panel_changed', { caseId, panel });
        set((s) => ({
          byCaseId: {
            ...s.byCaseId,
            [caseId]: { ...(s.byCaseId[caseId] ?? defaultWorkspace()), activePanel: panel, lastVisitedAt: new Date().toISOString() },
          },
        }));
      },
      setGraphScope: (caseId, patch) =>
        set((s) => {
          const cur = s.byCaseId[caseId] ?? defaultWorkspace();
          return {
            byCaseId: {
              ...s.byCaseId,
              [caseId]: {
                ...cur,
                graphScope: { ...cur.graphScope, ...patch },
                lastVisitedAt: new Date().toISOString(),
              },
            },
          };
        }),
      /** Append a decision to the log. */
      appendDecision: (caseId, entry) => {
        emitOperationalEvent('workspace:decision_logged', { caseId, decision: entry.decision });
        return set((s) => {
          const cur = s.byCaseId[caseId] ?? defaultWorkspace();
          const row: DecisionLogEntry = {
            id: entry.id ?? newId(),
            at: entry.at ?? new Date().toISOString(),
            author: entry.author,
            decision: entry.decision,
            rationale: entry.rationale,
            confidence: entry.confidence,
            factors: entry.factors,
          };
          return {
            byCaseId: {
              ...s.byCaseId,
              [caseId]: {
                ...cur,
                decisionLog: [row, ...cur.decisionLog].slice(0, 100),
                lastVisitedAt: new Date().toISOString(),
              },
            },
          };
        });
      },
      touchCase: (caseId) =>
        set((s) => ({
          byCaseId: {
            ...s.byCaseId,
            [caseId]: { ...(s.byCaseId[caseId] ?? defaultWorkspace()), lastVisitedAt: new Date().toISOString() },
          },
        })),
    }),
    { name: 'fortuna-investigation-workspace' },
  ),
);
