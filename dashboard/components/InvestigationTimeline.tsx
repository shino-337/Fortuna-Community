import React, { useEffect, useState } from 'react';
import { Clock } from 'lucide-react';
import { api, type InvestigationTimelineEntryApi } from '../lib/api';
import { formatDateTime } from '../lib/display';

const EVENT_LABELS: Record<string, string> = {
  'case.created': 'Case created',
  'case.archived': 'Case archived',
  'case.status_changed': 'Status changed',
  'case.assignment_changed': 'Assignment changed',
  'case.collaboration_updated': 'Collaboration updated',
  'entity.pinned': 'Evidence pinned',
  'entity.unpinned': 'Evidence unpinned',
  'remediation.added': 'Remediation added',
  'remediation.updated': 'Remediation updated',
  'remediation.removed': 'Remediation removed',
  'note.added': 'Note added',
  'handoff.note': 'Handoff note',
  'case.pivot': 'Pivot',
};

function eventLabel(type: string): string {
  return EVENT_LABELS[type] ?? type.replace(/\./g, ' · ');
}

export const InvestigationTimeline: React.FC<{ caseId: string | null }> = ({ caseId }) => {
  const [items, setItems] = useState<InvestigationTimelineEntryApi[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!caseId) {
      setItems([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    api
      .listInvestigationTimeline(caseId)
      .then((res) => {
        if (!cancelled) setItems(res.items ?? []);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e instanceof Error ? e.message : e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [caseId]);

  if (!caseId) return null;

  return (
    <section className="space-y-2" aria-label="Investigation timeline">
      <h3 className="text-caption font-semibold text-muted uppercase tracking-wider flex items-center gap-1.5">
        <Clock className="w-4 h-4" aria-hidden />
        Activity timeline
      </h3>
      {loading ? <p className="text-caption text-muted">Loading timeline…</p> : null}
      {error ? <p className="text-caption text-amber-200">{error}</p> : null}
      {!loading && !error && items.length === 0 ? (
        <p className="text-caption text-muted">No activity recorded yet.</p>
      ) : null}
      <ol className="relative border-l border-border ml-2 space-y-3 pl-4">
        {[...items].reverse().map((ev) => (
          <li key={ev.eventId || String(ev.id)} className="text-caption">
            <div className="absolute -left-[5px] mt-1.5 h-2 w-2 rounded-full bg-brand" aria-hidden />
            <div className="font-medium text-text">{eventLabel(ev.eventType)}</div>
            <div className="text-meta text-muted mt-0.5">
              {ev.summary}
              {ev.actorUsername ? ` · ${ev.actorUsername}` : ''}
              {ev.createdAt ? ` · ${formatDateTime(ev.createdAt)}` : ''}
            </div>
            {ev.after?.status ? (
              <div className="text-meta text-muted mt-0.5">→ {String(ev.after.status)}</div>
            ) : null}
          </li>
        ))}
      </ol>
    </section>
  );
};
