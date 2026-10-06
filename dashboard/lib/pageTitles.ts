/** One title per page, the same for every role; the sidebar uses the same words. */
export const PAGE_TITLES = {
  home: 'Home',
  riskOperations: 'Findings',
  investigations: 'Investigations',
  clusters: 'Clusters',
  resources: 'Inventory',
  capabilities: 'Capability Catalog',
  policyRules: 'Rules',
  attackAnalysis: 'Attack Paths',
  networkActivity: 'Network',
  monitoring: 'Platform',
  governance: 'Audit',
  settings: 'Users & Access',
  account: 'Account',
  certificates: 'Certificates',
  notifications: 'Notifications',
  users: 'Users',
  sbom: 'SBOM & Vulnerability Analysis',
} as const;

export type PageTitleKey = keyof typeof PAGE_TITLES;
