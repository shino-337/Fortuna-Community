/**
 * Shared classes for filters / forms — keep in sync with {@link FilterBar} controls.
 */

/** Selects and text inputs in toolbars (matches FilterBar inner controls). */
export const UI_FILTER_SELECT =
  'rounded-lg border border-border bg-base px-4 py-2 text-body text-text focus:border-brand focus:outline-none';

/** Slightly tighter select (e.g. secondary filters in a row). */
export const UI_FILTER_SELECT_SM =
  'rounded-lg border border-border bg-base px-3 py-2 text-body text-text focus:border-brand focus:outline-none';

/** Full-width fields on login / auth panels (same tone as toolbar inputs). */
export const UI_AUTH_INPUT =
  'w-full rounded-lg border border-border bg-base px-4 py-2 text-body text-text placeholder:text-muted-2 transition-all focus:border-brand focus:outline-none';

// --- Pills / segments (sort, filter chips, sub-view toggles; not FilterBar row) ---

/** Selected: brand fill + light label. Add `shadow` / `shadow-md` in clsx when needed. */
export const UI_PILL_ACTIVE = 'bg-brand-strong text-white';

/** Selected with elevation (Risk Center filter row). */
export const UI_PILL_ACTIVE_ELEVATED = 'bg-brand-strong text-white shadow-md';

/** Selected with visible border (SBOM severity/status chips). */
export const UI_PILL_ACTIVE_BORDERED = 'bg-brand-strong text-white border-brand';

/** Idle compact sort chip (SBOM Name / Severity / CVEs). */
export const UI_PILL_IDLE_COMPACT = 'bg-surface-2 text-muted hover:text-text';

/** Idle segment pair (Pod network Summary / Raw). */
export const UI_PILL_IDLE_SEGMENT = 'bg-surface-2 text-text hover:bg-surface-2/90';

/** Idle bordered chip (SBOM severity/status when unselected). */
export const UI_PILL_IDLE_FILTER = 'bg-surface-2/90 text-text border-border hover:border-border hover:text-text';

/** Idle rounded-full filters (Insights risk level / workflow). */
export const UI_PILL_IDLE_ROUNDED = 'bg-surface text-muted hover:bg-surface-2 border border-border';

/** Idle cell in a bordered split control (Insights layout toggle). */
export const UI_PILL_IDLE_SPLIT = 'bg-surface text-muted hover:bg-surface-2';

/** Selectable list card — selected (e.g. SBOM pod row). */
export const UI_SELECTABLE_ACTIVE = 'bg-brand-strong/10 border-brand shadow-lg shadow-black/20';

/** Selectable list card — idle */
export const UI_SELECTABLE_IDLE = 'bg-surface border-border hover:border-border';
