import React, { createContext, useCallback, useContext, useMemo, useState } from 'react';
import { AlertCircle, CheckCircle, X } from 'lucide-react';

type ToastVariant = 'error' | 'success' | 'info';

type ToastOptions = {
  title: string;
  description?: string;
  variant?: ToastVariant;
};

type ToastItem = ToastOptions & { id: number; variant: ToastVariant };

const ToastContext = createContext<((options: ToastOptions) => void) | null>(null);

const variantClass: Record<ToastVariant, string> = {
  error: 'border-critical/40 bg-critical/10 text-text',
  success: 'border-success/35 bg-success/10 text-text',
  info: 'border-border bg-surface text-text',
};

export const ToastProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [items, setItems] = useState<ToastItem[]>([]);

  const dismiss = useCallback((id: number) => {
    setItems((current) => current.filter((item) => item.id !== id));
  }, []);

  const notify = useCallback((options: ToastOptions) => {
    const id = Date.now() + Math.floor(Math.random() * 1000);
    const item: ToastItem = { ...options, id, variant: options.variant ?? 'info' };
    setItems((current) => [item, ...current].slice(0, 4));
    window.setTimeout(() => dismiss(id), 6000);
  }, [dismiss]);

  const visible = useMemo(() => items, [items]);

  return (
    <ToastContext.Provider value={notify}>
      {children}
      <div className="fixed right-4 top-4 z-toast flex w-[min(100vw-2rem,24rem)] flex-col gap-2" aria-live="polite" aria-relevant="additions text">
        {visible.map((item) => (
          <section
            key={item.id}
            role={item.variant === 'error' ? 'alert' : 'status'}
            className={`rounded-lg border px-3 py-3 shadow-xl ${variantClass[item.variant]}`}
          >
            <div className="flex items-start gap-2">
              {item.variant === 'success' ? (
                <CheckCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
              ) : (
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
              )}
              <div className="min-w-0 flex-1">
                <p className="text-body font-semibold text-text">{item.title}</p>
                {item.description ? <p className="mt-1 text-caption text-muted">{item.description}</p> : null}
              </div>
              <button
                type="button"
                className="rounded p-1 text-muted hover:bg-surface-2 hover:text-text focus:outline-none focus:ring-2 focus:ring-brand/50"
                onClick={() => dismiss(item.id)}
                aria-label="Dismiss notification"
              >
                <X className="h-4 w-4" aria-hidden />
              </button>
            </div>
          </section>
        ))}
      </div>
    </ToastContext.Provider>
  );
};

export function useToast() {
  const notify = useContext(ToastContext);
  if (!notify) {
    throw new Error('useToast must be used within ToastProvider');
  }
  return notify;
}
