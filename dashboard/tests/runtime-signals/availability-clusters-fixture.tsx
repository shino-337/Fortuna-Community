import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { MemoryRouter } from 'react-router-dom';
import { ClustersView } from '../../pages/inventory/ClustersView';

/** The Clusters view of Inventory with the page's Refresh button. */
function Fixture() {
  const [refreshKey, setRefreshKey] = useState(0);
  return (
    <>
      <button type="button" aria-label="Refresh" onClick={() => setRefreshKey((k) => k + 1)}>
        ↻
      </button>
      <ClustersView selectedClusterId={null} onOpenCluster={() => undefined} refreshKey={refreshKey} canFindings />
    </>
  );
}

createRoot(document.getElementById('root')!).render(
  <MemoryRouter initialEntries={['/resources?view=clusters']}>
    <Fixture />
  </MemoryRouter>,
);
