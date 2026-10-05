import { createRoot } from 'react-dom/client';
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom';
import { IdentityDetail } from '../../pages/IdentityDetail';
import { useAuthStore } from '../../store/authStore';
import type { User } from '../../types';

const readOnly = new URLSearchParams(window.location.search).get('readOnly') === '1';
const operationId = new URLSearchParams(window.location.search).get('operationId');
useAuthStore.getState().login({
  id: '1',
  username: 'operator',
  role: 'operator',
  permissions: readOnly ? ['inventory.read'] : ['inventory.read', 'inventory.modify', 'inventory.delete'],
} as User, 'fixture-token');

createRoot(document.getElementById('root')!).render(
  <MemoryRouter initialEntries={[`/identities/uid/sa-a?clusterId=cluster-a${operationId ? `&operationId=${encodeURIComponent(operationId)}` : ''}`]}>
    <Link to="/identities/uid/sa-b?clusterId=cluster-b">Switch identity</Link>
    <Routes><Route path="/identities/uid/:uid" element={<IdentityDetail />} /></Routes>
  </MemoryRouter>,
);
