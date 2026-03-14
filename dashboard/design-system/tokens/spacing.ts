export const SPACING = {
  sectionY: 'space-y-8',
  pageX: 'px-6',
  pageY: 'py-6',
  cardPrimary: 'p-6',
  cardSecondary: 'p-4',
  cardPanel: 'p-3',
  gapSm: 'gap-2',
  gapMd: 'gap-4',
  gapLg: 'gap-6',
} as const;

export type SpacingToken = keyof typeof SPACING;

