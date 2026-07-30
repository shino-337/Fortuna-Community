import React from 'react';
import { ShellChrome } from './ShellChrome';
import { useOperationalMaterialization } from '../../hooks/useOperationalMaterialization';
import { IncidentModeBanner } from '../IncidentModeBanner';
import { EscalationStatusBar } from '../EscalationStatusBar';
import { MultiIncidentStrip } from '../MultiIncidentStrip';
import { OperationalFatigueStrip } from '../OperationalFatigueStrip';
import { TelemetryHealthStrip } from '../TelemetryHealthStrip';

export const AdminShell: React.FC = () => {
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
          <EscalationStatusBar />
          <MultiIncidentStrip />
          <OperationalFatigueStrip />
          <TelemetryHealthStrip />
        </>
      }
    />
  );
};
