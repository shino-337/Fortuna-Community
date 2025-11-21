/**
 * Graph Visualization Constants
 * K8s Fortuna - Pink Theme
 */

import { NodeType } from './types'

// Node colors - Pink theme to match K8s Fortuna branding
export const NODE_COLORS: Record<string, string> = {
  // Primary entities - Pink shades
  [NodeType.SERVICE_ACCOUNT]: '#ec4899', // Pink 500
  [NodeType.USER]: '#db2777', // Pink 600
  [NodeType.GROUP]: '#be185d', // Pink 700
  
  // Permissions - Purple shades (complementary)
  [NodeType.ROLE]: '#a855f7', // Purple 500
  [NodeType.CLUSTER_ROLE]: '#9333ea', // Purple 600
  
  // Bindings - Amber/Orange shades
  [NodeType.ROLE_BINDING]: '#f59e0b', // Amber 500
  [NodeType.CLUSTER_ROLE_BINDING]: '#d97706', // Amber 600
  
  // Infrastructure - Blue/Cyan shades
  [NodeType.NAMESPACE]: '#06b6d4', // Cyan 500
  [NodeType.CLUSTER]: '#0891b2', // Cyan 600
  [NodeType.POD]: '#3b82f6', // Blue 500
  [NodeType.DEPLOYMENT]: '#2563eb', // Blue 600
  [NodeType.STATEFULSET]: '#1d4ed8', // Blue 700
  [NodeType.DAEMONSET]: '#1e40af', // Blue 800
  
  // References - Gray shades
  [NodeType.REFERENCE]: '#6b7280', // Gray 500
  [NodeType.SA_BINDING]: '#9ca3af', // Gray 400
  [NodeType.UNKNOWN]: '#9ca3af', // Gray 400
}

// Node radius by type (for SVG rendering)
export const NODE_RADIUS: Record<string, number> = {
  [NodeType.CLUSTER]: 28,
  [NodeType.CLUSTER_ROLE]: 24,
  [NodeType.NAMESPACE]: 22,
  [NodeType.DEPLOYMENT]: 20,
  [NodeType.STATEFULSET]: 20,
  [NodeType.DAEMONSET]: 20,
  [NodeType.ROLE]: 18,
  [NodeType.SERVICE_ACCOUNT]: 16,
  [NodeType.USER]: 16,
  [NodeType.GROUP]: 16,
  [NodeType.POD]: 14,
  [NodeType.ROLE_BINDING]: 12,
  [NodeType.CLUSTER_ROLE_BINDING]: 12,
  [NodeType.SA_BINDING]: 10,
  [NodeType.REFERENCE]: 10,
  [NodeType.UNKNOWN]: 10,
}

// Legend items for display (ordered by importance)
export const LEGEND_ITEMS = [
  { type: NodeType.SERVICE_ACCOUNT, label: 'Service Account', color: NODE_COLORS[NodeType.SERVICE_ACCOUNT] },
  { type: NodeType.ROLE, label: 'Role', color: NODE_COLORS[NodeType.ROLE] },
  { type: NodeType.CLUSTER_ROLE, label: 'Cluster Role', color: NODE_COLORS[NodeType.CLUSTER_ROLE] },
  { type: NodeType.ROLE_BINDING, label: 'Binding', color: NODE_COLORS[NodeType.ROLE_BINDING] },
  { type: NodeType.NAMESPACE, label: 'Namespace', color: NODE_COLORS[NodeType.NAMESPACE] },
  { type: NodeType.CLUSTER, label: 'Cluster', color: NODE_COLORS[NodeType.CLUSTER] },
]

// Force simulation parameters
export const FORCE_PARAMS = {
  linkDistance: 120,
  chargeStrength: -600,
  collideRadius: 40,
  centerStrength: 0.3,
}

// Zoom parameters
export const ZOOM_PARAMS = {
  scaleExtent: [0.1, 4] as [number, number],
  duration: 400,
  padding: 60,
}

