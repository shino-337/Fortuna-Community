export const TYPOGRAPHY = {
  pageTitle: 'text-2xl font-bold',
  cardTitle: 'text-lg font-semibold',
  body: 'text-sm',
  helper: 'text-xs',
  badge: 'text-[11px] font-semibold',
  microLabel: 'text-[10px] font-semibold tracking-wider uppercase',
} as const;

export type TypographyToken = keyof typeof TYPOGRAPHY;

