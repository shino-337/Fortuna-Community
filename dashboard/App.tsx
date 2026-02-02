import React, { useEffect } from 'react';
import { HashRouter, Routes, Route, Navigate } from 'react-router-dom';
import { Layout } from './components/Layout';
import { Login } from './pages/Login';
import { Dashboard } from './pages/Dashboard';
import { RiskCenter } from './pages/Insights';
import { Resources } from './pages/Resources';
import { AttackPaths } from './pages/AttackPaths';
import { Monitoring } from './pages/Metrics';
import { Rules } from './pages/Rules';
import { Settings } from './pages/Settings';
import { Certificates } from './pages/Certificates';
import { Audit } from './pages/Audit';
import { Reports } from './pages/Reports';
import { Notifications } from './pages/Notifications';
import { Sbom } from './pages/Sbom';
import { Capabilities } from './pages/Capabilities';
import { Clusters } from './pages/Clusters';
import { useAuthStore } from './store/authStore';

const ProtectedRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const hasHydrated = useAuthStore((state) => state._hasHydrated);
  if (!hasHydrated) {
    return (
      <div className="min-h-screen bg-slate-950 flex items-center justify-center">
        <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
};

const App: React.FC = () => {
  useEffect(() => {
    const t = setTimeout(() => useAuthStore.getState().setHasHydrated(true), 500);
    return () => clearTimeout(t);
  }, []);
  return (
    <HashRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        
        <Route path="/" element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }>
          <Route index element={<Dashboard />} />
          <Route path="risks" element={<RiskCenter />} />
          <Route path="sbom" element={<Sbom />} />
          <Route path="resources" element={<Resources />} />
          <Route path="attack-paths" element={<AttackPaths />} />
          <Route path="rules" element={<Rules />} />
          <Route path="monitoring" element={<Monitoring />} />
          <Route path="certificates" element={<Certificates />} />
          <Route path="audit" element={<Audit />} />
          <Route path="reports" element={<Reports />} />
          <Route path="notifications" element={<Notifications />} />
          <Route path="settings" element={<Settings />} />
          <Route path="capabilities" element={<Capabilities />} />
          <Route path="clusters" element={<Clusters />} />
          
          <Route path="insights" element={<Navigate to="/risks" replace />} />
          <Route path="metrics" element={<Navigate to="/monitoring" replace />} />
        </Route>
      </Routes>
    </HashRouter>
  );
};

export default App;
