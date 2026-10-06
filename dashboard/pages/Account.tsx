import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card } from '../design-system/components/Card';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty } from '../design-system/components/PageStatus';
import { useConfirm } from '../design-system/components/ConfirmDialog';
import { useToast } from '../design-system/components/Toast';
import { Button } from '../components/ui/Button';
import { api, isApiError } from '../lib/api';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { fortunaRoleShortLabel } from '../lib/fortunaRoles';
import { formatDateTime } from '../lib/display';
import { UI_AUTH_INPUT } from '../lib/formChrome';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH, UI_TR, UI_TD } from '../lib/tableChrome';
import { PAGE_TITLES } from '../lib/pageTitles';
import { useAuthStore } from '../store/authStore';
import type { FortunaUserSession } from '../types';

/** Session id ("sid") of the signed-in JWT, used only to label "This session". */
function currentSessionId(token: string | null): string | null {
  const payload = token?.split('.')[1];
  if (!payload) return null;
  try {
    const json = JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')));
    return typeof json.sid === 'string' ? json.sid : null;
  } catch {
    return null;
  }
}

/** The signed-in user's own profile, password and sessions; open to every role. */
export const Account: React.FC = () => {
  const user = useAuthStore((s) => s.user);
  const token = useAuthStore((s) => s.token);
  const updateUser = useAuthStore((s) => s.updateUser);
  const logout = useAuthStore((s) => s.logout);
  const permUser = usePermUser();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const toast = useToast();

  const canChangePassword = can(permUser, P.authPasswordChange);
  const canReadSessions = can(permUser, P.sessionsRead);
  const canRevokeSessions = can(permUser, P.sessionsRevoke);
  // Without user_id, Core lists every user's sessions for an admin, so ask for our own.
  const sessionsUserId = can(permUser, P.usersRead) ? String(user?.id ?? '') : undefined;
  const thisSessionId = useMemo(() => currentSessionId(token), [token]);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [passwordError, setPasswordError] = useState('');
  const [savingPassword, setSavingPassword] = useState(false);

  const [sessions, setSessions] = useState<FortunaUserSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [sessionsError, setSessionsError] = useState<string | null>(null);
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const loadSessions = useCallback(async () => {
    if (!canReadSessions) return;
    setSessionsLoading(true);
    setSessionsError(null);
    try {
      const rows = await api.listUserSessions(sessionsUserId);
      const ownId = String(user?.id ?? '');
      setSessions(rows.filter((s) => !s.revokedAt && (!ownId || s.userId === ownId)));
    } catch (e) {
      setSessionsError(isApiError(e) && e.message ? e.message : 'Your sessions could not be loaded.');
    } finally {
      setSessionsLoading(false);
    }
  }, [canReadSessions, sessionsUserId, user?.id]);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  const signOutHere = () => {
    logout();
    navigate('/login');
  };

  const handlePasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError('');
    if (newPassword !== confirmPassword) {
      setPasswordError('New passwords do not match.');
      return;
    }
    if (newPassword === currentPassword) {
      setPasswordError('Choose a password that is different from the current one.');
      return;
    }
    setSavingPassword(true);
    try {
      const updated = await api.changePassword(currentPassword, newPassword);
      updateUser({ ...(user ?? {}), ...updated, mustChangePassword: false, bootstrapCredential: false });
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      toast({ title: 'Password changed', variant: 'success' });
    } catch (err) {
      setPasswordError(
        isApiError(err) && err.message
          ? err.message
          : 'Could not change the password. Check the current password and the password rules.',
      );
    } finally {
      setSavingPassword(false);
    }
  };

  const revokeSession = async (session: FortunaUserSession) => {
    const isThis = session.id === thisSessionId;
    const ok = await confirm({
      title: isThis ? 'Sign out of this session?' : 'Sign out this session?',
      description: isThis
        ? 'You will be signed out here and need to sign in again.'
        : 'Whoever uses this session will need to sign in again.',
      confirmLabel: 'Sign out session',
      variant: 'danger',
    });
    if (!ok) return;
    setRevokingId(session.id);
    try {
      await api.revokeUserSession(session.id);
      if (isThis) {
        signOutHere();
        return;
      }
      await loadSessions();
    } catch (e) {
      toast({ title: 'Could not sign out the session', description: String(e instanceof Error ? e.message : e), variant: 'error' });
    } finally {
      setRevokingId(null);
    }
  };

  const revokeAll = async () => {
    const ok = await confirm({
      title: 'Sign out everywhere?',
      description: 'Every session of your account ends, including this one. You will need to sign in again.',
      confirmLabel: 'Sign out everywhere',
      variant: 'danger',
    });
    if (!ok) return;
    try {
      await api.revokeAllUserSessions();
      signOutHere();
    } catch (e) {
      toast({ title: 'Could not sign out your sessions', description: String(e instanceof Error ? e.message : e), variant: 'error' });
    }
  };

  const scope = user?.operationalScope;
  const clusterAccess = !scope?.restricted
    ? 'All clusters'
    : scope.clusters.length > 0
      ? scope.clusters.join(', ')
      : 'No clusters assigned';

  return (
    <PageLayout title={PAGE_TITLES.account} description="Your profile, password and signed-in sessions.">
      <div className="grid gap-5 xl:grid-cols-2">
        <Card title="Profile">
          <dl className="grid grid-cols-[8rem_minmax(0,1fr)] gap-x-4 gap-y-3 text-body">
            <dt className="text-muted">Username</dt>
            <dd className="min-w-0 truncate text-text">{user?.username || '—'}</dd>
            <dt className="text-muted">Email</dt>
            <dd className="min-w-0 truncate text-text">{user?.email || '—'}</dd>
            <dt className="text-muted">Role</dt>
            <dd className="text-text">{fortunaRoleShortLabel(user?.role)}</dd>
            <dt className="text-muted">Cluster access</dt>
            <dd className="min-w-0 break-words text-text">{clusterAccess}</dd>
            {user?.passwordChangedAt ? (
              <>
                <dt className="text-muted">Password changed</dt>
                <dd className="text-text">{formatDateTime(user.passwordChangedAt)}</dd>
              </>
            ) : null}
          </dl>
          <p className="mt-4 text-caption text-muted">Your role and cluster access are set by an administrator in Users &amp; Access.</p>
        </Card>

        {canChangePassword ? (
          <Card title="Change password">
            <form onSubmit={handlePasswordSubmit} className="space-y-4">
              {passwordError ? (
                <div className="rounded-lg border border-error-border bg-error-background p-3 text-body text-error-foreground" role="alert">
                  {passwordError}
                </div>
              ) : null}
              <div>
                <label htmlFor="account-current-password" className="mb-1 block text-body font-medium text-text">Current password</label>
                <input
                  id="account-current-password"
                  type="password"
                  required
                  autoComplete="current-password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  className={UI_AUTH_INPUT}
                />
              </div>
              <div>
                <label htmlFor="account-new-password" className="mb-1 block text-body font-medium text-text">New password</label>
                <input
                  id="account-new-password"
                  type="password"
                  required
                  minLength={12}
                  autoComplete="new-password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className={UI_AUTH_INPUT}
                  placeholder="At least 12 characters"
                />
              </div>
              <div>
                <label htmlFor="account-confirm-password" className="mb-1 block text-body font-medium text-text">Confirm new password</label>
                <input
                  id="account-confirm-password"
                  type="password"
                  required
                  minLength={12}
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className={UI_AUTH_INPUT}
                />
              </div>
              <Button type="submit" isLoading={savingPassword}>Change password</Button>
            </form>
          </Card>
        ) : null}
      </div>

      {canReadSessions ? (
        <Card
          className="mt-5"
          title="Your sessions"
          description="Places where your account is signed in. Sign out any you do not recognise."
          actions={
            canRevokeSessions && sessions.length > 0 ? (
              <Button size="sm" variant="secondary" type="button" onClick={() => void revokeAll()}>
                Sign out everywhere
              </Button>
            ) : null
          }
        >
          {sessionsLoading ? (
            <p className="py-6 text-body text-muted">Loading sessions…</p>
          ) : sessionsError ? (
            <PageEmpty
              title="Sessions unavailable"
              description={sessionsError}
              action={<Button size="sm" variant="secondary" onClick={() => void loadSessions()}>Try again</Button>}
              className="py-6"
            />
          ) : sessions.length === 0 ? (
            <PageEmpty title="No active sessions" description="Session tracking may not be enabled on this deployment." className="py-6" />
          ) : (
            <div className="ui-table-scroll">
              <table className={UI_TABLE}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH}>Signed in</th>
                    <th className={UI_TH}>Last active</th>
                    <th className={UI_TH}>IP address</th>
                    <th className={UI_TH}>Expires</th>
                    {canRevokeSessions ? <th className={`${UI_TH} text-right`}>Actions</th> : null}
                  </tr>
                </thead>
                <tbody>
                  {sessions.map((s) => (
                    <tr key={s.id} className={UI_TR}>
                      <td className={`${UI_TD} whitespace-nowrap`}>
                        {s.issuedAt ? formatDateTime(s.issuedAt) : '—'}
                        {s.id === thisSessionId ? (
                          <span className="ml-2 rounded border border-brand/40 bg-brand/10 px-1.5 py-0.5 text-meta text-brand">This session</span>
                        ) : null}
                      </td>
                      <td className={`${UI_TD} whitespace-nowrap`}>{s.lastActivityAt ? formatDateTime(s.lastActivityAt) : '—'}</td>
                      <td className={UI_TD}>{s.sourceIp || '—'}</td>
                      <td className={`${UI_TD} whitespace-nowrap`}>{s.expiresAt ? formatDateTime(s.expiresAt) : '—'}</td>
                      {canRevokeSessions ? (
                        <td className={`${UI_TD} text-right`}>
                          <Button
                            size="sm"
                            variant="ghost"
                            type="button"
                            disabled={revokingId === s.id}
                            onClick={() => void revokeSession(s)}
                            aria-label={`Sign out session started ${s.issuedAt ? formatDateTime(s.issuedAt) : ''}`}
                          >
                            Sign out
                          </Button>
                        </td>
                      ) : null}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      ) : null}
    </PageLayout>
  );
};
