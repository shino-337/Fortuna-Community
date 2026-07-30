import React, { useEffect, useState } from 'react';
import { Bookmark, BookmarkPlus, Trash2 } from 'lucide-react';
import { Button } from './ui/Button';
import type { PersonaId } from '../lib/persona';
import {
  loadRiskSavedViews,
  newSavedViewId,
  persistRiskSavedViews,
  type RiskFindingsSavedView,
} from '../lib/savedViews';

export interface RiskFindingsFilterSnapshot {
  statusFilter: 'all' | 'active' | 'resolved' | 'acknowledged';
  riskLevelFilter: string;
  searchTerm: string;
  clusterId?: string;
  sinceMinutes?: number;
}

/** Saved filter presets for risk findings — browser-local per persona. */
export const RiskFindingsSavedViews: React.FC<{
  personaId: PersonaId;
  current: RiskFindingsFilterSnapshot;
  onApply: (filters: RiskFindingsSavedView['filters']) => void;
}> = ({ personaId, current, onApply }) => {
  const [views, setViews] = useState<RiskFindingsSavedView[]>(() => loadRiskSavedViews(personaId));
  const [name, setName] = useState('');

  useEffect(() => {
    setViews(loadRiskSavedViews(personaId));
  }, [personaId]);

  useEffect(() => {
    persistRiskSavedViews(personaId, views);
  }, [personaId, views]);

  const saveCurrent = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    const next: RiskFindingsSavedView = {
      id: newSavedViewId(),
      name: trimmed,
      createdAt: new Date().toISOString(),
      personaId,
      filters: { ...current },
    };
    setViews((prev) => [next, ...prev].slice(0, 12));
    setName('');
  };

  return (
    <div className="rounded-lg border border-border bg-surface/40 p-3 space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <Bookmark className="w-4 h-4 text-brand shrink-0" aria-hidden />
        <span className="text-caption font-semibold text-text">Saved views</span>
        <span className="text-meta text-muted">({personaId} presets · browser-local)</span>
      </div>
      <div className="flex flex-wrap gap-2 items-center">
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Name this filter set…"
          className="min-w-[12rem] flex-1 rounded-md border border-border bg-base px-2 py-1.5 text-caption text-text"
          onKeyDown={(e) => {
            if (e.key === 'Enter') saveCurrent();
          }}
        />
        <Button type="button" variant="secondary" size="sm" onClick={saveCurrent} disabled={!name.trim()}>
          <BookmarkPlus className="w-4 h-4 mr-1" />
          Save
        </Button>
      </div>
      {views.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {views.map((v) => (
            <div
              key={v.id}
              className="inline-flex items-center gap-1 rounded-md border border-border bg-base/60 pl-2 pr-1 py-0.5"
            >
              <button
                type="button"
                className="text-caption text-brand hover:underline"
                onClick={() => onApply(v.filters)}
                title={`Saved ${new Date(v.createdAt).toLocaleString()}`}
              >
                {v.name}
              </button>
              <button
                type="button"
                className="p-1 text-muted hover:text-rose-300"
                aria-label={`Delete saved view ${v.name}`}
                onClick={() => setViews((prev) => prev.filter((x) => x.id !== v.id))}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}
        </div>
      ) : (
        <p className="text-meta text-muted">Save frequent triage filters (status, risk level, search, scope).</p>
      )}
    </div>
  );
};
