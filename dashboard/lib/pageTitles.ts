/** One title per page, the same for every role; the sidebar uses the same words. */
export const PAGE_TITLES = {
  home: 'Home',
  riskOperations: 'Findings',
  investigations: 'Cases',
  clusters: 'Clusters',
  resources: 'Inventory',
  capabilities: 'Capability Catalog',
  policyRules: 'Rules',
  attackAnalysis: 'Attack Paths',
  networkActivity: 'Network',
  monitoring: 'Platform',
  governance: 'Audit',
  setup: 'Setup',
  settings: 'Users & Access',
  account: 'Account',
  certificates: 'Certificates',
  notifications: 'Notifications',
  users: 'Users',
  sbom: 'SBOM & Vulnerability Analysis',
} as const;

export type PageTitleKey = keyof typeof PAGE_TITLES;

/** One description for every Rules section, so switching tabs does not rewrite the header. */
export const RULES_PAGE_DESCRIPTION =
  'What Fortuna detects, how it scores risk, and which capabilities your pods hold.';
