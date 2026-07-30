/** Recharts colors aligned with CSS tokens in index.css (:root). */
export interface ChartThemeColors {
  grid: string;
  axis: string;
  tooltipBg: string;
  tooltipBorder: string;
  threshold: string;
  riskLine: string;
  pceLine: string;
  spike: string;
  critical: string;
  high: string;
  medium: string;
  low: string;
  muted: string;
}

const FALLBACK: ChartThemeColors = {
  grid: 'rgb(var(--color-border))',
  axis: 'rgb(var(--color-muted))',
  tooltipBg: 'rgb(var(--color-surface))',
  tooltipBorder: 'rgb(var(--color-border))',
  threshold: 'rgb(var(--color-muted-2))',
  riskLine: 'rgb(var(--color-critical))',
  pceLine: 'rgb(var(--color-info))',
  spike: 'rgb(var(--color-critical))',
  critical: 'rgb(var(--color-critical))',
  high: 'rgb(var(--color-high))',
  medium: 'rgb(var(--color-medium))',
  low: 'rgb(var(--color-info))',
  muted: 'rgb(var(--color-muted))',
};

function cssRgb(varName: string, fallback: string): string {
  if (typeof document === 'undefined') return fallback;
  const raw = getComputedStyle(document.documentElement).getPropertyValue(varName).trim();
  if (!raw) return fallback;
  return `rgb(${raw.replace(/\s+/g, ', ')})`;
}

export function getChartThemeColors(): ChartThemeColors {
  return {
    grid: cssRgb('--color-border', FALLBACK.grid),
    axis: cssRgb('--color-muted', FALLBACK.axis),
    tooltipBg: cssRgb('--color-surface', FALLBACK.tooltipBg),
    tooltipBorder: cssRgb('--color-border', FALLBACK.tooltipBorder),
    threshold: cssRgb('--color-muted-2', FALLBACK.threshold),
    riskLine: cssRgb('--color-critical', FALLBACK.riskLine),
    pceLine: cssRgb('--color-info', FALLBACK.pceLine),
    spike: cssRgb('--color-critical', FALLBACK.spike),
    critical: cssRgb('--color-critical', FALLBACK.critical),
    high: cssRgb('--color-high', FALLBACK.high),
    medium: cssRgb('--color-medium', FALLBACK.medium),
    low: cssRgb('--color-info', FALLBACK.low),
    muted: cssRgb('--color-muted', FALLBACK.muted),
  };
}
