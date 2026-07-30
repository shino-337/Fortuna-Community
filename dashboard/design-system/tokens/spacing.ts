export const SPACING = {
  /** Vertical rhythm between major page blocks (matches PageLayout gap-6) */
  sectionY: 'space-y-6',
  /** Horizontal inset comes from Layout main; keep 0 here so content uses full usable width */
  pageX: 'px-0',
  pageY: 'py-6',
  cardPrimary: 'p-6',
  cardSecondary: 'p-4',
  cardPanel: 'p-3',
  gapSm: 'gap-2',
  gapMd: 'gap-4',
  gapLg: 'gap-6',
} as const;

export type SpacingToken = keyof typeof SPACING;

