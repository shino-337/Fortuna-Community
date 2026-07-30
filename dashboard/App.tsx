import React, { Suspense, useEffect, useRef } from 'react';
import { HashRouter, Routes, Route, Navigate } from 'react-router-dom';
import { Login } from './pages/Login';
import { ForcePasswordChange } from './pages/ForcePasswordChange';
import { PageLoading } from './design-system/components/PageStatus';
import { useAuthStore } from './store/authStore';
import { api } from './lib/api';
import { AppShell } from './components/shells/AppShell';
import { MaterializedRoutes } from './components/MaterializedRoutes';
import { useOperationalMaterialization } from './hooks/useOperationalMaterialization';
import { ErrorBoundary } from './components/ErrorBoundary';

const PersonaHome = React.lazy(() => import('./pages/PersonaHome').then((m) => ({ default: m.PersonaHome })));

const ProtectedRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const hasHydrated = useAuthStore((state) => state._hasHydrated);
  const user = useAuthStore((state) => state.user);
  const token = useAuthStore((state) => state.token);
  const login = useAuthStore((state) => state.login);
  const refreshedScopeKeyRef = useRef<string | null>(null);

  useEffect(() => {
    if (!isAuthenticated || !token || !user) return;
    if (user.operationalScope) return;
    const refreshKey = `${token}:${String(user.id ?? user.username ?? user.email ?? '')}`;
    if (refreshedScopeKeyRef.current === refreshKey) return;
    refreshedScopeKeyRef.current = refreshKey;
    void api.getCurrentUser().then(({ user: fresh }) => login(fresh, token)).catch(() => undefined);
  }, [isAuthenticated, token, user, login]);

  if (!hasHydrated) {
    return <PageLoading message="Restoring session..." className="min-h-dvh min-w-0 bg-base" />;
  }
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  if (user?.mustChangePassword) {
    return <ForcePasswordChange />;
  }
  return <>{children}</>;
};

const MaterializedHome: React.FC = () => {
  const { allowedRoutes, defaultRoute } = useOperationalMaterialization();
  if (allowedRoutes.includes('/')) {
    return <PersonaHome />;
  }
  return <Navigate to={defaultRoute} replace />;
};

const App: React.FC = () => {
  useEffect(() => {
    const t = setTimeout(() => useAuthStore.getState().setHasHydrated(true), 500);
    return () => clearTimeout(t);
  }, []);

  return (
    <HashRouter>
      <ErrorBoundary>
        <Suspense fallback={<PageLoading message="Loading operational workspace…" className="min-h-[50dvh]" />}>
          <Routes>
            <Route path="/login" element={<Login />} />

            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <AppShell />
                </ProtectedRoute>
              }
            >
              <Route index element={<MaterializedHome />} />
              <Route path="*" element={<MaterializedRoutes />} />
            </Route>
          </Routes>
        </Suspense>
      </ErrorBoundary>
    </HashRouter>
  );
};

export default App;
