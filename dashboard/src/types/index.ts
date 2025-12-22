export interface Cluster {
  id: string;
  name: string;
  endpoint: string;
  status: 'healthy' | 'warning' | 'error';
  nodeCount?: number;
  podCount?: number;
  lastSeen?: string;
}

export interface Insight {
  id: number;
  type: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
  description: string;
  affected_resources?: any;
  recommended_action?: string;
  created_at: string;
}

export interface InsightSummary {
  total: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface Pod {
  id: string;
  name: string;
  namespace: string;
  serviceAccount?: string;
  status: string;
  risk?: 'low' | 'medium' | 'high' | 'critical';
}

export interface ServiceAccount {
  id: string;
  name: string;
  namespace: string;
  cluster: string;
}

export interface Role {
  id: string;
  name: string;
  namespace?: string;
  kind: 'Role' | 'ClusterRole';
}

export interface GraphNode {
  id: string;
  type: 'Pod' | 'ServiceAccount' | 'Role' | 'ClusterRole';
  name: string;
  properties: Record<string, any>;
}

export interface GraphEdge {
  source: string;
  target: string;
  type: string;
}

export interface Rule {
  id: string;
  name: string;
  category: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
  enabled: boolean;
  description: string;
  conditions: any[];
  base_score: number;
  tags?: string[];
}

export interface RiskyResource {
  id: string;
  name: string;
  namespace: string;
  type: 'Pod' | 'ServiceAccount' | 'Role' | 'ClusterRole' | 'RoleBinding' | 'ClusterRoleBinding';
  riskScore: number;
  riskLevel: 'critical' | 'high' | 'medium' | 'low';
  insights: number;
  clusterId: string;
}

