import React from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Tabs } from '../design-system/components/Tabs';
import { useOperationalMaterialization } from '../hooks/useOperationalMaterialization';

export interface SectionNavItem {
  path: string;
  label: string;
}

/** Sections of one workspace that live on their own routes (e.g. Platform, Rules). */
export const PLATFORM_HEALTH_SECTIONS: SectionNavItem[] = [
  { path: '/monitoring', label: 'Pipeline & runtime' },
  { path: '/monitoring/certificates', label: 'Certificates' },
  { path: '/monitoring/notifications', label: 'Notifications' },
];

export const RULES_CATALOG_SECTIONS: SectionNavItem[] = [
  { path: '/rules', label: 'Detection & policies' },
  { path: '/rules/risk-scoring', label: 'Risk scoring rules' },
  { path: '/rules/catalog', label: 'Capability catalog' },
];

export const INVENTORY_SECTIONS: SectionNavItem[] = [
  { path: '/resources/clusters', label: 'Clusters' },
  { path: '/resources', label: 'Workloads & RBAC' },
];

/** Tab strip linking the sections of a workspace; sections the user cannot open are hidden. */
export const SectionNav: React.FC<{ sections: SectionNavItem[]; ariaLabel: string }> = ({ sections, ariaLabel }) => {
  const navigate = useNavigate();
  const location = useLocation();
  const { allowedRoutes } = useOperationalMaterialization();
  const visible = sections.filter((s) => allowedRoutes.includes(s.path));
  if (visible.length < 2) return null;
  const current = location.pathname.replace(/\/$/, '') || '/';
  return (
    <Tabs
      variant="underline"
      ariaLabel={ariaLabel}
      className="mb-4 border-b border-border"
      items={visible.map((s) => ({ id: s.path, label: s.label }))}
      value={visible.find((s) => s.path === current)?.path ?? visible[0].path}
      onChange={(path) => navigate(path)}
    />
  );
};
