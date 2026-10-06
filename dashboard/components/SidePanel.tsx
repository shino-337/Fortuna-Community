import React, { useEffect, useRef } from 'react';
import { X } from 'lucide-react';

interface SidePanelProps {
  open: boolean;
  onClose: () => void;
  /** Visible heading; also the dialog's accessible name. */
  title: React.ReactNode;
  /** One line under the title. */
  subtitle?: React.ReactNode;
  /** Buttons next to the close button. */
  actions?: React.ReactNode;
  children: React.ReactNode;
}

/** A right-hand panel over the page: Escape or the backdrop closes it, and focus stays inside while open. */
export const SidePanel: React.FC<SidePanelProps> = ({ open, onClose, title, subtitle, actions, children }) => {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const root = ref.current;
    const previous = document.activeElement as HTMLElement | null;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
        return;
      }
      if (e.key !== 'Tab' || !root) return;
      const focusable = Array.from(root.querySelectorAll<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])')).filter(
        (el) => !el.hasAttribute('disabled') && el.offsetParent !== null,
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement as HTMLElement | null;
      if (!active || !root.contains(active)) {
        e.preventDefault();
        first.focus();
      } else if (e.shiftKey && active === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && active === last) {
        e.preventDefault();
        first.focus();
      }
    };
    window.addEventListener('keydown', onKeyDown, true);
    const t = window.setTimeout(() => root?.querySelector<HTMLElement>('[data-autofocus], button, [href]')?.focus(), 0);
    return () => {
      window.clearTimeout(t);
      window.removeEventListener('keydown', onKeyDown, true);
      previous?.focus?.();
    };
  }, [open, onClose]);

  if (!open) return null;
  return (
    <>
      <div className="fixed inset-0 z-overlay bg-black/40" onClick={onClose} aria-hidden="true" />
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby="side-panel-title"
        className="fixed bottom-0 right-0 top-0 z-modal flex w-full max-w-lg flex-col overflow-y-auto overscroll-y-contain border-l border-border bg-surface shadow-xl"
      >
        <div className="flex items-start justify-between gap-3 border-b border-border p-4">
          <div className="min-w-0">
            <h2 id="side-panel-title" className="truncate text-section-title text-text">
              {title}
            </h2>
            {subtitle ? <div className="mt-1 text-caption text-muted">{subtitle}</div> : null}
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {actions}
            <button
              type="button"
              onClick={onClose}
              aria-label="Close"
              className="inline-flex min-h-10 min-w-10 items-center justify-center rounded text-muted hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
            >
              <X className="h-5 w-5" aria-hidden />
            </button>
          </div>
        </div>
        <div className="flex flex-1 flex-col gap-5 p-4">{children}</div>
      </div>
    </>
  );
};
