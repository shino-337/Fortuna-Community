import { createRoot } from 'react-dom/client';
import { MemoryRouter } from 'react-router-dom';
import { Clusters } from '../../pages/Clusters';

createRoot(document.getElementById('root')!).render(
  <MemoryRouter initialEntries={['/clusters']}>
    <Clusters />
  </MemoryRouter>,
);
