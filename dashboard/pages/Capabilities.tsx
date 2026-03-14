import React from 'react';
import { CapabilityMetadataBrowser } from '../components/CapabilityMetadataBrowser';
import { Card } from '../components/ui/Card';
import { Shield } from 'lucide-react';
import { PageLayout } from '../design-system/layouts/PageLayout';

export const Capabilities: React.FC = () => {
  return (
    <PageLayout
      title="Capability Catalog"
      description="Definitions, severity, preconditions, MITRE mapping, and attack steps."
    >
      <Card className="p-0 overflow-hidden">
        <div className="p-6 max-h-[60vh] overflow-y-auto">
          <CapabilityMetadataBrowser />
        </div>
      </Card>
    </PageLayout>
  );
};
