import { NodeType } from './types';

export const NODE_COLORS: Record<NodeType, string> = {
  [NodeType.USER]: '#10b981', // Emerald 500
  [NodeType.GROUP]: '#059669', // Emerald 600
  [NodeType.SERVICE_ACCOUNT]: '#34d399', // Emerald 400
  [NodeType.ROLE_BINDING]: '#f59e0b', // Amber 500
  [NodeType.CLUSTER_ROLE_BINDING]: '#d97706', // Amber 600
  [NodeType.ROLE]: '#3b82f6', // Blue 500
  [NodeType.CLUSTER_ROLE]: '#2563eb', // Blue 600
};

export const NODE_RADIUS: Record<NodeType, number> = {
  [NodeType.USER]: 15,
  [NodeType.GROUP]: 18,
  [NodeType.SERVICE_ACCOUNT]: 15,
  [NodeType.ROLE_BINDING]: 10,
  [NodeType.CLUSTER_ROLE_BINDING]: 12,
  [NodeType.ROLE]: 20,
  [NodeType.CLUSTER_ROLE]: 25,
};

export const LEGEND_ITEMS = [
  { type: NodeType.USER, label: 'User', color: NODE_COLORS[NodeType.USER] },
  { type: NodeType.SERVICE_ACCOUNT, label: 'Service Account', color: NODE_COLORS[NodeType.SERVICE_ACCOUNT] },
  { type: NodeType.ROLE_BINDING, label: 'Binding', color: NODE_COLORS[NodeType.ROLE_BINDING] },
  { type: NodeType.ROLE, label: 'Role', color: NODE_COLORS[NodeType.ROLE] },
  { type: NodeType.CLUSTER_ROLE, label: 'Cluster Role', color: NODE_COLORS[NodeType.CLUSTER_ROLE] },
];
