/**
 * Shared table chrome (headers, rows, cells) for dashboard pages.
 * Typography: headers `text-micro` uppercase; body `text-body`; compact cells `text-caption`.
 */

export const UI_TABLE = 'w-full border-collapse text-left text-text';

/** Apply on `<thead>` when the header should stay visible inside `ui-table-scroll`. */
export const UI_THEAD_STICKY = 'sticky top-0 z-10';

/**
 * Standard `<th>` — background + border for sticky stacking.
 * Add widths / `text-right` / `hidden sm:table-cell` after this string.
 */
export const UI_TH =
  'border-b border-border bg-base/95 px-4 py-3 text-left text-micro font-semibold uppercase tracking-wide text-muted backdrop-blur-sm sm:px-6 sm:py-3.5';

/** Slightly denser header (overview subtables, heatmap-adjacent lists). */
export const UI_TH_COMPACT =
  'border-b border-border bg-base/95 px-3 py-2 text-left text-micro font-semibold uppercase tracking-wide text-muted backdrop-blur-sm sm:px-4 sm:py-2.5';

/** Default body row */
export const UI_TR = 'border-b border-border/60 transition-colors hover:bg-surface-2/40';

/** Primary data cell (14px) */
export const UI_TD = 'px-4 py-3 align-top text-body text-text sm:px-6 sm:py-3.5';

/** Dense cell (12px) — secondary grids, dashboard-style tables */
export const UI_TD_COMPACT = 'px-4 py-3 align-top text-caption text-text sm:px-6 sm:py-3.5';

/** Dense cell matching `UI_TH_COMPACT` horizontal padding */
export const UI_TD_COMPACT_TIGHT = 'px-3 py-2 align-top text-caption text-text sm:px-4 sm:py-2.5';
