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
import { ErrorLogs } from './pages/ErrorLogs';
import { Reports } from './pages/Reports';
import { Notifications } from './pages/Notifications';
import { Capabilities } from './pages/Capabilities';
import { Clusters } from './pages/Clusters';
import { ClusterDetail } from './pages/ClusterDetail';
import { RiskDetail } from './pages/RiskDetail';
import { PodDetail } from './pages/PodDetail';
import { NodeDetail } from './pages/NodeDetail';
import { IdentityDetail } from './pages/IdentityDetail';
import { RuleDetail } from './pages/RuleDetail';
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
          <Route path="clusters" element={<Clusters />} />
          <Route path="clusters/:id" element={<ClusterDetail />} />
          <Route path="clusters/:clusterId/nodes/:nodeName" element={<NodeDetail />} />
          <Route path="resources" element={<Resources />} />
          <Route path="resources/pods/uid/:uid" element={<PodDetail />} />
          <Route path="resources/pods/:id" element={<PodDetail />} />
          <Route path="risks" element={<RiskCenter />} />
          <Route path="risks/findings" element={<RiskCenter />} />
          <Route path="risks/pce" element={<RiskCenter />} />
          <Route path="risks/evidence" element={<RiskCenter />} />
          <Route path="risks/:id" element={<RiskDetail />} />
          <Route path="capabilities" element={<Capabilities />} />
          <Route path="identities" element={<Navigate to="/resources?tab=ServiceAccount" replace />} />
          <Route path="identities/uid/:uid" element={<IdentityDetail />} />
          <Route path="identities/:id" element={<IdentityDetail />} />
          <Route path="rules" element={<Rules />} />
          <Route path="rules/:id" element={<RuleDetail />} />
          <Route path="attack-paths" element={<AttackPaths />} />
          <Route path="monitoring" element={<Monitoring />} />
          <Route path="settings" element={<Settings />} />
          {/* Secondary: reachable from Monitoring or direct URL */}
          <Route path="certificates" element={<Certificates />} />
          <Route path="audit" element={<Audit />} />
          <Route path="error-logs" element={<ErrorLogs />} />
          <Route path="reports" element={<Reports />} />
          <Route path="notifications" element={<Notifications />} />
          <Route path="sbom" element={<Navigate to="/resources" replace />} />
          <Route path="insights" element={<Navigate to="/risks" replace />} />
          <Route path="metrics" element={<Navigate to="/monitoring" replace />} />
        </Route>
      </Routes>
    </HashRouter>
  );
};

export default App;
