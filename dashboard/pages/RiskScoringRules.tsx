import React from 'react';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PAGE_TITLES } from '../lib/pageTitles';
import { RULES_CATALOG_SECTIONS, SectionNav } from '../components/SectionNav';
import { RiskRulesPanel } from '../components/rules/RiskRulesPanel';

export const RiskScoringRules: React.FC = () => (
  <PageLayout
    title={PAGE_TITLES.policyRules}
    description="Risk scoring rules evaluated by the risk engine. Rules stored in the database can be edited, imported and exported as YAML."
  >
    <SectionNav sections={RULES_CATALOG_SECTIONS} ariaLabel="Rules and catalog sections" />
    <RiskRulesPanel />
  </PageLayout>
);
