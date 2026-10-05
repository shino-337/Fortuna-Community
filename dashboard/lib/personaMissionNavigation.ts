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
  { label: string; path: string; iconKey: string; search?: string; highlight?: boolean; fallbackPaths?: string[] }
> = {
  home: { label: PAGE_TITLES.homeViewer, path: '/', iconKey: 'dashboard' },
  exposure: { label: PAGE_TITLES.riskOperations, path: '/risks/findings', iconKey: 'risks' },
  investigations: { label: PAGE_TITLES.investigations, path: '/investigation', iconKey: 'investigation' },
  resources: { label: PAGE_TITLES.resources, path: '/resources', iconKey: 'resources' },
  // Rules & Catalog opens on the first section the user can read (viewers only see the catalog).
  rules: { label: PAGE_TITLES.policyRules, path: '/rules', iconKey: 'rules', fallbackPaths: ['/rules/catalog'] },
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
  { section: 'Overview', keys: ['home', 'exposure', 'investigations'] },
  { section: 'Context', keys: ['attackPaths', 'runtime', 'resources', 'rules', 'telemetry'] },
];

const OPERATOR_MISSIONS: { section: string; keys: string[] }[] = [
  { section: 'Triage', keys: ['activeResponse', 'threatOps', 'investigations'] },
  { section: 'Evidence', keys: ['attackPaths', 'runtime', 'resources'] },
  { section: 'Controls', keys: ['rules', 'telemetry'] },
];

const ADMIN_MISSIONS: { section: string; keys: string[] }[] = [
  { section: 'Overview', keys: ['platform', 'telemetry'] },
  { section: 'Risk workflow', keys: ['threatOps', 'investigations', 'attackPaths', 'runtime'] },
  { section: 'Inventory', keys: ['resources'] },
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
          const entry = MISSION_CATALOG[k];
          if (!entry) return null;
          const { fallbackPaths, ...base } = entry;
          const path = [base.path, ...(fallbackPaths ?? [])].find((candidate) => routeAllowed(candidate, allowedRoutes));
          if (!path) return null;
          return { id: k, ...base, path };
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
