import React from 'react';
import { CapabilityMetadataBrowser } from '../components/CapabilityMetadataBrowser';
import { Card } from '../components/ui/Card';
import { Shield } from 'lucide-react';

export const Capabilities: React.FC = () => {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <Shield className="w-7 h-7 text-pink-500" />
          Capability Catalog
        </h1>
        <p className="text-slate-400 mt-1">
          Definitions, severity, preconditions, MITRE mapping, and attack steps.
        </p>
      </div>
      <Card className="p-6">
        <CapabilityMetadataBrowser />
      </Card>
    </div>
  );
};
