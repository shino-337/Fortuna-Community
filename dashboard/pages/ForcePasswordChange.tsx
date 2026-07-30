import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../components/ui/Button';
import { api, isApiError } from '../lib/api';
import { UI_AUTH_INPUT } from '../lib/formChrome';
import { resolvePersona } from '../lib/persona';
import { useAuthStore } from '../store/authStore';

export const ForcePasswordChange: React.FC = () => {
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const user = useAuthStore((state) => state.user);
  const updateUser = useAuthStore((state) => state.updateUser);
  const logout = useAuthStore((state) => state.logout);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    if (newPassword !== confirmPassword) {
      setError('New passwords do not match.');
      return;
    }
    if (newPassword === currentPassword) {
      setError('Choose a new password that is different from the bootstrap password.');
      return;
    }
    setIsSaving(true);
    try {
      const updated = await api.changePassword(currentPassword, newPassword);
      const nextUser = { ...(user ?? {}), ...updated, mustChangePassword: false, bootstrapCredential: false };
      updateUser(nextUser);
      navigate(resolvePersona(nextUser).profile.homeRoute, { replace: true });
    } catch (err) {
      if (isApiError(err) && err.message) {
        setError(err.message);
      } else {
        setError('Could not change password. Check the current password and policy requirements.');
      }
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="min-h-dvh min-w-0 bg-base flex flex-col items-center justify-center p-4">
      <div className="w-full max-w-md rounded-2xl border border-border bg-surface p-8 shadow-xl">
        <div className="mb-6 flex flex-col items-center text-center">
          <img src="/logo.png" alt="Fortuna" className="mb-4 h-12 w-12" />
          <h1 className="text-page-title text-text">Change bootstrap password</h1>
          <p className="mt-2 text-body text-muted">
            This admin account was created with bootstrap credentials. Change the password before using Fortuna.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          {error && (
            <div className="rounded-lg border border-error-border bg-error-background p-3 text-body text-error-foreground" role="alert">
              {error}
            </div>
          )}

          <div>
            <label htmlFor="current-password" className="mb-1 block text-body font-medium text-text">
              Current password
            </label>
            <input
              id="current-password"
              type="password"
              required
              autoComplete="current-password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              className={UI_AUTH_INPUT}
              placeholder="Bootstrap password"
            />
          </div>

          <div>
            <label htmlFor="new-password" className="mb-1 block text-body font-medium text-text">
              New password
            </label>
            <input
              id="new-password"
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
            <label htmlFor="confirm-password" className="mb-1 block text-body font-medium text-text">
              Confirm new password
            </label>
            <input
              id="confirm-password"
              type="password"
              required
              minLength={12}
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className={UI_AUTH_INPUT}
              placeholder="Repeat new password"
            />
          </div>

          <Button type="submit" className="w-full py-3" isLoading={isSaving}>
            Change password
          </Button>
          <Button type="button" variant="ghost" className="w-full" onClick={logout}>
            Sign out
          </Button>
        </form>
      </div>
    </div>
  );
};
