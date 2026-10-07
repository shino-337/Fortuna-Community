import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { Cluster, User, FortunaUserSession } from '../types';
import { api, getAvailabilityIssue, type AvailabilityIssue } from '../lib/api';
import { Shield, Plus, Trash2 } from 'lucide-react';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty, PageError } from '../design-system/components/PageStatus';
import { useConfirm } from '../design-system/components/ConfirmDialog';
import { Dialog } from '../design-system/components/Dialog';
import { useToast } from '../design-system/components/Toast';
import { formatDateTime } from '../lib/display';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH, UI_TR, UI_TD } from '../lib/tableChrome';
import { UI_FILTER_SELECT_SM } from '../lib/formChrome';
import {
  FORTUNA_ROLE_SUMMARY,
  FORTUNA_ROLE_HELP_ROWS,
  fortunaRoleSelectLabel,
  fortunaRoleShortLabel,
  fortunaRoleTooltip,
  normalizeFortunaRoleKey,
} from '../lib/fortunaRoles';
import { useAuthStore } from '../store/authStore';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { ACTION_IDS, canRunAction } from '../lib/actionAccess';
import { isPlatformAdmin } from '../lib/roles';
import { PAGE_TITLES } from '../lib/pageTitles';
import { AvailabilityNotice } from '../components/AvailabilityNotice';


function parseUserScopeClusters(user: User): { restricted: boolean; clusters: string[]; invalid: boolean } {
  if (user.operationalScope) {
    return {
      restricted: Boolean(user.operationalScope.restricted),
      clusters: Array.isArray(user.operationalScope.clusters) ? user.operationalScope.clusters : [],
      invalid: false,
    };
  }
  const raw = String(user.scopeJson || '').trim();
  if (raw === '' || raw === '{}') return { restricted: false, clusters: [], invalid: false };
  try {
    const parsed = JSON.parse(raw) as { clusters?: unknown; cluster_ids?: unknown };
    const listed = Array.isArray(parsed.clusters) || Array.isArray(parsed.cluster_ids);
    const list = Array.isArray(parsed.clusters) ? parsed.clusters : Array.isArray(parsed.cluster_ids) ? parsed.cluster_ids : [];
    // An empty list is a valid scope that grants no cluster; only "{}" means every cluster.
    return {
      restricted: listed,
      clusters: list.map((id) => String(id)).filter(Boolean),
      invalid: false,
    };
  } catch {
    return { restricted: true, clusters: [], invalid: true };
  }
}

function buildClusterScopeJson(mode: 'all' | 'selected', clusterIds: string[]): string {
  if (mode === 'all') return '{}';
  return JSON.stringify({ clusters: Array.from(new Set(clusterIds.filter(Boolean))).sort() });
}

function clusterDisplayName(cluster: Cluster): string {
  return cluster.name && cluster.name !== cluster.id ? `${cluster.name} (${cluster.id})` : cluster.id;
}

/** Validation and persistence are enforced on the server only. Client only sends payload and displays server errors. */

export const Settings: React.FC = () => {
  const { user } = useAuthStore();
  const confirm = useConfirm();
  const toast = useToast();
  const permUser = usePermUser();
  const canUsers = can(permUser, P.usersRead);
  const canUsersUpdate = canRunAction(permUser, ACTION_IDS.userUpdate);
  const canUsersRoleAssign = canRunAction(permUser, ACTION_IDS.userRoleAssign);
  const canUsersCreate = canRunAction(permUser, ACTION_IDS.userCreate);
  const canUsersDelete = canRunAction(permUser, ACTION_IDS.userDelete);
  const canUsersRowActions = canUsersUpdate || canUsersRoleAssign || canUsersDelete;
  const canRegister = can(permUser, P.authRegister);
  const canSessionsRead = can(permUser, P.sessionsRead);
  const canSessionsRevoke = canRunAction(permUser, ACTION_IDS.sessionRevoke);
  const canSessionsRevokeAll = canRunAction(permUser, ACTION_IDS.sessionRevokeAll);
  const canUsersReadForSessions = can(permUser, P.usersRead);

  const isFortunaAdmin = isPlatformAdmin(user);
  // Mirrors core: only a platform admin changes or deletes accounts.
  const userAdminCannotManage = useCallback((_row: User) => !isFortunaAdmin, [isFortunaAdmin]);
  const fortunaRoleEditOptions = useMemo(
    () => (isFortunaAdmin ? (['admin', 'cluster_admin', 'operator', 'viewer'] as const) : ([] as const)),
    [isFortunaAdmin],
  );

  const settingsTabs = useMemo(() => {
    const t: string[] = [];
    if (canUsers) t.push('Users');
    if (canSessionsRead) t.push('Sessions');
    return t;
  }, [canUsers, canSessionsRead]);

  const [activeTab, setActiveTab] = useState(() => settingsTabs[0] ?? '');
  const [users, setUsers] = useState<User[]>([]);
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const clusterById = useMemo(() => new Map(clusters.map((cluster) => [cluster.id, cluster])), [clusters]);
  const [userSearch, setUserSearch] = useState('');
  const [userRoleFilter, setUserRoleFilter] = useState('');
  const [userStatusFilter, setUserStatusFilter] = useState<'all' | 'active' | 'disabled'>('all');
  const userRoleOptions = useMemo(
    () => Array.from(new Set(users.map((u) => normalizeFortunaRoleKey(u.role)).filter(Boolean))).sort() as string[],
    [users],
  );
  const visibleUsers = useMemo(() => {
    const q = userSearch.trim().toLowerCase();
    return users.filter((u) => {
      if (q && !`${u.username ?? ''} ${u.name ?? ''} ${u.email ?? ''}`.toLowerCase().includes(q)) return false;
      if (userRoleFilter && normalizeFortunaRoleKey(u.role) !== userRoleFilter) return false;
      if (userStatusFilter === 'active' && u.active === false) return false;
      if (userStatusFilter === 'disabled' && u.active !== false) return false;
      return true;
    });
  }, [users, userSearch, userRoleFilter, userStatusFilter]);
  const [loadingUsers, setLoadingUsers] = useState(false);
  const [loadingClusters, setLoadingClusters] = useState(false);
  const [clusterAvailabilityIssue, setClusterAvailabilityIssue] = useState<AvailabilityIssue | null>(null);
  const [usersError, setUsersError] = useState<string | null>(null);
  const [savingUserId, setSavingUserId] = useState<string | null>(null);
  const [addUserOpen, setAddUserOpen] = useState(false);
  const [newUsername, setNewUsername] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newUserRole, setNewUserRole] = useState('viewer');
  // New accounts start with no cluster: every cluster is a deliberate choice.
  const [newUserScopeMode, setNewUserScopeMode] = useState<'all' | 'selected'>('selected');
  const [newUserScopeClusters, setNewUserScopeClusters] = useState<string[]>([]);
  const [addUserError, setAddUserError] = useState('');
  const [addUserBusy, setAddUserBusy] = useState(false);
  const [scopeEditorUser, setScopeEditorUser] = useState<User | null>(null);
  const [scopeMode, setScopeMode] = useState<'all' | 'selected'>('all');
  const [selectedScopeClusters, setSelectedScopeClusters] = useState<string[]>([]);
  const [scopeSaving, setScopeSaving] = useState(false);
  const [scopeError, setScopeError] = useState('');

  const [sessionRows, setSessionRows] = useState<FortunaUserSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [sessionUserIdFilter, setSessionUserIdFilter] = useState('');
  const [sessionsError, setSessionsError] = useState<string | null>(null);
  // Free-text filters are debounced before they reach the API (each keystroke would otherwise refetch).
  const [debouncedSessionUserId, setDebouncedSessionUserId] = useState('');
  const governanceRequestRef = useRef(0);
  useEffect(() => {
    const t = window.setTimeout(() => setDebouncedSessionUserId(sessionUserIdFilter.trim()), 300);
    return () => window.clearTimeout(t);
  }, [sessionUserIdFilter]);
  const [revokingSessionId, setRevokingSessionId] = useState<string | null>(null);

  const loadUsers = useCallback(async () => {
    if (!canUsers) return;
    setLoadingUsers(true);
    setUsersError(null);
    try {
      const data = await api.getUsers();
      setUsers(data);
    } catch (e) {
      setUsers([]);
      setUsersError(e instanceof Error ? e.message : "Failed to load users");
    } finally {
      setLoadingUsers(false);
    }
  }, [canUsers]);

  const loadClustersForScope = useCallback(async () => {
    if (!isFortunaAdmin || !canUsers) return;
    setLoadingClusters(true);
    try {
      setClusters(await api.getClustersStats());
      setClusterAvailabilityIssue(null);
    } catch (err) {
      setClusterAvailabilityIssue(getAvailabilityIssue(err, 'Cluster scope inventory'));
    } finally {
      setLoadingClusters(false);
    }
  }, [canUsers, isFortunaAdmin]);

  const refreshUsersSilently = useCallback(async () => {
    try {
      const data = await api.getUsers();
      setUsers(data);
      setUsersError(null);
    } catch (e) {
      setUsersError(e instanceof Error ? e.message : "Failed to refresh users");
    }
  }, []);


  const loadGovernance = useCallback(async () => {
    // Overlapping filter changes: only the newest request may update the tables.
    const seq = ++governanceRequestRef.current;
    const isStale = () => seq !== governanceRequestRef.current;
    if (canSessionsRead) {
      setSessionsLoading(true);
      try {
        const uid =
          isFortunaAdmin && canUsersReadForSessions && debouncedSessionUserId !== ''
            ? debouncedSessionUserId
            : undefined;
        const rows = await api.listUserSessions(uid);
        if (isStale()) return;
        setSessionRows(rows);
        setSessionsError(null);
      } catch (e) {
        if (isStale()) return;
        setSessionRows([]);
        setSessionsError(e instanceof Error ? e.message : 'Failed to load sessions');
      } finally {
        if (!isStale()) setSessionsLoading(false);
      }
    }
  }, [
    canSessionsRead,
    isFortunaAdmin,
    canUsersReadForSessions,
    debouncedSessionUserId,
    toast,
  ]);

  const revokeFortunaSession = useCallback(
    async (sessionId: string) => {
      const confirmed = await confirm({
        title: 'Revoke session',
        description: 'That client must sign in again after this session is revoked.',
        confirmLabel: 'Revoke session',
        variant: 'danger',
      });
      if (!confirmed) return;
      setRevokingSessionId(sessionId);
      try {
        await api.revokeUserSession(sessionId);
        await loadGovernance();
      } catch (e) {
        toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
      } finally {
        setRevokingSessionId(null);
      }
    },
    [confirm, loadGovernance, toast],
  );

  const revokeAllMySessions = useCallback(async () => {
    if (!canSessionsRevoke) return;
    const confirmed = await confirm({
      title: 'Revoke all active sessions',
      description: 'You will need to sign in again on this browser after all active sessions are revoked.',
      confirmLabel: 'Revoke all sessions',
      variant: 'danger',
    });
    if (!confirmed) return;
    try {
      await api.revokeAllUserSessions();
      useAuthStore.getState().logout();
    } catch (e) {
      toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
    }
  }, [canSessionsRevoke, confirm, toast]);

  const updateFortunaUser = useCallback(
    async (rowId: string, body: { role?: string; active?: boolean; scopeJson?: string }) => {
      setSavingUserId(rowId);
      try {
        await api.updateUser(rowId, body);
        await refreshUsersSilently();
      } catch (e) {
        toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
      } finally {
        setSavingUserId(null);
      }
    },
    [refreshUsersSilently, toast],
  );

  const openScopeEditor = useCallback((row: User) => {
    const parsed = parseUserScopeClusters(row);
    setScopeEditorUser(row);
    setScopeMode(parsed.restricted ? 'selected' : 'all');
    setSelectedScopeClusters(parsed.clusters);
    setScopeError(parsed.invalid ? 'Current scope JSON is invalid. Saving will replace it with the selected cluster scope.' : '');
  }, []);

  const saveScopeEditor = useCallback(async () => {
    if (!scopeEditorUser) return;
    setScopeError('');
    if (scopeMode === 'selected' && clusterAvailabilityIssue) {
      setScopeError('Cluster inventory is unavailable. Retry the inventory before changing a selected-cluster scope.');
      return;
    }
    if ((scopeEditorUser.role || '').toLowerCase() === 'cluster_admin' && (scopeMode === 'all' || selectedScopeClusters.length === 0)) {
      setScopeError('A cluster admin needs at least one selected cluster.');
      return;
    }
    setScopeSaving(true);
    try {
      await api.updateUser(scopeEditorUser.id, {
        scopeJson: buildClusterScopeJson(scopeMode, selectedScopeClusters),
      });
      await refreshUsersSilently();
      setScopeEditorUser(null);
      toast({ title: 'Cluster access updated', description: `${scopeEditorUser.username} scope was saved.`, variant: 'success' });
    } catch (e) {
      setScopeError(String(e instanceof Error ? e.message : e));
    } finally {
      setScopeSaving(false);
    }
  }, [clusterAvailabilityIssue, refreshUsersSilently, scopeEditorUser, scopeMode, selectedScopeClusters, toast]);

  const removeFortunaUser = useCallback(
    async (rowId: string) => {
      setSavingUserId(rowId);
      try {
        await api.deleteUser(rowId);
        await refreshUsersSilently();
      } catch (e) {
        toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
      } finally {
        setSavingUserId(null);
      }
    },
    [refreshUsersSilently, toast],
  );

  const registerFortunaUser = useCallback(async () => {
    setAddUserError('');
    if (newUsername.trim().length < 3) {
      setAddUserError('Username must be at least 3 characters.');
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(newEmail.trim())) {
      setAddUserError('Valid email required.');
      return;
    }
    if (newPassword.length < 12) {
      setAddUserError('Password must be at least 12 characters.');
      return;
    }
    const newRoleIsAdmin = newUserRole.toLowerCase() === 'admin';
    if (isFortunaAdmin && !newRoleIsAdmin && newUserScopeMode === 'selected' && clusterAvailabilityIssue) {
      setAddUserError('Cluster inventory is unavailable. Retry the inventory before assigning a selected-cluster scope.');
      return;
    }
    if (newUserRole.toLowerCase() === 'cluster_admin' && (newUserScopeMode === 'all' || newUserScopeClusters.length === 0)) {
      setAddUserError('A cluster admin needs at least one selected cluster.');
      return;
    }
    setAddUserBusy(true);
    try {
      await api.registerUser({
        username: newUsername.trim(),
        email: newEmail.trim(),
        password: newPassword,
        role: newUserRole,
        scopeJson: isFortunaAdmin && !newRoleIsAdmin ? buildClusterScopeJson(newUserScopeMode, newUserScopeClusters) : undefined,
      });
      setAddUserOpen(false);
      setNewUsername('');
      setNewEmail('');
      setNewPassword('');
      setNewUserRole('viewer');
      setNewUserScopeMode('all');
      setNewUserScopeClusters([]);
      await refreshUsersSilently();
    } catch (e) {
      setAddUserError(String(e instanceof Error ? e.message : e));
    } finally {
      setAddUserBusy(false);
    }
  }, [clusterAvailabilityIssue, isFortunaAdmin, newUsername, newEmail, newPassword, newUserRole, newUserScopeMode, newUserScopeClusters, refreshUsersSilently]);

  useEffect(() => {
    if (activeTab === "Users" && canUsers) {
      void loadUsers();
      void loadClustersForScope();
    }
  }, [activeTab, loadUsers, loadClustersForScope, canUsers]);

  useEffect(() => {
    if (activeTab === 'Sessions' && canSessionsRead) {
      void loadGovernance();
    }
  }, [activeTab, canSessionsRead, loadGovernance]);

  useEffect(() => {
    if (settingsTabs.length === 0) return;
    if (!settingsTabs.includes(activeTab)) {
      setActiveTab(settingsTabs[0]);
    }
  }, [settingsTabs, activeTab]);
  if (settingsTabs.length === 0) {
    return (
      <PageLayout title={PAGE_TITLES.settings} description="Fortuna administration">
        <PageEmpty
          title="No settings available"
          description="Your role does not include user or session administration permissions."
          className="py-10"
        />
      </PageLayout>
    );
  }

  return (
    <PageLayout
      title={PAGE_TITLES.settings}
      description="Who can sign in, their role, cluster access and sessions. Your own password and sessions are under Account."
    >
      <div className="border-b border-border">
        <nav className="flex space-x-6 overflow-x-auto">
          {settingsTabs.map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`pb-4 text-body font-medium border-b-2 transition-colors whitespace-nowrap ${activeTab === tab ? 'border-brand text-brand' : 'border-transparent text-muted hover:text-text hover:border-border'}`}
            >
              {tab}
            </button>
          ))}
        </nav>
      </div>
      <div className="mt-4 grid gap-6">
        {activeTab === 'Users' && (
          <>
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div className="flex flex-wrap items-center gap-2">
                <input
                  type="search"
                  value={userSearch}
                  onChange={(e) => setUserSearch(e.target.value)}
                  placeholder="Search name or email"
                  aria-label="Search users"
                  className={`${UI_FILTER_SELECT_SM} w-64`}
                />
                <select
                  value={userRoleFilter}
                  onChange={(e) => setUserRoleFilter(e.target.value)}
                  aria-label="Filter by role"
                  className={UI_FILTER_SELECT_SM}
                >
                  <option value="">All roles</option>
                  {userRoleOptions.map((role) => (
                    <option key={role} value={role}>
                      {fortunaRoleShortLabel(role)}
                    </option>
                  ))}
                </select>
                <select
                  value={userStatusFilter}
                  onChange={(e) => setUserStatusFilter(e.target.value as 'all' | 'active' | 'disabled')}
                  aria-label="Filter by status"
                  className={UI_FILTER_SELECT_SM}
                >
                  <option value="all">Any status</option>
                  <option value="active">Active</option>
                  <option value="disabled">Disabled</option>
                </select>
                <span className="text-caption text-muted">
                  {visibleUsers.length === users.length
                    ? `${users.length} user${users.length === 1 ? '' : 's'}`
                    : `${visibleUsers.length} of ${users.length} users`}
                </span>
              </div>
              {canRegister && canUsersCreate && canUsers && (
                <Button
                  type="button"
                  size="sm"
                  onClick={() => {
                    setAddUserError('');
                    setNewUserScopeMode('all');
                    setNewUserScopeClusters([]);
                    setAddUserOpen(true);
                  }}
                >
                  <Plus className="w-4 h-4 mr-2" /> Add user
                </Button>
              )}
            </div>
            <Card className="p-0 overflow-hidden">
            {loadingUsers ? (
              <div className="p-8 text-muted">Loading users...</div>
            ) : usersError ? (
              <PageError
                title="Could not load users"
                description={usersError}
                className="py-8"
                action={<Button type="button" variant="secondary" size="sm" onClick={() => void loadUsers()}>Retry</Button>}
              />
            ) : users.length === 0 ? (
              <PageEmpty title="No users" description="No user records returned by /api/v1/users." className="py-8" />
            ) : (
              <div className="ui-table-scroll">
                <table className={UI_TABLE}>
                  <thead className={UI_THEAD_STICKY}>
                    <tr>
                      <th className={UI_TH}>User</th>
                      <th className={UI_TH}>Fortuna role</th>
                      <th className={UI_TH}>Cluster access</th>
                      <th className={UI_TH}>Status</th>
                      {(canUsersRowActions) && <th className={`${UI_TH} text-right`}>Actions</th>}
                    </tr>
                  </thead>
                  <tbody>
                    {visibleUsers.length === 0 ? (
                      <tr>
                        <td colSpan={canUsersRowActions ? 5 : 4} className={`${UI_TD} py-6 text-center text-muted`}>
                          No users match these filters.
                        </td>
                      </tr>
                    ) : null}
                    {visibleUsers.map((u) => (
                      <tr key={u.id} className={UI_TR}>
                        <td className={UI_TD}>
                          <div className="text-text font-medium">{u.username || u.name || 'unknown'}</div>
                          <div className="text-muted text-caption">{u.email || '—'}</div>
                        </td>
                        <td className={UI_TD}>
                          {(() => {
                            const rowRole = (u.role || 'viewer').toLowerCase();
                            // The server refuses a user admin changing their own role.
                            const ownRowLocked = !isFortunaAdmin && String(user?.id) === u.id;
                            if (canUsersRoleAssign && !userAdminCannotManage(u) && !ownRowLocked) {
                              const selVal = rowRole === 'user' ? 'operator' : rowRole;
                              return (
                                <select
                                  className="bg-base border border-border rounded px-2 py-1.5 text-body text-text min-w-[12rem] max-w-[20rem]"
                                  value={selVal}
                                  disabled={savingUserId === u.id}
                                  title="Fortuna application role (not Kubernetes RBAC)"
                                  onChange={(e) => {
                                    const next = e.target.value;
                                    void updateFortunaUser(u.id, { role: next });
                                  }}
                                >
                                  {/* A user admin may move an operator down to viewer but not assign operator. */}
                                  {(fortunaRoleEditOptions as readonly string[]).includes(selVal) ? null : (
                                    <option value={selVal} disabled>
                                      {fortunaRoleSelectLabel(selVal)}
                                    </option>
                                  )}
                                  {fortunaRoleEditOptions.map((r) => (
                                    <option key={r} value={r}>
                                      {fortunaRoleSelectLabel(r)}
                                    </option>
                                  ))}
                                </select>
                              );
                            }
                            return (
                              <div
                                className="flex items-start gap-2 text-muted"
                                title={fortunaRoleTooltip(u.role)}
                              >
                                <Shield
                                  className={`w-3 h-3 mt-0.5 shrink-0 ${
                                    rowRole === 'admin'
                                      ? 'text-brand'
                                      : rowRole === 'cluster_admin'
                                        ? 'text-info'
                                        : 'text-muted'
                                  }`}
                                />
                                <span className="text-body text-text">{fortunaRoleShortLabel(u.role)}</span>
                              </div>
                            );
                          })()}
                        </td>
                        <td className={UI_TD}>
                          {(() => {
                            const parsed = parseUserScopeClusters(u);
                            const rowRole = (u.role || 'viewer').toLowerCase();
                            const clusterNames = parsed.clusters
                              .map((id) => clusterById.get(id)?.name || id)
                              .join(', ');
                            const adminUnrestricted = rowRole === 'admin';
                            const label = adminUnrestricted
                              ? 'All clusters'
                              : parsed.invalid
                                ? 'Invalid scope'
                                : parsed.restricted
                                  ? parsed.clusters.length === 0
                                    ? 'No clusters'
                                    : `${parsed.clusters.length} cluster${parsed.clusters.length === 1 ? '' : 's'}`
                                  : 'All clusters';
                            return (
                              <div className="flex flex-wrap items-center gap-2">
                                <span
                                  className={`inline-flex rounded-full border px-2 py-0.5 text-caption ${
                                    parsed.invalid
                                      ? 'border-critical/30 bg-critical/10 text-critical'
                                      : parsed.restricted
                                        ? 'border-info/30 bg-info/10 text-info'
                                        : 'border-border bg-surface-2/60 text-muted'
                                  }`}
                                  title={parsed.restricted ? clusterNames || 'Selected clusters' : 'Unrestricted cluster access'}
                                >
                                  {label}
                                </span>
                                {parsed.restricted && !parsed.invalid ? (
                                  <span className="max-w-[14rem] truncate text-caption text-muted" title={clusterNames}>
                                    {clusterNames || 'Selected cluster ids'}
                                  </span>
                                ) : null}
                                {isFortunaAdmin && !adminUnrestricted ? (
                                  <button
                                    type="button"
                                    className="rounded px-2 py-1 text-caption text-brand transition-colors hover:bg-brand/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 disabled:cursor-not-allowed disabled:opacity-50"
                                    disabled={savingUserId === u.id || loadingClusters}
                                    onClick={() => openScopeEditor(u)}
                                  >
                                    Edit scope
                                  </button>
                                ) : null}
                              </div>
                            );
                          })()}
                        </td>
                        <td className={UI_TD}>
                          {(() => {
                            if (canUsersUpdate && !userAdminCannotManage(u)) {
                              return (
                                <label className="inline-flex items-center gap-2 text-body text-text cursor-pointer select-none">
                                  <input
                                    type="checkbox"
                                    className="rounded border-border"
                                    checked={u.active !== false}
                                    disabled={savingUserId === u.id}
                                    onChange={(e) => {
                                      void updateFortunaUser(u.id, { active: e.target.checked });
                                    }}
                                  />
                                  <span>{u.active === false ? 'disabled' : 'active'}</span>
                                </label>
                              );
                            }
                            return (
                              <span
                                className={`px-2 py-0.5 rounded-full text-caption border capitalize ${
                                  u.active === false ? 'bg-muted/20 text-muted border-border' : 'bg-success/10 text-success border-success/20'
                                }`}
                              >
                                {u.status || (u.active === false ? 'disabled' : 'active')}
                              </span>
                            );
                          })()}
                        </td>
                        {(canUsersRowActions) && (
                          <td className={`${UI_TD} text-right`}>
                            {canUsersDelete && String(user?.id) !== u.id && !userAdminCannotManage(u) && (
                              <button
                                type="button"
                                className="inline-flex min-h-10 min-w-10 items-center justify-center rounded-lg text-muted transition-colors hover:bg-critical/10 hover:text-critical focus:outline-none focus-visible:ring-2 focus-visible:ring-critical/60 disabled:cursor-not-allowed disabled:opacity-50 sm:min-h-8 sm:min-w-8"
                                aria-label={`Delete user ${u.username || u.id}`}
                                disabled={savingUserId === u.id}
                                onClick={() => {
                                  void confirm({
                                    title: 'Delete Fortuna user',
                                    description: `Delete user ${u.username || u.id}? This removes their dashboard account access.`,
                                    confirmLabel: 'Delete user',
                                    variant: 'danger',
                                  }).then((confirmed) => {
                                    if (confirmed) void removeFortunaUser(u.id);
                                  });
                                }}
                              >
                                <Trash2 className="w-4 h-4" />
                              </button>
                            )}
                          </td>
                        )}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
            <details className="rounded-lg border border-border/90 bg-surface-2/25 px-4 py-3 sm:px-5">
              <summary className="cursor-pointer select-none text-body font-medium text-text">What each role can do</summary>
              <ul className="mt-3 grid gap-3 sm:grid-cols-2 text-caption text-muted">
                {FORTUNA_ROLE_HELP_ROWS.map((row) => (
                  <li key={row.key} className="rounded-lg border border-border/70 bg-base/40 px-3 py-2.5">
                    <span className="font-semibold text-text">{row.title}</span>
                    <p className="mt-1.5 leading-snug">{row.body}</p>
                  </li>
                ))}
              </ul>
              <p className="text-caption text-muted mt-4 pt-3 border-t border-border leading-snug">
                <span className="font-medium text-text">How roles stack:</span> {FORTUNA_ROLE_SUMMARY}
              </p>
            </details>
          </>
        )}

        {activeTab === 'Sessions' && (
          <>
            <Card className="p-4 sm:p-5 border border-border/90 bg-surface-2/25">
              <h3 className="mb-2 text-card-title text-text">Sessions</h3>
              <p className="text-caption text-muted leading-snug">
                Manage JWT-bound dashboard sessions. Security activity, the platform audit log, and access analytics live on the Audit page.
              </p>
            </Card>

            {canSessionsRead && (
              <Card className="p-0 overflow-hidden">
                <div className="p-4 border-b border-border flex flex-wrap justify-between gap-3 items-start">
                  <div>
                    <h4 className="text-body font-semibold text-text">Sessions</h4>
                    <p className="text-caption text-muted mt-0.5">
                      JWT must reference an active server session when <span className="font-mono">user_sessions</span> is migrated. Revocation takes effect immediately.
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2 items-center">
                    {isFortunaAdmin && canUsersReadForSessions && (
                      <div className="flex items-center gap-2">
                        <label className="text-caption text-muted whitespace-nowrap">Filter by user id</label>
                        <input
                          className="bg-base border border-border rounded px-2 py-1.5 text-body w-28"
                          value={sessionUserIdFilter}
                          onChange={(e) => setSessionUserIdFilter(e.target.value.replace(/[^\d]/g, ''))}
                          placeholder="uid"
                        />
                        <Button size="sm" variant="secondary" type="button" onClick={() => void loadGovernance()}>
                          Refresh
                        </Button>
                      </div>
                    )}
                    {canSessionsRevoke && (
                      <Button size="sm" variant="secondary" type="button" onClick={() => void revokeAllMySessions()}>
                        Revoke all my sessions
                      </Button>
                    )}
                    {isFortunaAdmin && canSessionsRevokeAll && sessionUserIdFilter.trim() !== '' && (
                      <Button
                        size="sm"
                        variant="secondary"
                        type="button"
                        onClick={async () => {
                          const targetUserId = sessionUserIdFilter.trim();
                          const confirmed = await confirm({
                            title: 'Revoke all sessions for user',
                            description: `Revoke every session for Fortuna user id ${targetUserId}? They must sign in again.`,
                            confirmLabel: 'Revoke user sessions',
                            variant: 'danger',
                          });
                          if (!confirmed) return;
                          try {
                            await api.revokeAllUserSessions(targetUserId);
                            await loadGovernance();
                          } catch (e) {
                            toast({ title: 'Settings action failed', description: String(e instanceof Error ? e.message : e), variant: 'error' });
                          }
                        }}
                      >
                        Revoke all for user
                      </Button>
                    )}
                  </div>
                </div>
                {sessionsLoading ? (
                  <div className="p-8 text-muted">Loading sessions…</div>
                ) : sessionsError ? (
                  <PageEmpty title="Sessions unavailable" description={sessionsError} className="py-8" />
                ) : sessionRows.length === 0 ? (
                  <PageEmpty
                    title="No sessions"
                    description="No session rows returned (table may not be migrated yet, or filters exclude all rows)."
                    className="py-8"
                  />
                ) : (
                  <div className="ui-table-scroll">
                    <table className={UI_TABLE}>
                      <thead className={UI_THEAD_STICKY}>
                        <tr>
                          <th className={UI_TH}>Session</th>
                          <th className={UI_TH}>User</th>
                          <th className={UI_TH}>Issued</th>
                          <th className={UI_TH}>Expires</th>
                          <th className={UI_TH}>Last activity</th>
                          <th className={UI_TH}>IP</th>
                          <th className={UI_TH}>State</th>
                          {canSessionsRevoke && <th className={`${UI_TH} text-right`}>Actions</th>}
                        </tr>
                      </thead>
                      <tbody>
                        {sessionRows.map((s) => {
                          const isOwn = String(user?.id ?? '') === s.userId;
                          // Matches core: another user's session needs sessions.revoke_all.
                          const mayRevoke = canSessionsRevoke && (isOwn || canSessionsRevokeAll);
                          return (
                            <tr key={s.id} className={UI_TR}>
                              <td className={`${UI_TD} font-mono text-caption`}>{s.id.slice(0, 8)}…</td>
                              <td className={UI_TD}>{s.userId}</td>
                              <td className={`${UI_TD} text-caption whitespace-nowrap`}>
                                {s.issuedAt ? formatDateTime(s.issuedAt) : '—'}
                              </td>
                              <td className={`${UI_TD} text-caption whitespace-nowrap`}>
                                {s.expiresAt ? formatDateTime(s.expiresAt) : '—'}
                              </td>
                              <td className={`${UI_TD} text-caption whitespace-nowrap`}>
                                {s.lastActivityAt ? formatDateTime(s.lastActivityAt) : '—'}
                              </td>
                              <td className={`${UI_TD} text-caption`}>{s.sourceIp || '—'}</td>
                              <td className={UI_TD}>
                                {s.revokedAt ? (
                                  <span className="text-warning text-caption">revoked</span>
                                ) : (
                                  <span className="text-success text-caption">active</span>
                                )}
                              </td>
                              {canSessionsRevoke && (
                                <td className={`${UI_TD} text-right`}>
                                  {mayRevoke && !s.revokedAt ? (
                                    <button
                                      type="button"
                                      className="rounded px-2 py-1 text-caption text-muted transition-colors hover:bg-critical/10 hover:text-critical focus:outline-none focus-visible:ring-2 focus-visible:ring-critical/60 disabled:cursor-not-allowed disabled:opacity-50"
                                      aria-label={`Revoke session ${s.id}`}
                                      disabled={revokingSessionId === s.id}
                                      onClick={() => void revokeFortunaSession(s.id)}
                                    >
                                      {revokingSessionId === s.id ? '…' : 'Revoke'}
                                    </button>
                                  ) : (
                                    <span className="text-caption text-muted">—</span>
                                  )}
                                </td>
                              )}
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
              </Card>
            )}
          </>
        )}

      </div>

      <Dialog
        open={addUserOpen}
        title="Add Fortuna user"
        description="Creates a dashboard user via registration. Password must meet server policy (at least 12 characters)."
        size="sm"
        closeDisabled={addUserBusy}
        onClose={() => setAddUserOpen(false)}
        footer={
          <>
            <Button type="button" variant="secondary" disabled={addUserBusy} onClick={() => setAddUserOpen(false)}>
              Cancel
            </Button>
            <Button
              type="button"
              isLoading={addUserBusy}
              disabled={addUserBusy || (newUserRole.toLowerCase() !== 'admin' && newUserScopeMode === 'selected' && Boolean(clusterAvailabilityIssue))}
              onClick={() => void registerFortunaUser()}
            >
              Create user
            </Button>
          </>
        }
      >
            <div className="space-y-4">
              {addUserError ? (
                <div className="p-3 rounded bg-critical/10 border border-critical/30 text-critical text-body">{addUserError}</div>
              ) : null}
              <div>
                <label htmlFor="settings-new-username" className="block text-caption text-muted mb-1">Username</label>
                <input
                  id="settings-new-username"
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  autoComplete="off"
                  disabled={addUserBusy}
                />
              </div>
              <div>
                <label htmlFor="settings-new-email" className="block text-caption text-muted mb-1">Email</label>
                <input
                  id="settings-new-email"
                  type="email"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  autoComplete="off"
                  disabled={addUserBusy}
                />
              </div>
              <div>
                <label htmlFor="settings-new-password" className="block text-caption text-muted mb-1">Password</label>
                <input
                  id="settings-new-password"
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  autoComplete="new-password"
                  disabled={addUserBusy}
                />
              </div>
              <div>
                <label htmlFor="settings-new-role" className="block text-caption text-muted mb-1">Fortuna role</label>
                <select
                  id="settings-new-role"
                  className="w-full bg-base border border-border rounded px-3 py-2 text-body text-text"
                  value={(fortunaRoleEditOptions as readonly string[]).includes(newUserRole) ? newUserRole : fortunaRoleEditOptions[0]}
                  onChange={(e) => setNewUserRole(e.target.value)}
                  disabled={addUserBusy}
                >
                  {fortunaRoleEditOptions.map((r) => (
                    <option key={r} value={r}>
                      {fortunaRoleSelectLabel(r)}
                    </option>
                  ))}
                </select>
              </div>
              {isFortunaAdmin ? (
                <div className="space-y-3 rounded-lg border border-border bg-base/35 p-3">
                  <div>
                    <div className="text-caption font-semibold uppercase tracking-wide text-muted">Cluster access</div>
                    <p className="mt-1 text-caption text-muted">
                      Scope applies to security data after the user signs in. Admin role remains unrestricted by server policy.
                    </p>
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <label className="flex cursor-pointer items-start gap-2 rounded-lg border border-border bg-surface/50 p-2.5 text-body text-text">
                      <input
                        type="radio"
                        name="settings-new-user-scope-mode"
                        className="mt-1"
                        checked={newUserScopeMode === 'all'}
                        disabled={addUserBusy}
                        onChange={() => setNewUserScopeMode('all')}
                      />
                      <span>
                        <span className="block font-semibold">All clusters</span>
                        <span className="block text-caption text-muted">Current and future clusters.</span>
                      </span>
                    </label>
                    <label className="flex cursor-pointer items-start gap-2 rounded-lg border border-border bg-surface/50 p-2.5 text-body text-text">
                      <input
                        type="radio"
                        name="settings-new-user-scope-mode"
                        className="mt-1"
                        checked={newUserScopeMode === 'selected'}
                        disabled={addUserBusy}
                        onChange={() => setNewUserScopeMode('selected')}
                      />
                      <span>
                        <span className="block font-semibold">Selected clusters</span>
                        <span className="block text-caption text-muted">Restrict by cluster_id allow-list. With none selected, the account sees no cluster until you add one.</span>
                      </span>
                    </label>
                  </div>
                  {newUserRole.toLowerCase() === 'admin' ? (
                    <div className="rounded border border-border bg-surface/50 p-2 text-caption text-muted">
                      Platform admin is always unrestricted. Cluster scope is applied to viewer, operator, and user admin accounts.
                    </div>
                  ) : loadingClusters && clusters.length === 0 ? (
                    <div className="text-caption text-muted">Loading clusters…</div>
                  ) : clusterAvailabilityIssue && clusters.length === 0 ? (
                    <AvailabilityNotice
                      issue={clusterAvailabilityIssue}
                      onRetry={clusterAvailabilityIssue.retryable ? () => void loadClustersForScope() : undefined}
                    />
                  ) : clusters.length === 0 ? (
                    <div className="rounded border border-border bg-surface/50 p-2 text-caption text-muted">
                      No active clusters returned by /inventory/clusters/stats.
                    </div>
                  ) : (
                    <div className={newUserScopeMode === 'selected' ? 'grid gap-2' : 'grid gap-2 opacity-55'}>
                      {clusters.map((cluster) => (
                        <label key={cluster.id} className="flex cursor-pointer items-start gap-2 rounded border border-border bg-surface/50 p-2">
                          <input
                            type="checkbox"
                            className="mt-1"
                            checked={newUserScopeClusters.includes(cluster.id)}
                            disabled={addUserBusy || newUserScopeMode !== 'selected' || Boolean(clusterAvailabilityIssue)}
                            onChange={(event) => {
                              const checked = event.target.checked;
                              setNewUserScopeClusters((current) =>
                                checked
                                  ? Array.from(new Set([...current, cluster.id]))
                                  : current.filter((id) => id !== cluster.id),
                              );
                            }}
                          />
                          <span className="min-w-0">
                            <span className="block truncate text-body font-semibold text-text" title={clusterDisplayName(cluster)}>
                              {cluster.name || cluster.id}
                            </span>
                            <span className="block break-all font-mono text-[11px] text-muted">{cluster.id}</span>
                          </span>
                        </label>
                      ))}
                    </div>
                  )}
                </div>
              ) : null}
            </div>
      </Dialog>

      <Dialog
        open={!!scopeEditorUser}
        title="Edit cluster access"
        description="Cluster access limits which monitored clusters this user can see in inventory, runtime, findings, attack paths, and reports. Platform admins remain unrestricted."
        size="lg"
        closeDisabled={scopeSaving}
        onClose={() => setScopeEditorUser(null)}
        footer={
          <>
            <Button type="button" variant="secondary" disabled={scopeSaving} onClick={() => setScopeEditorUser(null)}>
              Cancel
            </Button>
            <Button
              type="button"
              isLoading={scopeSaving}
              disabled={scopeSaving || (scopeMode === 'selected' && Boolean(clusterAvailabilityIssue))}
              onClick={() => void saveScopeEditor()}
            >
              Save access
            </Button>
          </>
        }
      >
        <div className="space-y-5">
          {scopeEditorUser ? (
            <div className="rounded-lg border border-border bg-base/40 px-3 py-2.5">
              <div className="text-body font-semibold text-text">{scopeEditorUser.username}</div>
              <div className="mt-1 text-caption text-muted">
                {scopeEditorUser.email || 'No email'} · {fortunaRoleShortLabel(scopeEditorUser.role)}
              </div>
              {scopeEditorUser.permissions?.length ? (
                <div className="mt-3 border-t border-border pt-3">
                  <div className="mb-2 text-caption font-semibold uppercase tracking-wide text-muted">Effective permissions</div>
                  <div className="flex flex-wrap gap-1.5">
                    {scopeEditorUser.permissions.slice(0, 10).map((permission) => (
                      <span
                        key={permission}
                        className="rounded border border-border bg-surface-2/70 px-2 py-0.5 font-mono text-[11px] text-muted"
                      >
                        {permission}
                      </span>
                    ))}
                    {scopeEditorUser.permissions.length > 10 ? (
                      <span className="rounded border border-border bg-surface-2/70 px-2 py-0.5 text-[11px] text-muted">
                        +{scopeEditorUser.permissions.length - 10} more
                      </span>
                    ) : null}
                  </div>
                </div>
              ) : null}
            </div>
          ) : null}

          {scopeError ? (
            <div className="rounded-lg border border-critical/30 bg-critical/10 p-3 text-body text-critical">{scopeError}</div>
          ) : null}

          <fieldset className="space-y-3">
            <legend className="text-caption font-semibold uppercase tracking-wide text-muted">Access mode</legend>
            <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-base/35 p-3 text-body text-text">
              <input
                type="radio"
                name="settings-cluster-scope-mode"
                className="mt-1"
                checked={scopeMode === 'all'}
                disabled={scopeSaving}
                onChange={() => setScopeMode('all')}
              />
              <span>
                <span className="block font-semibold">All clusters</span>
                <span className="mt-1 block text-caption text-muted">
                  User can access every current and future monitored cluster allowed by their Fortuna role.
                </span>
              </span>
            </label>
            <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-base/35 p-3 text-body text-text">
              <input
                type="radio"
                name="settings-cluster-scope-mode"
                className="mt-1"
                checked={scopeMode === 'selected'}
                disabled={scopeSaving}
                onChange={() => setScopeMode('selected')}
              />
              <span>
                <span className="block font-semibold">Selected clusters</span>
                <span className="mt-1 block text-caption text-muted">
                  User only sees security data whose cluster_id is in this allow-list.
                </span>
              </span>
            </label>
          </fieldset>

          <div className={scopeMode === 'selected' ? 'space-y-3' : 'space-y-3 opacity-55'}>
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-body font-semibold text-text">Cluster allow-list</h3>
              {scopeMode === 'selected' ? (
                <span className="text-caption text-muted">
                  {selectedScopeClusters.length} selected
                </span>
              ) : null}
            </div>
            {loadingClusters && clusters.length === 0 ? (
              <div className="rounded-lg border border-border bg-base/35 p-4 text-body text-muted">Loading clusters…</div>
            ) : clusterAvailabilityIssue && clusters.length === 0 ? (
              <AvailabilityNotice
                issue={clusterAvailabilityIssue}
                onRetry={clusterAvailabilityIssue.retryable ? () => void loadClustersForScope() : undefined}
              />
            ) : (
              <>
                {clusterAvailabilityIssue ? (
                  <AvailabilityNotice
                    issue={clusterAvailabilityIssue}
                    onRetry={clusterAvailabilityIssue.retryable ? () => void loadClustersForScope() : undefined}
                    className="mb-3"
                  />
                ) : null}
                {clusters.length === 0 ? (
              <PageEmpty
                title="No clusters available"
                description="No active synced clusters were returned by /inventory/clusters/stats."
                className="rounded-lg border border-border py-6"
              />
                ) : (
                  <div className="grid gap-2 sm:grid-cols-2">
                    {clusters.map((cluster) => {
                      const checked = selectedScopeClusters.includes(cluster.id);
                      return (
                        <label
                          key={cluster.id}
                          className="flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-base/35 p-3"
                        >
                          <input
                            type="checkbox"
                            className="mt-1"
                            checked={checked}
                            disabled={scopeSaving || scopeMode !== 'selected' || Boolean(clusterAvailabilityIssue)}
                            onChange={(event) => {
                              const nextChecked = event.target.checked;
                              setSelectedScopeClusters((current) =>
                                nextChecked
                                  ? Array.from(new Set([...current, cluster.id]))
                                  : current.filter((id) => id !== cluster.id),
                              );
                            }}
                          />
                          <span className="min-w-0">
                            <span className="block truncate text-body font-semibold text-text" title={clusterDisplayName(cluster)}>
                              {cluster.name || cluster.id}
                            </span>
                            <span className="mt-1 block break-all font-mono text-[11px] text-muted">{cluster.id}</span>
                            <span className="mt-1 block text-caption text-muted">
                              {cluster.podCount ?? cluster.pods ?? 0} pods · {cluster.connectionStatus || cluster.status || 'unknown'}
                            </span>
                          </span>
                        </label>
                      );
                    })}
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      </Dialog>
    </PageLayout>
  );
};
