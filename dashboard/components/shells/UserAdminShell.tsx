import React from 'react';
import { ShellChrome } from './ShellChrome';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';

/** Identity administration only — no operational cognition strips. */
export const UserAdminShell: React.FC = () => {
  const plane = useOperationalMaterialization();

  return (
    <ShellChrome
      identityLabel={plane.identityLabel}
      identityDescription={plane.identityDescription}
      navSections={plane.navigation}
      showClusterSelector={false}
      showFindingSearch={false}
      showDataControl={false}
    />
  );
};
