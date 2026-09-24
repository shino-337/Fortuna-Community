import React from 'react';
import { createRoot } from 'react-dom/client';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ClusterDetail } from '../../pages/ClusterDetail';
import { CapabilityDetail } from '../../pages/CapabilityDetail';
import { NodeDetail } from '../../pages/NodeDetail';
import { PodDetail } from '../../pages/PodDetail';

const path = new URLSearchParams(window.location.search).get('path') || '/clusters/cluster-a';

createRoot(document.getElementById('root')!).render(
  <MemoryRouter initialEntries={[path]}>
    <Routes>
      <Route path="/clusters/:id" element={<ClusterDetail />} />
      <Route path="/clusters/:clusterId/nodes/:nodeName" element={<NodeDetail />} />
      <Route path="/capabilities/:id" element={<CapabilityDetail />} />
      <Route path="/resources/pods/uid/:uid" element={<PodDetail />} />
    </Routes>
  </MemoryRouter>,
);
