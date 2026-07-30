import React, { useEffect, useId, useRef } from 'react';
import clsx from 'clsx';
import { X } from 'lucide-react';
import { Button } from '../../components/ui/Button';

type DialogSize = 'sm' | 'md' | 'lg' | 'xl';

interface DialogProps {
  open: boolean;
  title: string;
  description?: React.ReactNode;
  children: React.ReactNode;
  footer?: React.ReactNode;
  onClose: () => void;
  closeDisabled?: boolean;
  size?: DialogSize;
  className?: string;
  bodyClassName?: string;
}

const sizeClass: Record<DialogSize, string> = {
  sm: 'max-w-md',
  md: 'max-w-lg',
  lg: 'max-w-2xl',
  xl: 'max-w-4xl',
};

export const Dialog: React.FC<DialogProps> = ({
  open,
  title,
  description,
  children,
  footer,
  onClose,
  closeDisabled = false,
  size = 'md',
  className,
  bodyClassName,
}) => {
  const titleId = useId();
  const descriptionId = useId();
  const dialogRef = useRef<HTMLElement | null>(null);
  const closeRef = useRef<HTMLButtonElement | null>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!open) return;
    previousFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const t = window.setTimeout(() => closeRef.current?.focus(), 0);
    return () => window.clearTimeout(t);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !closeDisabled) {
        onClose();
        return;
      }
      if (event.key !== 'Tab') return;
      const focusable = Array.from(
        dialogRef.current?.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
        ) ?? [],
      ).filter((el) => !el.hasAttribute('disabled') && el.getAttribute('aria-hidden') !== 'true');
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [closeDisabled, onClose, open]);

  useEffect(() => {
    if (open) return;
    window.setTimeout(() => previousFocusRef.current?.focus(), 0);
  }, [open]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-modal flex items-center justify-center bg-black/65 px-4 py-6"
      role="presentation"
      onMouseDown={(event) => {
        if (!closeDisabled && event.target === event.currentTarget) onClose();
      }}
    >
      <section
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
        className={clsx(
          'flex max-h-[90dvh] w-full min-w-0 flex-col overflow-hidden rounded-xl border border-border bg-surface shadow-2xl',
          sizeClass[size],
          className,
        )}
      >
        <div className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
          <div className="min-w-0">
            <h2 id={titleId} className="text-section-title text-text">
              {title}
            </h2>
            {description ? (
              <div id={descriptionId} className="mt-1.5 max-w-[68ch] text-caption text-muted">
                {description}
              </div>
            ) : null}
          </div>
          <Button
            ref={closeRef}
            type="button"
            variant="ghost"
            size="touch"
            className="shrink-0"
            aria-label={`Close ${title}`}
            disabled={closeDisabled}
            onClick={onClose}
          >
            <X className="h-4 w-4" aria-hidden />
          </Button>
        </div>
        <div className={clsx('min-h-0 flex-1 overflow-y-auto overscroll-y-contain p-5', bodyClassName)}>
          {children}
        </div>
        {footer ? (
          <div className="flex flex-col-reverse gap-2 border-t border-border bg-base/40 px-5 py-4 sm:flex-row sm:justify-end">
            {footer}
          </div>
        ) : null}
      </section>
    </div>
  );
};
