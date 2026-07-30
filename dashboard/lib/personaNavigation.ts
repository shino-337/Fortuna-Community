import type { PersonaId } from './persona';
import type { FeatureId } from './featureRegistry';
import { PAGE_TITLES } from './pageTitles';

export type PersonaNavItem = {
  iconKey: string;
  feature: FeatureId;
  label: string;
  path: string;
  search?: string;
  highlight?: boolean;
};

export type PersonaNavSection = {
  title: string;
  items: PersonaNavItem[];
};

const NAV_CATALOG: Record<string, Omit<PersonaNavItem, 'iconKey' | 'feature'> & { feature: FeatureId }> = {
  dashboard: { feature: 'dashboard', label: PAGE_TITLES.dashboard, path: '/' },
  platformIntegrity: { feature: 'dashboard', label: PAGE_TITLES.homeAdmin, path: '/' },
  clusters: { feature: 'clusters', label: PAGE_TITLES.clusters, path: '/clusters' },
  resources: { feature: 'resources', label: PAGE_TITLES.resources, path: '/resources' },
  network: { feature: 'network_activity', label: 'Runtime Network', path: '/network-activity' },
  risks: { feature: 'risk_operations', label: 'Findings Queue', path: '/risks' },
  investigation: {
    feature: 'investigation',
    label: PAGE_TITLES.investigations,
    path: '/investigation',
    highlight: true,
  },
  capabilities: { feature: 'capabilities', label: PAGE_TITLES.capabilities, path: '/capabilities' },
  rules: { feature: 'rules_catalog', label: PAGE_TITLES.policyRules, path: '/rules' },
  attackPaths: { feature: 'attack_paths', label: PAGE_TITLES.attackAnalysis, path: '/attack-paths' },
  monitoring: { feature: 'monitoring', label: PAGE_TITLES.monitoring, path: '/monitoring' },
  governance: { feature: 'governance', label: PAGE_TITLES.governance, path: '/governance' },
  reports: { feature: 'reports', label: PAGE_TITLES.reports, path: '/reports' },
  settings: { feature: 'settings', label: PAGE_TITLES.settings, path: '/settings' },
};

const PERSONA_NAV_ORDER: Record<PersonaId, { section: string; keys: string[] }[]> = {
  viewer: [
    { section: 'Overview', keys: ['dashboard', 'risks', 'investigation', 'reports'] },
    { section: 'Context', keys: ['attackPaths', 'network', 'resources', 'clusters', 'capabilities', 'monitoring'] },
  ],
  operator: [
    { section: 'Triage', keys: ['dashboard', 'risks', 'investigation'] },
    { section: 'Evidence', keys: ['attackPaths', 'network', 'resources', 'clusters', 'capabilities'] },
    { section: 'Controls', keys: ['rules'] },
  ],
  admin: [
    { section: 'Overview', keys: ['platformIntegrity', 'dashboard', 'monitoring'] },
    { section: 'Risk workflow', keys: ['risks', 'investigation', 'attackPaths', 'network'] },
    { section: 'Inventory', keys: ['resources', 'clusters', 'capabilities', 'reports'] },
    { section: 'Controls', keys: ['rules'] },
    { section: 'Administration', keys: ['governance', 'settings'] },
  ],
  user_admin: [{ section: 'Access management', keys: ['settings'] }],
};

export function buildPersonaNavigation(personaId: PersonaId): PersonaNavSection[] {
  const plan = PERSONA_NAV_ORDER[personaId] ?? PERSONA_NAV_ORDER.viewer;
  return plan.map(({ section, keys }) => ({
    title: section,
    items: keys
      .map((k) => {
        const base = NAV_CATALOG[k];
        if (!base) return null;
        return { iconKey: k, ...base };
      })
      .filter(Boolean) as PersonaNavItem[],
  }));
}
