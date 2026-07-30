
import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';
import { api } from '../lib/api';
import { Button } from '../components/ui/Button';
import { UI_AUTH_INPUT } from '../lib/formChrome';
import { resolvePersona } from '../lib/persona';
import { isApiError } from '../lib/api';

export const Login: React.FC = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [fieldErrors, setFieldErrors] = useState<{ username?: string; password?: string }>({});
  const login = useAuthStore((state) => state.login);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const nextFieldErrors: typeof fieldErrors = {};
    if (!username.trim()) nextFieldErrors.username = 'Enter your Fortuna username or email.';
    if (!password) nextFieldErrors.password = 'Enter your password.';
    setFieldErrors(nextFieldErrors);
    if (Object.keys(nextFieldErrors).length > 0) return;

    setIsLoading(true);
    setError('');

    try {
      const { user, token } = await api.login(username, password);
      login(user, token);
      navigate(resolvePersona(user).profile.homeRoute);
    } catch (err) {
      if (isApiError(err)) {
        if (err.status === 401) {
          setError('Username or password is incorrect. Check the account and try again.');
        } else if (err.status === 429) {
          setError('Too many sign-in attempts. Wait a moment before trying again.');
        } else if (err.status >= 500) {
          setError('Fortuna authentication service is unavailable. Check Core health and try again.');
        } else {
          setError(err.message || 'Sign-in failed. Review the form and try again.');
        }
      } else if (err instanceof TypeError) {
        setError('Network error. Confirm the dashboard can reach the Core API.');
      } else {
        setError('Sign-in failed. Try again or contact an administrator.');
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-dvh min-w-0 bg-base flex flex-col items-center justify-center p-4">
      <div className="w-full max-w-md bg-surface rounded-2xl shadow-xl p-8 border border-border">
        <div className="flex flex-col items-center mb-8">
          <img src="/logo.png" alt="Fortuna" className="w-12 h-12 mb-4" />
          <h1 className="text-page-title text-text">Welcome back</h1>
          <p className="text-muted mt-2">Sign in to access Fortuna</p>
          <p className="text-meta text-muted-2 text-center mt-3 max-w-sm leading-snug">
            Access is controlled by <strong className="text-muted">Fortuna application roles</strong> (Admin, User admin, Operator, Viewer). These are not the same as Kubernetes Role or ClusterRole objects in your clusters.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-6">
          {error && (
            <div className="rounded-lg border border-error-border bg-error-background p-3 text-body text-error-foreground" role="alert">
              {error}
            </div>
          )}

          <div>
            <label htmlFor="login-username" className="block text-body font-medium text-text mb-1">
              Username or email
            </label>
            <input
              id="login-username"
              type="text"
              required
              autoComplete="username"
              value={username}
              onChange={(e) => {
                setUsername(e.target.value);
                if (fieldErrors.username) setFieldErrors((prev) => ({ ...prev, username: undefined }));
              }}
              aria-invalid={fieldErrors.username ? true : undefined}
              aria-describedby={fieldErrors.username ? 'login-username-error' : undefined}
              className={UI_AUTH_INPUT}
              placeholder="Username or email"
            />
            {fieldErrors.username ? (
              <p id="login-username-error" className="mt-1 text-caption text-error-foreground">
                {fieldErrors.username}
              </p>
            ) : null}
          </div>

          <div>
            <label htmlFor="login-password" className="block text-body font-medium text-text mb-1">
              Password
            </label>
            <input
              id="login-password"
              type="password"
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => {
                setPassword(e.target.value);
                if (fieldErrors.password) setFieldErrors((prev) => ({ ...prev, password: undefined }));
              }}
              aria-invalid={fieldErrors.password ? true : undefined}
              aria-describedby={fieldErrors.password ? 'login-password-error' : undefined}
              className={UI_AUTH_INPUT}
              placeholder="••••••••"
            />
            {fieldErrors.password ? (
              <p id="login-password-error" className="mt-1 text-caption text-error-foreground">
                {fieldErrors.password}
              </p>
            ) : null}
          </div>

          <Button 
            type="submit" 
            className="w-full py-3" 
            isLoading={isLoading}
          >
            Sign in
          </Button>

          <div className="text-center text-body text-muted">
            <p>Use your Fortuna username or email.</p>
          </div>
        </form>
      </div>
    </div>
  );
};
