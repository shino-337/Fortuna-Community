
export enum Severity {
  CRITICAL = 'Critical',
  HIGH = 'High',
  MEDIUM = 'Medium',
  LOW = 'Low'
}

export enum RiskType {
  CLUSTER_ADMIN = 'Cluster Admin',
  WILDCARD = 'Wildcard',
  OVER_PRIVILEGED = 'Over-Privileged',
  PRIVILEGE_ESCALATION = 'Privilege Escalation',
  NONE = 'None'
}

export interface Risk {
  id: string;
  type: RiskType;
  severity: Severity;
  description: string;
}

export interface ServiceAccount {
  id: string;
  name: string;
  namespace: string;
  cluster: string;
  uid: string;
  created: string;
  risks: Risk[];
  roles: string[];
  riskCount?: number;
}

export interface ClusterStats {
  nodes: { total: number; ready: number };
  pods: { total: number; running: number; pending: number; failed: number };
  cpu: { used: number; total: number; unit: string };
  memory: { used: number; total: number; unit: string };
  deployments: number;
  statefulsets: number;
  daemonsets: number;
  services: number;
  ingresses: number;
}

export interface GenericResource {
  id: string;
  name: string;
  namespace: string;
  cluster: string;
  status: 'Running' | 'Pending' | 'Failed' | 'Succeeded' | 'Bound' | 'Active';
  age: string;
  [key: string]: any; // Allow flexible fields like 'images', 'replicas', etc.
}

// --- Network Policy Types ---

export interface NetworkPolicy {
  id: string;
  name: string;
  namespace: string;
  cluster: string;
  uid: string;
  createdAt: string;
  updatedAt?: string;
  podSelector: Record<string, any>;
  policyTypes: ('Ingress' | 'Egress')[];
  ingress?: any[]; 
  egress?: any[];
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

// --- Audit Log Types ---

export interface AuditLogEntry {
  id: string;
  timestamp: string;
  verb: 'create' | 'update' | 'delete' | 'get' | 'list' | 'patch';
  resourceKind: string;
  resourceName: string;
  namespace: string;
  cluster: string;
  user: string;
  sourceIP: string;
  details: string; // JSON string payload
}

// --- Graph Visualization Types ---

export type GraphNodeType = 'ServiceAccount' | 'Role' | 'ClusterRole' | 'RoleBinding' | 'ClusterRoleBinding';

export interface GraphNodeData {
  id: string;
  label: string;
  type: GraphNodeType;
  namespace?: string;
  cluster?: string;
  riskLevel?: Severity; // If node has risks
  meta?: any; // Extra details for sidebar
}

export interface GraphEdgeData {
  id: string;
  source: string;
  target: string;
  label?: string; // e.g., "binds-to", "refers-to"
}

export interface GraphElement {
  data: GraphNodeData | GraphEdgeData;
}