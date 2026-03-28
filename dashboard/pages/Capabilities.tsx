import React from 'react';
import { CapabilityMetadataBrowser } from '../components/CapabilityMetadataBrowser';
import { Card } from '../components/ui/Card';
import { Link } from 'react-router-dom';
import { BookOpen, Shield, Wrench } from 'lucide-react';
import { PageLayout } from '../design-system/layouts/PageLayout';

export const Capabilities: React.FC = () => {
  return (
    <PageLayout
      title="Capability Knowledge"
      description="Knowledge base for capability meaning and security semantics. Use this page to understand what a capability means, not to triage incidents."
    >
      <Card className="p-4 mb-4 border-slate-800 bg-slate-900/60">
        <div className="grid gap-3 md:grid-cols-3">
          <div className="rounded-lg border border-slate-800 bg-slate-950/60 p-3">
            <div className="ui-micro-label mb-1 flex items-center gap-1">
              <BookOpen className="w-3.5 h-3.5" /> Page goal
            </div>
            <p className="text-sm text-slate-300">Explain capability definition, ATT&CK mapping, impact, and mitigation guidance.</p>
          </div>
          <div className="rounded-lg border border-slate-800 bg-slate-950/60 p-3">
            <div className="ui-micro-label mb-1 flex items-center gap-1">
              <Shield className="w-3.5 h-3.5" /> Use this page when
            </div>
            <p className="text-sm text-slate-300">You need to interpret an alert or train SOC team on capability semantics.</p>
          </div>
          <div className="rounded-lg border border-slate-800 bg-slate-950/60 p-3">
            <div className="ui-micro-label mb-1 flex items-center gap-1">
              <Wrench className="w-3.5 h-3.5" /> Next action
            </div>
            <p className="text-sm text-slate-300">
              For incident triage go to <Link className="text-pink-400 hover:underline" to="/risks/findings">Risk Findings</Link>. For rule tuning go to{' '}
              <Link className="text-pink-400 hover:underline" to="/rules">Detection & Policy Catalog</Link>.
            </p>
          </div>
        </div>
      </Card>
      <Card className="p-0 overflow-hidden">
        <div className="p-6 ui-table-scroll">
          <CapabilityMetadataBrowser />
        </div>
      </Card>
    </PageLayout>
  );
};
