import axios from 'axios'

// Use relative path by default to go through nginx proxy
// If VITE_API_URL is set and is an absolute URL, use it; otherwise use relative path
const getApiBaseURL = () => {
  const envUrl = import.meta.env.VITE_API_URL as string
  if (envUrl && (envUrl.startsWith('http://') || envUrl.startsWith('https://'))) {
    return envUrl
  }
  // Use relative path to go through nginx proxy
  return '/api/v1'
}

const api = axios.create({
  baseURL: getApiBaseURL(),
  headers: {
    'Content-Type': 'application/json',
  },
})

export interface Cluster {
  id: string
  name: string
  endpoint: string
  status: string
  lastSync?: string
  createdAt?: string
  updatedAt?: string
}

export interface ClusterStats extends Cluster {
  serviceAccountCount: number
  roleCount: number
  clusterRoleCount: number
  roleBindingCount: number
  clusterRoleBindingCount: number
  podCount: number
  deploymentCount: number
  connectionStatus: 'connected' | 'disconnected' | 'degraded' | 'unknown'
  agentVersion?: string
}

export interface ServiceAccount {
  id: number
  clusterId: string
  name: string
  namespace: string
  uid: string
  labels?: string
  secrets?: string
  createdAt: string
  updatedAt: string
}

export interface Container {
  name: string
  image: string
  cpuRequest?: string
  memoryRequest?: string
  cpuLimit?: string
  memoryLimit?: string
}

export interface Condition {
  type: string
  status: string
  reason?: string
  message?: string
}

export interface Deployment {
  id: number
  clusterId: string
  uid: string
  name: string
  namespace: string
  replicasDesired: number
  replicasReady: number
  replicasAvailable: number
  replicasUnavailable: number
  replicasUpdated: number
  strategy: string
  containers: Container[]
  labels: Record<string, string>
  annotations: Record<string, string>
  selector: Record<string, string>
  conditions: Condition[]
  createdAt: string
  updatedAt: string
}

export interface GraphNode {
  id: string
  label: string
  type: 'serviceaccount' | 'role' | 'namespace' | 'cluster' | 'clusterrole'
  data?: Record<string, any>
}

export interface GraphEdge {
  id: string
  source: string
  target: string
  type: string
  data?: Record<string, any>
}

export interface GraphData {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export const clustersApi = {
  getAll: () => api.get<{ clusters: Cluster[] }>('/clusters'),
  getById: (id: string) => api.get<Cluster>(`/clusters/${id}`),
  getStats: () => api.get<{ clusters: ClusterStats[]; total: number }>('/clusters/stats'),
}

export interface Rule {
  verbs: string[]
  apiGroups: string[]
  resources: string[]
  resourceNames: string[] | null
  nonResourceURLs: string[] | null
}

export interface RoleBindingPermission {
  roleBinding: {
    id: number
    name: string
    namespace: string
    roleRef: string
    subjects: string
  }
  role: {
    id: number
    name: string
    namespace: string
    rules: string
  }
}

export interface ClusterRoleBindingPermission {
  clusterRoleBinding: {
    id: number
    name: string
    roleRef: string
    subjects: string
  }
  clusterRole: {
    id: number
    name: string
    rules: string
  }
}

export interface ServiceAccountPermissions {
  serviceAccountId: number
  roleBindings: RoleBindingPermission[]
  clusterRoleBindings: ClusterRoleBindingPermission[]
  effectiveRules: Rule[]
}

export const serviceAccountsApi = {
  getAll: (params?: { cluster?: string; namespace?: string; page?: number; pageSize?: number }) =>
    api.get<{ serviceAccounts: ServiceAccount[]; total: number; page: number; pageSize: number }>('/serviceaccounts', { params }),
  getById: (id: string) => api.get<ServiceAccount>(`/serviceaccounts/${id}`),
  getPermissions: (id: string) => api.get<ServiceAccountPermissions>(`/serviceaccounts/${id}/permissions`),
  update: (id: string, data: Partial<ServiceAccount>) =>
    api.put(`/serviceaccounts/${id}`, data),
  delete: (id: string) => api.delete(`/serviceaccounts/${id}`),
}

export interface ReplicaSet {
  id: number
  clusterId: string
  uid: string
  name: string
  namespace: string
  replicas: number
  readyReplicas: number
  availableReplicas: number
  fullyLabeledReplicas: number
  ownerKind?: string
  ownerName?: string
  ownerUid?: string
  containers: Container[]
  labels: Record<string, string>
  annotations: Record<string, string>
  selector: Record<string, string>
  conditions: Condition[]
  createdAt: string
  updatedAt: string
}

export const deploymentsApi = {
  getAll: (params?: { cluster?: string; namespace?: string; page?: number; pageSize?: number }) =>
    api.get<{ deployments: Deployment[]; total: number; page: number; pageSize: number }>('/deployments', { params }),
  getById: (id: string) => api.get<Deployment>(`/deployments/${id}`),
}

export const replicasetsApi = {
  getAll: (params?: { cluster?: string; namespace?: string; ownerKind?: string; ownerName?: string; page?: number; pageSize?: number }) =>
    api.get<{ replicasets: ReplicaSet[]; total: number; page: number; pageSize: number }>('/replicasets', { params }),
  getById: (id: string) => api.get<ReplicaSet>(`/replicasets/${id}`),
}

export interface StatefulSet {
  id: number
  clusterId: string
  uid: string
  name: string
  namespace: string
  replicas: number
  readyReplicas: number
  containers: Container[]
  labels: Record<string, string>
  annotations: Record<string, string>
  selector: Record<string, string>
  conditions: Condition[]
  createdAt: string
  updatedAt: string
}

export const statefulsetsApi = {
  getAll: (params?: { cluster?: string; namespace?: string; page?: number; pageSize?: number }) =>
    api.get<{ statefulsets: StatefulSet[]; total: number; page: number; pageSize: number }>('/statefulsets', { params }),
  getById: (id: string) => api.get<StatefulSet>(`/statefulsets/${id}`),
}

export interface Service {
  id: number
  clusterId: string
  uid: string
  name: string
  namespace: string
  type: string
  clusterIP?: string
  externalIPs?: string[]
  ports?: string
  selector?: Record<string, string>
  labels?: Record<string, string>
  annotations?: Record<string, string>
  createdAt: string
  updatedAt: string
}

export const servicesApi = {
  getAll: (params?: { cluster?: string; namespace?: string; page?: number; pageSize?: number }) =>
    api.get<{ services: Service[]; total: number; page: number; pageSize: number }>('/services', { params }),
  getById: (id: string) => api.get<Service>(`/services/${id}`),
}

export const graphApi = {
  getGraph: (params?: { cluster?: string; namespace?: string }) =>
    api.get<GraphData>('/graph', { params }),
}

export interface AuditLog {
  id: number
  clusterId?: string
  action: string
  resource: string
  resourceId?: string
  details?: string
  user?: string
  ip?: string
  createdAt: string
}

export const auditApi = {
  getLogs: (params?: { cluster?: string; resource?: string; action?: string; page?: number; pageSize?: number }) =>
    api.get<{ logs: AuditLog[]; total: number; page: number; pageSize: number }>('/audit', { params }),
  getReports: () => api.get('/audit/reports'),
}

export interface User {
  id: number
  username: string
  email: string
  role: string
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  user: User
  expiresAt?: string
}

export const authApi = {
  login: (username: string, password: string) =>
    api.post<LoginResponse>('/auth/login', { username, password }),
  getCurrentUser: () => api.get<{ user: User }>('/me'),
  register: (data: { username: string; email: string; password: string }) =>
    api.post('/auth/register', data),
  changePassword: (oldPassword: string, newPassword: string) =>
    api.post('/change-password', { oldPassword, newPassword }),
}

// Add request interceptor to include token in every request
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('ksam_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Add response interceptor to handle 401 errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      // Token expired or invalid, redirect to login
      localStorage.removeItem('ksam_token')
      delete api.defaults.headers.common['Authorization']
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  }
)

export default api

