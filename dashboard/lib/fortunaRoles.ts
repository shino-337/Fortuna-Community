/**
 * Fortuna **application** roles (JWT / Settings → Users), distinct from Kubernetes RBAC.
 * Backend: core/pkg/models/user.go, core/pkg/authorization/registry.go, core/internal/api/auth_handlers.go.
 */

/** Normalize a Fortuna role key to the canonical form. */
export function normalizeFortunaRoleKey(role: string | null | undefined): string;
/** @returns The normalized role (admin/cluster_admin/operator/viewer) — "user" maps to operator, everything else is lowercased as-is. */
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
  operator: 'Operator',
  viewer: 'Viewer',
  risk_evaluator: 'Service: risk evaluation',
  system: 'System (audit only)',
};

/** One row per role for Settings help panel and tooltips. */
export const FORTUNA_ROLE_HELP_ROWS = [
  {
    key: 'admin',
    title: 'Admin',
    body:
      'Full platform: every cluster, user accounts, roles and cluster scope, rules and policies (which apply to every cluster), platform audit, error logs and certificate rotation.',
  },
  {
    key: 'cluster_admin',
    title: 'Cluster admin',
    body:
      'Runs security for its assigned clusters (at least one): everything an Operator does, plus exceptions, deleting findings, archiving cases and revoking or deleting ServiceAccounts in those clusters. No user management, platform audit, or rule and policy changes.',
  },
  {
    key: 'operator',
    title: 'Operator',
    body:
      'Day-to-day work inside its cluster scope: triage findings (ack, assign, resolve, dismiss, reopen, bulk), cases, findings export and risk evaluation. Reads rules and policies. Exceptions, deletes and Kubernetes changes go to a Cluster admin.',
  },
  {
    key: 'viewer',
    title: 'Viewer',
    body:
      'Read-only inside its cluster scope: findings, inventory, runtime, attack paths, malware telemetry, cases and high-level monitoring. No changes.',
  },
] as const;

const SELECT_LABEL: Record<string, string> = {
  admin: 'Admin — full platform',
  cluster_admin: 'Cluster admin — scoped security operations',
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

/** Short paragraph: how the roles stack (for Settings and cross-links). */
export const FORTUNA_ROLE_SUMMARY =
  'Each role holds everything the one below it can do: Viewer, Operator, Cluster admin, Admin. Only an Admin manages accounts and picks which clusters an account sees; a new account sees no cluster until then.';
