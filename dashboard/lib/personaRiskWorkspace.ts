import type { PersonaId } from './persona';

/** The Findings queue plus its secondary view (runtime evidence). */
export type RiskTabId = 'triage' | 'reference';

export type RiskFindingsColKey = 'type' | 'resource' | 'evidence' | 'nsCluster' | 'detected' | 'updated';

export interface RiskWorkspaceConfig {
  visibleTabs: RiskTabId[];
  defaultTab: RiskTabId;
  defaultCols: Record<RiskFindingsColKey, boolean>;
  pageSize: number;
  showBulkToolbar: boolean;
  showRowSelection: boolean;
  showSavedViews: boolean;
  narrativeTable: boolean;
}

const VIEWER_COLS: Record<RiskFindingsColKey, boolean> = {
  type: false,
  resource: true,
  evidence: false,
  nsCluster: true,
  detected: false,
  updated: false,
};

const OPERATOR_COLS: Record<RiskFindingsColKey, boolean> = {
  type: true,
  resource: true,
  evidence: false,
  nsCluster: true,
  detected: true,
  updated: false,
};

const ADMIN_COLS: Record<RiskFindingsColKey, boolean> = { ...OPERATOR_COLS };

const CONFIG: Record<PersonaId, RiskWorkspaceConfig> = {
  viewer: {
    visibleTabs: ['triage'],
    defaultTab: 'triage',
    defaultCols: VIEWER_COLS,
    pageSize: 15,
    showBulkToolbar: false,
    showRowSelection: false,
    showSavedViews: false,
    narrativeTable: true,
  },
  operator: {
    visibleTabs: ['triage', 'reference'],
    defaultTab: 'triage',
    defaultCols: OPERATOR_COLS,
    pageSize: 25,
    showBulkToolbar: true,
    showRowSelection: true,
    showSavedViews: true,
    narrativeTable: false,
  },
  admin: {
    visibleTabs: ['triage', 'reference'],
    defaultTab: 'triage',
    defaultCols: ADMIN_COLS,
    pageSize: 20,
    showBulkToolbar: true,
    showRowSelection: true,
    showSavedViews: true,
    narrativeTable: false,
  },
};

export function getRiskWorkspaceConfig(personaId: PersonaId): RiskWorkspaceConfig {
  return CONFIG[personaId] ?? CONFIG.viewer;
}
