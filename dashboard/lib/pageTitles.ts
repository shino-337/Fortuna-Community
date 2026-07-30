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
  capabilities: 'Capability Exposure',
  policyRules: 'Policy Rules',
  attackAnalysis: 'Attack Paths',
  networkActivity: 'Network Activity',
  monitoring: 'Pipeline & Runtime Health',
  governance: 'Audit & Governance',
  settings: 'Users & Settings',
  certificates: 'Certificates',
  notifications: 'Notifications',
  users: 'Users',
  sbom: 'SBOM & Vulnerability Analysis',
} as const;

export type PageTitleKey = keyof typeof PAGE_TITLES;
