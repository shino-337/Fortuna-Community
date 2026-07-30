import React from 'react';
import { ViewerShell } from './ViewerShell';
import { OperatorShell } from './OperatorShell';
import { AdminShell } from './AdminShell';
import { UserAdminShell } from './UserAdminShell';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';

export const AppShell: React.FC = () => {
  const { shellVariant } = useOperationalMaterialization();

  switch (shellVariant) {
    case 'user_admin':
      return <UserAdminShell />;
    case 'admin':
      return <AdminShell />;
    case 'operator':
      return <OperatorShell />;
    case 'viewer':
    default:
      return <ViewerShell />;
  }
};
