import React from 'react';
import { createRoot } from 'react-dom/client';
import { RuntimeSignalsTable } from '../../components/RuntimeSignalsTable';

// Isolated test entry; not included in the production build.
createRoot(document.getElementById('root')!).render(<RuntimeSignalsTable />);
