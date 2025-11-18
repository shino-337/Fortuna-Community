export type Role = 'admin' | 'user'

export interface Permission {
  resource: string
  action: 'read' | 'create' | 'update' | 'delete'
}

// Define role permissions
const rolePermissions: Record<Role, Permission[]> = {
  admin: [
    // Admin has full access
    { resource: 'serviceaccounts', action: 'read' },
    { resource: 'serviceaccounts', action: 'create' },
    { resource: 'serviceaccounts', action: 'update' },
    { resource: 'serviceaccounts', action: 'delete' },
    { resource: 'audit', action: 'read' },
    { resource: 'graph', action: 'read' },
    { resource: 'users', action: 'read' },
    { resource: 'users', action: 'create' },
    { resource: 'users', action: 'update' },
    { resource: 'users', action: 'delete' },
  ],
  user: [
    // User has read-only access
    { resource: 'serviceaccounts', action: 'read' },
    { resource: 'audit', action: 'read' },
    { resource: 'graph', action: 'read' },
  ],
}

export function hasPermission(
  userRole: string | undefined,
  resource: string,
  action: 'read' | 'create' | 'update' | 'delete'
): boolean {
  if (!userRole) return false
  
  const role = userRole.toLowerCase() as Role
  const permissions = rolePermissions[role]
  
  if (!permissions) return false
  
  return permissions.some(
    (p) => p.resource === resource && p.action === action
  )
}

export function canRead(userRole: string | undefined, resource: string): boolean {
  return hasPermission(userRole, resource, 'read')
}

export function canCreate(userRole: string | undefined, resource: string): boolean {
  return hasPermission(userRole, resource, 'create')
}

export function canUpdate(userRole: string | undefined, resource: string): boolean {
  return hasPermission(userRole, resource, 'update')
}

export function canDelete(userRole: string | undefined, resource: string): boolean {
  return hasPermission(userRole, resource, 'delete')
}

export function isAdmin(userRole: string | undefined): boolean {
  return userRole?.toLowerCase() === 'admin'
}

