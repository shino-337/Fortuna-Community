import React from 'react';
import { ViewerDashboard } from './dashboards/ViewerDashboard';
import { OperatorDashboard } from './dashboards/OperatorDashboard';
import { AdminDashboard } from './dashboards/AdminDashboard';
import { useOperationalMaterialization } from '../hooks/useOperationalMaterialization';

export const PersonaHome: React.FC = () => {
  const { shellVariant } = useOperationalMaterialization();

  switch (shellVariant) {
    case 'admin':
      return <AdminDashboard />;
    case 'operator':
      return <OperatorDashboard />;
    case 'viewer':
    default:
      return <ViewerDashboard />;
  }
};
