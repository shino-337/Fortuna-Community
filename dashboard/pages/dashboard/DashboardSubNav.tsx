import React from 'react';
import clsx from 'clsx';
import type { DashboardMode, DashboardSectionId } from './types';
import type { PersonaId } from '../../lib/persona';
import { isDashboardSectionVisible, type DashboardWidgetId } from '../../lib/dashboardComposition';

const SECTIONS: { id: DashboardSectionId; label: string; modes: DashboardMode[] }[] = [
  { id: 'risk-overview', label: 'Risk', modes: ['overview', 'full'] },
  { id: 'entry-points', label: 'Entry points', modes: ['overview', 'full'] },
  { id: 'cluster-health', label: 'Cluster', modes: ['overview', 'full'] },
  { id: 'exposure', label: 'Exposure', modes: ['full'] },
  { id: 'attack-analysis', label: 'Attack paths', modes: ['full'] },
  { id: 'activity', label: 'Activity', modes: ['overview', 'full'] },
];

export const DashboardSubNav: React.FC<{
  mode: DashboardMode;
  personaId: PersonaId;
  widgets: DashboardWidgetId[];
}> = ({ mode, personaId, widgets }) => {
  const visible = SECTIONS.filter(
    (s) => s.modes.includes(mode) && isDashboardSectionVisible(personaId, s.id, mode, widgets),
  );

  const scrollTo = (id: DashboardSectionId) => {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  return (
    <nav
      aria-label="Dashboard sections"
      className="sticky top-0 z-20 -mx-1 mb-2 flex gap-1 overflow-x-auto rounded-lg border border-border/80 bg-base/90 px-1 py-1 backdrop-blur-md"
    >
      {visible.map((s) => (
        <button
          key={s.id}
          type="button"
          onClick={() => scrollTo(s.id)}
          className={clsx(
            'shrink-0 rounded-md px-3 py-1.5 text-caption font-semibold transition-colors',
            'text-muted hover:bg-surface-2/80 hover:text-text',
          )}
        >
          {s.label}
        </button>
      ))}
    </nav>
  );
};
