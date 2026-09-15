import React from 'react';
import { createRoot } from 'react-dom/client';
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom';
import { IdentityDetail } from '../../pages/IdentityDetail';

createRoot(document.getElementById('root')!).render(
  <MemoryRouter initialEntries={['/identities/uid/sa-a']}>
    <Link to="/identities/uid/sa-b">Switch identity</Link>
    <Routes><Route path="/identities/uid/:uid" element={<IdentityDetail />} /></Routes>
  </MemoryRouter>,
);
