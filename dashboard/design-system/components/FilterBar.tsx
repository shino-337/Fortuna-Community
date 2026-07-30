import React from 'react';
import clsx from 'clsx';
import { Search } from 'lucide-react';

export interface FilterBarToggle {
  id: string;
  label: string;
  icon?: React.ReactNode;
  active: boolean;
  onClick: () => void;
  /** Active-state emphasis (default neutral brand tint) */
  activeTone?: 'brand' | 'success' | 'danger';
}

export interface FilterBarProps {
  /** When true, omit outer chrome (use inside PageLayout `toolbar` — wrapper already styled). */
  embedded?: boolean;
  className?: string;
  leading?: React.ReactNode;
  search?: {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    inputClassName?: string;
    onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  };
  namespace?: {
    value: string;
    onChange: (value: string) => void;
    options: string[];
    disabled?: boolean;
    title?: string;
    emptyLabel?: string;
    /** Free-text + datalist (e.g. Network activity) instead of a plain select */
    inputMode?: boolean;
    datalistId?: string;
    placeholder?: string;
    onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
  };
  sort?: {
    value: string;
    onChange: (value: string) => void;
    options: { value: string; label: string }[];
    selectClassName?: string;
  };
  toggles?: FilterBarToggle[];
  trailing?: React.ReactNode;
  children?: React.ReactNode;
}

/** Shared toolbar below PageHeader: search, namespace, sort, quick toggles */
export const FilterBar: React.FC<FilterBarProps> = ({
  embedded = false,
  className,
  leading,
  search,
  namespace,
  sort,
  toggles,
  trailing,
  children,
}) => (
  <div
    className={clsx(
      'flex flex-col gap-3',
      !embedded && 'ui-filter-bar',
      className
    )}
  >
    <div className="grid grid-cols-1 gap-3 lg:grid-cols-[auto_minmax(0,1fr)] lg:items-center">
      {leading ? <div className="flex flex-wrap items-center gap-2">{leading}</div> : null}

      <div className={clsx('grid grid-cols-1 gap-3 sm:grid-cols-[minmax(16rem,1fr)_repeat(3,max-content)] sm:items-center lg:justify-end', !leading && 'lg:col-span-2')}>
        {search ? (
          <div className={clsx('relative min-w-0', search.inputClassName)}>
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
            <input
              type="text"
              value={search.value}
              onChange={(e) => search.onChange(e.target.value)}
              onKeyDown={search.onKeyDown}
              placeholder={search.placeholder ?? 'Search…'}
              className="h-10 w-full rounded-lg border border-border bg-base pl-9 pr-4 text-body text-text placeholder:text-muted focus:border-brand focus:outline-none"
            />
          </div>
        ) : null}

        {namespace ? (
          namespace.inputMode ? (
            <>
              <input
                type="text"
                title={namespace.title}
                disabled={namespace.disabled}
                list={namespace.options.length > 0 && namespace.datalistId ? namespace.datalistId : undefined}
                value={namespace.value}
                onChange={(e) => namespace.onChange(e.target.value)}
                onKeyDown={namespace.onKeyDown}
                placeholder={namespace.placeholder ?? namespace.emptyLabel ?? 'All namespaces'}
                className="h-10 w-full rounded-lg border border-border bg-base px-4 text-body text-text placeholder:text-muted-2 focus:border-brand focus:outline-none disabled:opacity-50 sm:w-[14rem]"
              />
              {namespace.options.length > 0 && namespace.datalistId ? (
                <datalist id={namespace.datalistId}>
                  {namespace.options.map((ns) => (
                    <option key={ns} value={ns} />
                  ))}
                </datalist>
              ) : null}
            </>
          ) : (
            <select
              title={namespace.title}
              disabled={namespace.disabled}
              value={namespace.value}
              onChange={(e) => namespace.onChange(e.target.value)}
              className="h-10 w-full rounded-lg border border-border bg-base px-4 text-body text-text focus:border-brand focus:outline-none disabled:opacity-50 sm:w-[14rem]"
            >
              <option value="">{namespace.emptyLabel ?? 'All namespaces'}</option>
              {namespace.options.map((ns) => (
                <option key={ns} value={ns}>
                  {ns}
                </option>
              ))}
            </select>
          )
        ) : null}

        {sort ? (
          <select
            value={sort.value}
            onChange={(e) => sort.onChange(e.target.value)}
            className={clsx(
              'h-10 w-full rounded-lg border border-border bg-base px-4 text-body text-text focus:border-brand focus:outline-none sm:w-56',
              sort.selectClassName
            )}
          >
            {sort.options.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        ) : null}

        {trailing ? <div className="flex min-w-0 flex-wrap items-center gap-2 sm:justify-end">{trailing}</div> : null}
      </div>
    </div>

    {toggles?.length ? (
      <div className="flex flex-wrap items-center gap-2 border-t border-border/60 pt-3">
        {toggles.map((t) => {
          const tone = t.activeTone ?? 'brand';
          const activeCls =
            tone === 'success'
              ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300'
              : tone === 'danger'
                ? 'border-red-500/40 bg-red-500/10 text-red-300'
                : 'border-brand/50 bg-brand/15 text-brand';
          return (
          <button
            key={t.id}
            type="button"
            onClick={t.onClick}
            className={clsx(
              'inline-flex h-8 items-center gap-1.5 whitespace-nowrap rounded-lg border px-3 text-caption font-semibold transition-colors',
              t.active
                ? activeCls
                : 'border-border bg-surface/40 text-muted hover:text-text'
            )}
          >
            {t.icon ? <span className="shrink-0">{t.icon}</span> : null}
            {t.label}
          </button>
          );
        })}
      </div>
    ) : null}

    {children}
  </div>
);
