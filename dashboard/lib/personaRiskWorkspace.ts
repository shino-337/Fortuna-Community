import type { PersonaId } from './persona';

export type RiskTabId = 'overview' | 'triage' | 'pce' | 'reference';

export type RiskFindingsColKey = 'type' | 'resource' | 'score' | 'nsCluster' | 'detected' | 'updated';

export interface RiskWorkspaceConfig {
  visibleTabs: RiskTabId[];
  defaultTab: RiskTabId;
  defaultFindingsRoute: string;
  defaultCols: Record<RiskFindingsColKey, boolean>;
  pageSize: number;
  showBulkToolbar: boolean;
  showRowSelection: boolean;
  showOperatorAnalytics: boolean;
  showSavedViews: boolean;
  narrativeTable: boolean;
  showProvenanceColumn: boolean;
  showTelemetryMeta: boolean;
  bannerTitle: string;
  bannerBody: string;
}

const VIEWER_COLS: Record<RiskFindingsColKey, boolean> = {
  type: false,
  resource: true,
  score: true,
  nsCluster: true,
  detected: false,
  updated: false,
};

const OPERATOR_COLS: Record<RiskFindingsColKey, boolean> = {
  type: true,
  resource: true,
  score: true,
  nsCluster: true,
  detected: true,
  updated: true,
};

const ADMIN_COLS: Record<RiskFindingsColKey, boolean> = { ...OPERATOR_COLS };

const CONFIG: Record<PersonaId, RiskWorkspaceConfig> = {
  viewer: {
    visibleTabs: ['overview', 'triage'],
    defaultTab: 'overview',
    defaultFindingsRoute: '/risks',
    defaultCols: VIEWER_COLS,
    pageSize: 15,
    showBulkToolbar: false,
    showRowSelection: false,
    showOperatorAnalytics: false,
    showSavedViews: false,
    narrativeTable: true,
    showProvenanceColumn: true,
    showTelemetryMeta: false,
    bannerTitle: 'Observer workspace',
    bannerBody: 'Narrative exposure and remediation progress — bulk triage controls are hidden for your role.',
  },
  operator: {
    visibleTabs: ['overview', 'triage', 'pce', 'reference'],
    defaultTab: 'triage',
    defaultFindingsRoute: '/risks/findings',
    defaultCols: OPERATOR_COLS,
    pageSize: 25,
    showBulkToolbar: true,
    showRowSelection: true,
    showOperatorAnalytics: true,
    showSavedViews: true,
    narrativeTable: false,
    showProvenanceColumn: true,
    showTelemetryMeta: false,
    bannerTitle: 'Responder workspace',
    bannerBody: 'Dense triage queue, saved views, and bulk actions — optimized for investigation velocity.',
  },
  admin: {
    visibleTabs: ['overview', 'triage', 'pce', 'reference'],
    defaultTab: 'overview',
    defaultFindingsRoute: '/risks',
    defaultCols: ADMIN_COLS,
    pageSize: 20,
    showBulkToolbar: true,
    showRowSelection: true,
    showOperatorAnalytics: true,
    showSavedViews: true,
    narrativeTable: false,
    showProvenanceColumn: true,
    showTelemetryMeta: true,
    bannerTitle: 'Governance workspace',
    bannerBody: 'Full telemetry context, pipeline health, and investigation oversight.',
  },
  user_admin: {
    visibleTabs: [],
    defaultTab: 'overview',
    defaultFindingsRoute: '/settings',
    defaultCols: VIEWER_COLS,
    pageSize: 15,
    showBulkToolbar: false,
    showRowSelection: false,
    showOperatorAnalytics: false,
    showSavedViews: false,
    narrativeTable: false,
    showProvenanceColumn: false,
    showTelemetryMeta: false,
    bannerTitle: '',
    bannerBody: '',
  },
};

export function getRiskWorkspaceConfig(personaId: PersonaId): RiskWorkspaceConfig {
  return CONFIG[personaId] ?? CONFIG.viewer;
}
