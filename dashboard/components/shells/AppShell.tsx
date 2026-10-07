import React from 'react';
import { ShellChrome } from './ShellChrome';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';

/**
 * One shell for every role. The role only changes which navigation items exist
 * (by permission).
 */
export const AppShell: React.FC = () => {
  const plane = useOperationalMaterialization();

  return (
    <ShellChrome
      navSections={plane.navigation}
      allowedRoutes={plane.allowedRoutes}
      showScope
      showFindingSearch
      showBanner
    />
  );
};
