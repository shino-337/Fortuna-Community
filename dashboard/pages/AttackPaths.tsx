import React from 'react';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { ComingSoon } from '../components/ComingSoon';

export const AttackPaths: React.FC = () => (
  <PageLayout
    title="Attack Paths"
    description="Visualize paths from workloads to sensitive resources."
  >
    <ComingSoon
      title="Attack Paths"
      description="Attack path visualization from pods to high-value targets (e.g. cluster-admin, secrets) will be available in a future release."
    />
  </PageLayout>
);
