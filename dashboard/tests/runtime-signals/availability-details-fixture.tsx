import React from 'react';
import { createRoot } from 'react-dom/client';
import { MemoryRouter, Route, Routes, useNavigate } from 'react-router-dom';
import { ClusterDetail } from '../../pages/ClusterDetail';
import { CapabilityDetail } from '../../pages/CapabilityDetail';
import { NodeDetail } from '../../pages/NodeDetail';
import { PodDetail } from '../../pages/PodDetail';

const path = new URLSearchParams(window.location.search).get('path') || '/clusters/cluster-a';

const NavigationBridge: React.FC = () => {
  const navigate = useNavigate();
  React.useEffect(() => {
    const target = window as typeof window & { __availabilityNavigate?: (to: string) => void };
    target.__availabilityNavigate = (to: string) => navigate(to);
    return () => {
      delete target.__availabilityNavigate;
    };
  }, [navigate]);
  return null;
};

createRoot(document.getElementById('root')!).render(
  <MemoryRouter initialEntries={[path]}>
    <NavigationBridge />
    <Routes>
      <Route path="/clusters/:id" element={<ClusterDetail />} />
      <Route path="/clusters/:clusterId/nodes/:nodeName" element={<NodeDetail />} />
      <Route path="/capabilities/:id" element={<CapabilityDetail />} />
      <Route path="/resources/pods/uid/:uid" element={<PodDetail />} />
    </Routes>
  </MemoryRouter>,
);
