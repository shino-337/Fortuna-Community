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
  getAll: () => api.get<Cluster[]>('/clusters'),
  getById: (id: string) => api.get<Cluster>(`/clusters/${id}`),
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

