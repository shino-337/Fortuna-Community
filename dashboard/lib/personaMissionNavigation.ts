/**
 * Mission-centric navigation — derived only from materialized operational surfaces.
 */
import type { MaterializedOperationalPlane } from './routeMaterialization';
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
  home: { label: PAGE_TITLES.home, path: '/', iconKey: 'dashboard' },
  findings: { label: PAGE_TITLES.riskOperations, path: '/risks/findings', iconKey: 'risks' },
  investigations: { label: PAGE_TITLES.investigations, path: '/investigation', iconKey: 'investigation' },
  attackPaths: { label: PAGE_TITLES.attackAnalysis, path: '/attack-paths', iconKey: 'attackPaths' },
  network: { label: PAGE_TITLES.networkActivity, path: '/network-activity', iconKey: 'network' },
  inventory: { label: PAGE_TITLES.resources, path: '/resources', iconKey: 'resources', fallbackPaths: ['/resources/clusters'] },
  // Rules opens on the first section the user can read (viewers only see the catalog).
  rules: { label: PAGE_TITLES.policyRules, path: '/rules', iconKey: 'rules', fallbackPaths: ['/rules/catalog'] },
  platform: { label: PAGE_TITLES.monitoring, path: '/monitoring', iconKey: 'monitoring' },
  audit: { label: PAGE_TITLES.governance, path: '/governance', iconKey: 'governance' },
  users: { label: PAGE_TITLES.settings, path: '/settings', iconKey: 'settings' },
};

/**
 * One navigation for every role: the same groups, names and order. A role only
 * changes which items appear, and only through permissions (allowedRoutes).
 */
const NAVIGATION_PLAN: { section: string; keys: string[] }[] = [
  { section: 'Work', keys: ['home', 'findings', 'investigations'] },
  { section: 'Explore', keys: ['attackPaths', 'network', 'inventory'] },
  { section: 'Configure', keys: ['rules', 'platform'] },
  { section: 'Administration', keys: ['audit', 'users'] },
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

export function buildMissionNavigation(plane: MaterializedOperationalPlane): MissionNavSection[] {
  return buildSections(NAVIGATION_PLAN, plane.allowedRoutes);
}
