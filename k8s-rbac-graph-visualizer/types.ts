import * as d3 from 'd3';

export enum NodeType {
  USER = 'User',
  GROUP = 'Group',
  SERVICE_ACCOUNT = 'ServiceAccount',
  ROLE = 'Role',
  CLUSTER_ROLE = 'ClusterRole',
  ROLE_BINDING = 'RoleBinding',
  CLUSTER_ROLE_BINDING = 'ClusterRoleBinding',
}

export interface RbacNode extends d3.SimulationNodeDatum {
  id: string;
  name: string;
  type: NodeType;
  clusterName?: string; // Added cluster context
  namespace?: string;
  rules?: Array<{
    apiGroups: string[];
    resources: string[];
    verbs: string[];
  }>;
  subjects?: Array<{
    kind: string;
    name: string;
    namespace?: string;
  }>;
  roleRef?: {
    kind: string;
    name: string;
    apiGroup: string;
  };
  radius?: number;

  // Explicitly add d3 simulation properties to avoid TS errors
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
  fx?: number | null;
  fy?: number | null;
  index?: number;
}

export interface RbacLink extends d3.SimulationLinkDatum<RbacNode> {
  source: string | RbacNode;
  target: string | RbacNode;
  type: 'binds_to' | 'subject_of';
}

export interface GraphData {
  nodes: RbacNode[];
  links: RbacLink[];
}

export interface FilterState {
  showSystem: boolean;
  nodeTypes: Record<NodeType, boolean>;
  clusters: Record<string, boolean>;   // New filter
  namespaces: Record<string, boolean>; // New filter
}