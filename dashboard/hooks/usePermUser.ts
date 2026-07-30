import { useMemo } from 'react';
import { useAuthStore } from '../store/authStore';
import { can, canAny, canAll, P, type PermissionString } from '../lib/permissions';

/** Auth user with permissions filled for admin legacy rows. */
export function usePermUser() {
  const user = useAuthStore((s) => s.user);
  return useMemo(() => {
    if (!user) return null;
    if (user.permissions && user.permissions.length > 0) return user;
    if (String(user.role || '').toLowerCase() === 'admin') {
      return { ...user, permissions: Object.values(P) as string[] };
    }
    return user;
  }, [user]);
}

export function useCan(need: PermissionString): boolean {
  const permUser = usePermUser();
  return can(permUser, need);
}

export function useCanAny(needs: PermissionString[]): boolean {
  const permUser = usePermUser();
  return canAny(permUser, needs);
}

export function useCanAll(needs: PermissionString[]): boolean {
  const permUser = usePermUser();
  return canAll(permUser, needs);
}
