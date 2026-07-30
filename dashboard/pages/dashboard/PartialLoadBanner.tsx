import React, { useState } from 'react';
import { AlertTriangle, X } from 'lucide-react';

export const PartialLoadBanner: React.FC<{
  messages: string[];
  title?: string;
  actions?: React.ReactNode;
}> = ({ messages, title = 'Some dashboard sections did not load', actions }) => {
  const [dismissed, setDismissed] = useState(false);
  if (dismissed || messages.length === 0) return null;

  return (
    <div
      role="status"
      className="flex items-start gap-3 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2.5 text-caption text-amber-100"
    >
      <AlertTriangle className="h-4 w-4 shrink-0 text-amber-400 mt-0.5" aria-hidden />
      <div className="min-w-0 flex-1">
        <p className="font-medium text-text">{title}</p>
        <ul className="mt-1 list-disc pl-4 text-muted leading-relaxed">
          {messages.map((m) => (
            <li key={m}>{m}</li>
          ))}
        </ul>
        {actions && <div className="mt-2 flex flex-wrap gap-2">{actions}</div>}
      </div>
      <button
        type="button"
        className="shrink-0 rounded p-1 text-muted hover:text-text"
        aria-label="Dismiss partial load notice"
        onClick={() => setDismissed(true)}
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
};
