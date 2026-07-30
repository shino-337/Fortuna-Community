import React from 'react';
import { ShellChrome } from './ShellChrome';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';

export const ViewerShell: React.FC = () => {
  const plane = useOperationalMaterialization();

  return (
    <ShellChrome
      identityLabel={plane.identityLabel}
      identityDescription={plane.identityDescription}
      navSections={plane.navigation}
      showDataControl
      showFindingSearch
    />
  );
};
