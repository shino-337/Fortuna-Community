export const PAGE_TITLES = {
  homeViewer: 'My Exposure',
  homeOperator: 'Active Response',
  homeAdmin: 'Platform Integrity',
  dashboard: 'Operations Dashboard',
  riskOperations: 'Risk Findings',
  investigations: 'Investigations',
  reports: 'Reports',
  clusters: 'Clusters',
  resources: 'Kubernetes Inventory',
  capabilities: 'Capability Catalog',
  policyRules: 'Rules & Catalog',
  attackAnalysis: 'Attack Paths',
  networkActivity: 'Network Activity',
  monitoring: 'Platform Health',
  governance: 'Audit',
  settings: 'Users & Settings',
  certificates: 'Certificates',
  notifications: 'Notifications',
  users: 'Users',
  sbom: 'SBOM & Vulnerability Analysis',
} as const;

export type PageTitleKey = keyof typeof PAGE_TITLES;
