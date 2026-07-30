import React from 'react';
import { Link } from 'react-router-dom';
import { Activity, AlertTriangle } from 'lucide-react';

/** Admin monitoring workspace emphasis (Phase 2). */
export const MonitoringPersonaStrip: React.FC = () => (
  <div className="mb-4 rounded-lg border border-sky-500/25 bg-sky-500/10 px-4 py-3 flex flex-wrap gap-3 items-center justify-between">
    <div className="flex items-start gap-2">
      <Activity className="w-5 h-5 text-sky-300 shrink-0 mt-0.5" />
      <div>
        <p className="text-caption font-semibold text-text">Platform integrity</p>
        <p className="text-meta text-muted mt-0.5">
          Identify telemetry degradation within 60 seconds — agents, workers, queues, and audit anchors.
        </p>
      </div>
    </div>
    <Link
      to="/governance"
      className="text-caption text-brand hover:underline inline-flex items-center gap-1 shrink-0"
    >
      <AlertTriangle className="w-3.5 h-3.5" />
      Governance signals
    </Link>
  </div>
);
