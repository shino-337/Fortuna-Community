import React from 'react';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PAGE_TITLES, RULES_PAGE_DESCRIPTION } from '../lib/pageTitles';
import { RULES_CATALOG_SECTIONS, SectionNav } from '../components/SectionNav';
import { RiskRulesPanel } from '../components/rules/RiskRulesPanel';

export const RiskScoringRules: React.FC = () => (
  <PageLayout
    title={PAGE_TITLES.policyRules}
    description={RULES_PAGE_DESCRIPTION}
  >
    <SectionNav sections={RULES_CATALOG_SECTIONS} ariaLabel="Rules and catalog sections" />
    <RiskRulesPanel />
  </PageLayout>
);
