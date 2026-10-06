import React from 'react';
import { ShellChrome } from './ShellChrome';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';

/**
 * One shell for every role. The role only changes which navigation items exist
 * (by permission) and, for User admin, hides cluster scope, search and the
 * system banner because that role never reads cluster data.
 */
export const AppShell: React.FC = () => {
  const plane = useOperationalMaterialization();
  const accountsOnly = plane.shellVariant === 'user_admin';

  return (
    <ShellChrome
      navSections={plane.navigation}
      allowedRoutes={plane.allowedRoutes}
      showScope={!accountsOnly}
      showFindingSearch={!accountsOnly}
      showBanner={!accountsOnly}
    />
  );
};
