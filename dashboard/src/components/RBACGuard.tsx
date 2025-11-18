import { ReactNode } from 'react'
import { useAuth } from '../contexts/AuthContext'
import { hasPermission } from '../utils/rbac'

interface RBACGuardProps {
  resource: string
  action: 'read' | 'create' | 'update' | 'delete'
  children: ReactNode
  fallback?: ReactNode
}

export function RBACGuard({ resource, action, children, fallback = null }: RBACGuardProps) {
  const { user } = useAuth()
  
  if (!hasPermission(user?.role, resource, action)) {
    return <>{fallback}</>
  }
  
  return <>{children}</>
}

interface RBACButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  resource: string
  action: 'create' | 'update' | 'delete'
  children: ReactNode
}

export function RBACButton({ resource, action, children, ...props }: RBACButtonProps) {
  const { user } = useAuth()
  const allowed = hasPermission(user?.role, resource, action)
  
  return (
    <button
      {...props}
      disabled={!allowed || props.disabled}
      className={`${props.className} ${!allowed ? 'opacity-50 cursor-not-allowed' : ''}`}
      title={!allowed ? 'You do not have permission to perform this action' : props.title}
    >
      {children}
    </button>
  )
}

