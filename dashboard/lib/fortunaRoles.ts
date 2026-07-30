/**
 * Fortuna **application** roles (JWT / Settings → Users), distinct from Kubernetes RBAC.
 * Backend: core/pkg/models/user.go, core/pkg/authorization/registry.go, core/internal/api/auth_handlers.go.
 */

/** Normalize a Fortuna role key to the canonical form. */
export function normalizeFortunaRoleKey(role: string | null | undefined): string;
/** @returns The normalized role (admin/cluster_admin/user_admin/operator/viewer) — "user" maps to operator, everything else is lowercased as-is. */
export function normalizeFortunaRoleKey(role: string | null | undefined): string {
  const r = String(role ?? '')
    .trim()
    .toLowerCase();
  if (r === 'user') return 'operator';
  return r;
}

const SHORT_LABEL: Record<string, string> = {
  admin: 'Admin',
  cluster_admin: 'Cluster admin',
  user_admin: 'User admin',
  operator: 'Operator',
  viewer: 'Viewer',
};

/** One row per role for Settings help panel and tooltips. */
export const FORTUNA_ROLE_HELP_ROWS = [
  {
    key: 'admin',
    title: 'Admin',
    body:
      'Full platform: clusters, findings, inventory, policies, risk evaluation, platform audit, certificate rotation, malware upload, and full user management—including other Admin accounts.',
  },
  {
    key: 'user_admin',
    title: 'User admin',
    body:
      'Fortuna account administration only: create and manage users with roles User admin, Operator, or Viewer. No access to security data, clusters, or platform audit logs; cannot create or modify Admin users.',
  },
  {
    key: 'cluster_admin',
    title: 'Cluster admin',
    body:
      'Cluster-scoped security administration: triage findings, evaluate risk, inspect inventory/runtime/attack paths, and manage operational evidence inside assigned clusters. No Fortuna user management, platform audit, global policy/rule writes, malware upload, or certificate rotation.',
  },
  {
    key: 'operator',
    title: 'Operator',
    body:
      'Day-to-day security operations: triage findings (ack/resolve/dismiss/reopen, bulk, exceptions), inventory changes, runtime, governed graph traversal (not advanced Cypher), findings CSV/PDF export when granted export.findings, policies read, risk rules, risk evaluation. Session list/revoke when granted sessions.*. No platform-wide audit catalog, Fortuna user admin, or policy publish/reload unless granted separately.',
  },
  {
    key: 'viewer',
    title: 'Viewer',
    body:
      'Read-only: findings, inventory, runtime, graph summaries and paths (no ad-hoc graph traversal), malware telemetry, and high-level monitoring (metrics and agent status only — not operational error logs). Per-finding activity is available only inside a finding you can open. No writes to findings or inventory.',
  },
] as const;

const SELECT_LABEL: Record<string, string> = {
  admin: 'Admin — full platform',
  cluster_admin: 'Cluster admin — scoped security operations',
  user_admin: 'User admin — accounts only',
  operator: 'Operator — security operations',
  viewer: 'Viewer — read-only',
};

/** Get a short human-readable label for a Fortuna role. */
export function fortunaRoleShortLabel(role: string | null | undefined): string;
/** @returns A concise label (e.g., "Admin", "Operator") — defaults to "User" if the role is unknown or empty, and never returns an API key directly. */
export function fortunaRoleShortLabel(role: string | null | undefined): string {
  const k = normalizeFortunaRoleKey(role);
  if (!k) return 'User';
  return SHORT_LABEL[k] ?? (String(role).trim() || 'User');
}

/** Get a tooltip text for a Fortuna role. */
export function fortunaRoleTooltip(role: string | null | undefined): string;
/** @returns The detailed description of the role from FORTUNA_ROLE_HELP_ROWS, or a fallback message if unknown. */
export function fortunaRoleTooltip(role: string | null | undefined): string {
  const k = normalizeFortunaRoleKey(role);
  const row = FORTUNA_ROLE_HELP_ROWS.find((r) => r.key === k);
  return row?.body ?? 'Fortuna application role (see Settings → Users).';
}

/** Get a select label for a Fortuna role. */
export function fortunaRoleSelectLabel(apiRoleValue: string): string;
/** @returns A formatted select option like "Admin — full platform" or the raw API value if unknown. Never returns an API key directly. */
export function fortunaRoleSelectLabel(apiRoleValue: string): string {
  const k = normalizeFortunaRoleKey(apiRoleValue);
  return SELECT_LABEL[k] ?? apiRoleValue;
}

/** Short paragraph: Admin vs user_admin (for Settings and cross-links). */
export const FORTUNA_ADMIN_VS_USER_ADMIN =
  'Admin is the only role with full security and platform capabilities and may assign any Fortuna role, including Admin and Cluster admin. Cluster admin operates security workflows only inside assigned clusters. User admin manages Fortuna login accounts only (User admin, Operator, Viewer); they cannot access clusters, findings, or platform audit.';
