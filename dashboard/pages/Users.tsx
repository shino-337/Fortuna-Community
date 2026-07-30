import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../lib/api';
import { User } from '../types';
import { Card } from '../design-system/components/Card';
import { PageEmpty, PageError, PageLoading } from '../design-system/components/PageStatus';
import { Tabs } from '../design-system/components/Tabs';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../lib/tableChrome';
import { UserPlus, Mail, Shield, Key, CheckCircle, XCircle, MoreVertical, RefreshCw } from 'lucide-react';
import { PAGE_TITLES } from '../lib/pageTitles';

const ROLE_ROWS = [
  {
    id: 'admin',
    label: 'Admin',
    description: 'Full platform administration, governance, and response access.',
    permissions: ['View all resources', 'Manage clusters', 'Manage users'],
  },
  {
    id: 'user_admin',
    label: 'User admin',
    description: 'Manage Fortuna application users and role assignments.',
    permissions: ['View assigned resources', 'Manage users', 'Review access changes'],
  },
  {
    id: 'operator',
    label: 'Operator',
    description: 'Triage findings, run response workflows, and manage investigations.',
    permissions: ['View assigned resources', 'Run response actions', 'Manage investigations'],
  },
  {
    id: 'viewer',
    label: 'Viewer',
    description: 'Read-only scoped exposure, evidence, and shared investigation access.',
    permissions: ['View assigned resources', 'Read evidence', 'No destructive actions'],
  },
];

const tabs = [
  { id: 'users', label: 'Users' },
  { id: 'roles', label: 'Roles' },
  { id: 'api-keys', label: 'API keys' },
];

function roleLabel(role?: string): string {
  return ROLE_ROWS.find((row) => row.id === role)?.label ?? (role ? role.replace(/_/g, ' ') : 'Unassigned');
}

export const Users: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState('users');

  const loadUsers = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getUsers();
      setUsers(data);
    } catch (e) {
      setUsers([]);
      setError(e instanceof Error ? e.message : 'Failed to load users');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  const activeUsers = useMemo(() => users.filter((user) => user.status !== 'disabled' && user.active !== false).length, [users]);

  return (
    <PageLayout
      title={PAGE_TITLES.users}
      description="Fortuna application accounts and role metadata. Kubernetes service accounts live under Resources."
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="secondary" onClick={() => void loadUsers()} isLoading={loading}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Refresh
          </Button>
          <Button disabled title="User invitation is managed from Settings in this build">
            <UserPlus className="mr-2 h-4 w-4" />
            Invite user
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <Tabs
          variant="underline"
          items={tabs}
          value={activeTab}
          onChange={setActiveTab}
          ariaLabel="User management sections"
          getPanelId={(id) => `users-panel-${id}`}
        />

        {activeTab === 'users' ? (
          <section id="users-panel-users" role="tabpanel" aria-labelledby="resources-tab-users">
            <div className="mb-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <Card variant="panel" className="p-3">
                <p className="text-caption text-muted">Total users</p>
                <p className="mt-1 text-metric text-text">{loading ? '...' : users.length}</p>
              </Card>
              <Card variant="panel" className="p-3">
                <p className="text-caption text-muted">Active users</p>
                <p className="mt-1 text-metric text-text">{loading ? '...' : activeUsers}</p>
              </Card>
            </div>

            {loading ? (
              <Card className="p-0">
                <PageLoading message="Loading users..." className="min-h-[16rem]" />
              </Card>
            ) : error ? (
              <PageError
                title="Could not load users"
                description={error}
                action={<Button variant="secondary" onClick={() => void loadUsers()}>Retry</Button>}
              />
            ) : users.length === 0 ? (
              <PageEmpty
                title="No Fortuna users yet"
                description="Create the first account from Settings or verify that Core is connected to the Fortuna users table."
                action={<Button variant="secondary" onClick={() => void loadUsers()}>Refresh users</Button>}
              />
            ) : (
              <Card className="p-0 overflow-hidden">
                <div className="ui-table-scroll">
                  <table className={UI_TABLE}>
                    <thead className={UI_THEAD_STICKY}>
                      <tr>
                        <th className={UI_TH}>User</th>
                        <th className={UI_TH}>Role</th>
                        <th className={UI_TH}>Status</th>
                        <th className={UI_TH}>Last login</th>
                        <th className={`${UI_TH} text-right`}>Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {users.map((user) => {
                        const displayName = user.name || user.username || user.email || user.id;
                        const initial = displayName.trim().charAt(0).toUpperCase() || '?';
                        const status = user.status || (user.active === false ? 'disabled' : 'active');
                        return (
                          <tr key={user.id} className={UI_TR}>
                            <td className={UI_TD}>
                              <div className="flex min-w-0 items-center">
                                <div className="mr-3 flex h-10 w-10 shrink-0 items-center justify-center rounded-full border border-border bg-surface-2 text-caption font-semibold text-muted">
                                  {initial}
                                </div>
                                <div className="min-w-0">
                                  <div className="truncate font-medium text-text">{displayName}</div>
                                  <div className="mt-0.5 flex min-w-0 items-center text-caption text-muted">
                                    <Mail className="mr-1 h-3 w-3 shrink-0" aria-hidden />
                                    <span className="truncate">{user.email || user.username}</span>
                                  </div>
                                </div>
                              </div>
                            </td>
                            <td className={UI_TD}>
                              <div className="flex items-center">
                                <Shield className={`mr-2 h-4 w-4 ${user.role === 'admin' ? 'text-brand' : 'text-muted'}`} aria-hidden />
                                <span className="capitalize text-text">{roleLabel(user.role)}</span>
                              </div>
                            </td>
                            <td className={UI_TD}>
                              <span className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-caption font-medium capitalize ${
                                status === 'disabled'
                                  ? 'border-border bg-surface-2 text-muted'
                                  : 'border-success/30 bg-success/10 text-success'
                              }`}>
                                {status}
                              </span>
                            </td>
                            <td className={`${UI_TD} font-mono text-caption text-muted`}>
                              {user.lastLogin ?? 'Never signed in'}
                            </td>
                            <td className={`${UI_TD} text-right`}>
                              <Button
                                variant="ghost"
                                size="touch"
                                aria-label={`Open actions for ${displayName}`}
                                title="User actions are managed from Settings"
                                disabled
                              >
                                <MoreVertical className="h-5 w-5" aria-hidden />
                              </Button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </Card>
            )}
          </section>
        ) : null}

        {activeTab === 'roles' ? (
          <section id="users-panel-roles" role="tabpanel" aria-labelledby="resources-tab-roles">
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              {ROLE_ROWS.map((role) => (
                <Card key={role.id} title={`${role.label} role`} description={role.description}>
                  <ul className="mt-2 space-y-3">
                    {role.permissions.map((permission) => (
                      <li key={permission} className="flex items-center text-body text-text">
                        {permission.startsWith('No ') ? (
                          <XCircle className="mr-3 h-4 w-4 text-muted-2" aria-hidden />
                        ) : (
                          <CheckCircle className="mr-3 h-4 w-4 text-success" aria-hidden />
                        )}
                        {permission}
                      </li>
                    ))}
                  </ul>
                </Card>
              ))}
            </div>
          </section>
        ) : null}

        {activeTab === 'api-keys' ? (
          <section id="users-panel-api-keys" role="tabpanel" aria-labelledby="resources-tab-api-keys">
            <Card>
              <PageEmpty
                icon={<Key className="h-10 w-10 text-muted-2 opacity-60" aria-hidden />}
                title="No API key management in this view"
                description="API key administration is not enabled in this dashboard route. Use Settings for available account controls."
              />
            </Card>
          </section>
        ) : null}
      </div>
    </PageLayout>
  );
};
