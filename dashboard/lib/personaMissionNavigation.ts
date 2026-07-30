/**
 * Mission-centric navigation — derived only from materialized operational surfaces.
 */
import type { MaterializedOperationalPlane } from './routeMaterialization';
import type { PersonaId } from './persona';
import { PAGE_TITLES } from './pageTitles';

export interface MissionNavItem {
  id: string;
  label: string;
  path: string;
  search?: string;
  iconKey: string;
  highlight?: boolean;
}

export interface MissionNavSection {
  title: string;
  items: MissionNavItem[];
}

const MISSION_CATALOG: Record<
  string,
  { label: string; path: string; iconKey: string; search?: string; highlight?: boolean }
> = {
  home: { label: PAGE_TITLES.homeViewer, path: '/', iconKey: 'dashboard' },
  dashboardKpis: { label: PAGE_TITLES.dashboard, path: '/dashboard', iconKey: 'dashboard' },
  exposure: { label: PAGE_TITLES.riskOperations, path: '/risks/findings', iconKey: 'risks' },
  investigations: { label: PAGE_TITLES.investigations, path: '/investigation', iconKey: 'investigation' },
  reports: { label: PAGE_TITLES.reports, path: '/reports', iconKey: 'reports' },
  clusters: { label: PAGE_TITLES.clusters, path: '/clusters', iconKey: 'clusters' },
  resources: { label: PAGE_TITLES.resources, path: '/resources', iconKey: 'resources' },
  capabilities: { label: PAGE_TITLES.capabilities, path: '/capabilities', iconKey: 'capabilities' },
  rules: { label: PAGE_TITLES.policyRules, path: '/rules', iconKey: 'rules' },
  attackPaths: { label: PAGE_TITLES.attackAnalysis, path: '/attack-paths', iconKey: 'attackPaths' },
  activeResponse: { label: PAGE_TITLES.homeOperator, path: '/', iconKey: 'dashboard' },
  threatOps: { label: 'Findings Queue', path: '/risks/findings', iconKey: 'risks' },
  runtime: { label: 'Runtime Network', path: '/network-activity', iconKey: 'network' },
  platform: { label: PAGE_TITLES.homeAdmin, path: '/', iconKey: 'dashboard' },
  governance: { label: PAGE_TITLES.governance, path: '/governance', iconKey: 'governance' },
  telemetry: { label: PAGE_TITLES.monitoring, path: '/monitoring', iconKey: 'monitoring' },
  settings: { label: PAGE_TITLES.settings, path: '/settings', iconKey: 'settings' },
};

const VIEWER_MISSIONS: { section: string; keys: string[] }[] = [
  { section: 'Overview', keys: ['home', 'dashboardKpis', 'exposure', 'investigations', 'reports'] },
  { section: 'Context', keys: ['attackPaths', 'runtime', 'resources', 'clusters', 'capabilities', 'telemetry'] },
];

const OPERATOR_MISSIONS: { section: string; keys: string[] }[] = [
  { section: 'Triage', keys: ['activeResponse', 'dashboardKpis', 'threatOps', 'investigations'] },
  { section: 'Evidence', keys: ['attackPaths', 'runtime', 'resources', 'clusters', 'capabilities'] },
  { section: 'Controls', keys: ['rules'] },
];

const ADMIN_MISSIONS: { section: string; keys: string[] }[] = [
  { section: 'Overview', keys: ['platform', 'dashboardKpis', 'telemetry'] },
  { section: 'Risk workflow', keys: ['threatOps', 'investigations', 'attackPaths', 'runtime'] },
  { section: 'Inventory', keys: ['resources', 'clusters', 'capabilities', 'reports'] },
  { section: 'Controls', keys: ['rules'] },
  { section: 'Administration', keys: ['governance', 'settings'] },
];

const USER_ADMIN_MISSIONS: { section: string; keys: string[] }[] = [
  { section: 'Access management', keys: ['settings'] },
];

function routeAllowed(path: string, allowedRoutes: string[]): boolean {
  const p = path.split('?')[0];
  return allowedRoutes.includes(p);
}

function buildSections(
  plan: { section: string; keys: string[] }[],
  allowedRoutes: string[],
): MissionNavSection[] {
  return plan
    .map(({ section, keys }) => ({
      title: section,
      items: keys
        .map((k) => {
          const base = MISSION_CATALOG[k];
          if (!base) return null;
          if (!routeAllowed(base.path, allowedRoutes)) return null;
          return { id: k, ...base };
        })
        .filter(Boolean) as MissionNavItem[],
    }))
    .filter((s) => s.items.length > 0);
}

export function buildMissionNavigation(
  personaId: PersonaId,
  plane: MaterializedOperationalPlane,
): MissionNavSection[] {
  const { allowedRoutes } = plane;
  switch (personaId) {
    case 'user_admin':
      return buildSections(USER_ADMIN_MISSIONS, allowedRoutes);
    case 'admin':
      return buildSections(ADMIN_MISSIONS, allowedRoutes);
    case 'operator':
      return buildSections(OPERATOR_MISSIONS, allowedRoutes);
    case 'viewer':
    default:
      return buildSections(VIEWER_MISSIONS, allowedRoutes);
  }
}
