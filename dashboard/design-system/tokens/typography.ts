export const TYPOGRAPHY = {
  pageTitle: 'text-page-title text-text',
  sectionTitle: 'text-section-title text-text',
  cardTitle: 'text-card-title font-semibold text-text',
  body: 'text-body text-text',
  caption: 'text-caption text-text',
  meta: 'text-meta text-muted-2',
  micro: 'text-micro text-muted-2',
  nano: 'text-nano text-muted-2',
  helper: 'text-caption text-muted-2',
  badge: 'text-caption font-semibold',
  microLabel: 'ui-micro-label',
} as const;

export type TypographyToken = keyof typeof TYPOGRAPHY;

