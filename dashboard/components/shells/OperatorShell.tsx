import React from 'react';
import { ShellChrome } from './ShellChrome';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';
import { IncidentModeBanner } from '../IncidentModeBanner';
import { EscalationStatusBar } from '../EscalationStatusBar';
import { RuntimeThreatStrip } from '../RuntimeThreatStrip';
import { MultiIncidentStrip } from '../MultiIncidentStrip';
import { OperationalFatigueStrip } from '../OperationalFatigueStrip';

export const OperatorShell: React.FC = () => {
  const plane = useOperationalMaterialization();

  return (
    <ShellChrome
      identityLabel={plane.identityLabel}
      identityDescription={plane.identityDescription}
      navSections={plane.navigation}
      showDataControl
      showFindingSearch
      globalStrips={
        <>
          <IncidentModeBanner />
          <RuntimeThreatStrip />
          <EscalationStatusBar />
          <MultiIncidentStrip />
          <OperationalFatigueStrip />
        </>
      }
    />
  );
};
