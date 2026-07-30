import React, { useRef } from 'react';
import clsx from 'clsx';
import { UI_PILL_ACTIVE } from '../../lib/formChrome';

export interface TabItem {
  id: string;
  label: string;
  icon?: React.ReactNode;
}

export interface TabsProps {
  items: TabItem[];
  value: string;
  onChange: (id: string) => void;
  className?: string;
  ariaLabel?: string;
  getPanelId?: (id: string) => string;
  /** pill = default segmented control; underline = horizontal tabs in a toolbar */
  variant?: 'pill' | 'underline';
}

export const Tabs: React.FC<TabsProps> = ({
  items,
  value,
  onChange,
  className = '',
  ariaLabel = 'Section tabs',
  getPanelId,
  variant = 'pill',
}) => {
  const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({});

  const selectByIndex = (index: number) => {
    if (items.length === 0) return;
    const next = items[(index + items.length) % items.length];
    onChange(next.id);
    window.requestAnimationFrame(() => tabRefs.current[next.id]?.focus());
  };

  const isUnderline = variant === 'underline';

  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className={clsx(
        isUnderline
          ? 'flex gap-1 overflow-x-auto overscroll-x-contain scrollbar-thin scroll-smooth snap-x snap-mandatory'
          : 'inline-flex flex-wrap gap-1 p-1 bg-surface/80 rounded-lg border border-border mb-6',
        className,
      )}
    >
      {items.map(({ id, label, icon }, index) => (
        <button
          key={id}
          ref={(node) => { tabRefs.current[id] = node; }}
          type="button"
          role="tab"
          id={isUnderline ? `resources-tab-${id}` : `tab-${id}`}
          aria-selected={value === id}
          aria-controls={getPanelId?.(id) ?? (isUnderline ? `resources-panel-${id}` : undefined)}
          tabIndex={value === id ? 0 : -1}
          onClick={() => onChange(id)}
          onKeyDown={(event) => {
            if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
              event.preventDefault();
              selectByIndex(index + 1);
            } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
              event.preventDefault();
              selectByIndex(index - 1);
            } else if (event.key === 'Home') {
              event.preventDefault();
              selectByIndex(0);
            } else if (event.key === 'End') {
              event.preventDefault();
              selectByIndex(items.length - 1);
            }
          }}
          className={clsx(
            'flex items-center font-medium transition-colors',
            isUnderline
              ? clsx(
                  'min-w-0 shrink-0 snap-start whitespace-nowrap border-b-2 px-3 py-3 text-body sm:px-4',
                  value === id
                    ? 'border-brand bg-surface text-brand'
                    : 'border-transparent text-muted hover:bg-muted/30 hover:text-text',
                )
              : clsx(
                  'gap-2 px-4 py-2 rounded-md text-body',
                  value === id ? clsx(UI_PILL_ACTIVE, 'shadow') : 'text-muted hover:bg-surface-2 hover:text-text',
                ),
          )}
        >
          {icon ? <span className={clsx(isUnderline && 'mr-2 shrink-0')} aria-hidden>{icon}</span> : null}
          {label}
        </button>
      ))}
    </div>
  );
};

