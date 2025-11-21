/**
 * Graph Visualization Types
 * K8s Fortuna Platform
 */

import * as d3 from 'd3'

export enum NodeType {
  USER = 'user',
  GROUP = 'group',
  SERVICE_ACCOUNT = 'serviceaccount',
  ROLE = 'role',
  CLUSTER_ROLE = 'clusterrole',
  ROLE_BINDING = 'rolebinding',
  CLUSTER_ROLE_BINDING = 'clusterrolebinding',
  NAMESPACE = 'namespace',
  CLUSTER = 'cluster',
  POD = 'pod',
  DEPLOYMENT = 'deployment',
  STATEFULSET = 'statefulset',
  DAEMONSET = 'daemonset',
  REFERENCE = 'reference',
  SA_BINDING = 'sa-binding',
  UNKNOWN = 'unknown',
}

export interface RbacNode extends d3.SimulationNodeDatum {
  id: string
  label: string
  type: string
  clusterName?: string
  namespace?: string
  data: Record<string, any>
  radius?: number
  
  // D3 simulation properties (explicitly declared for TypeScript)
  x?: number
  y?: number
  vx?: number
  vy?: number
  fx?: number | null
  fy?: number | null
  index?: number
}

export interface RbacLink extends d3.SimulationLinkDatum<RbacNode> {
  source: string | RbacNode
  target: string | RbacNode
  type: string
  data?: Record<string, any>
}

export interface GraphData {
  nodes: RbacNode[]
  links: RbacLink[]
}

export interface FilterState {
  cluster?: string
  namespace?: string
  nodeTypes: Record<string, boolean>
  connectionTypes: Record<string, boolean>
}

