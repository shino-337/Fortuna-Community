/** Human-readable duration from a minute count (for UI scope labels). */
export function formatMinutesHuman(minutes: number | undefined): string {
  if (minutes == null || !Number.isFinite(minutes) || minutes <= 0) return 'all time';
  const m = Math.round(minutes);
  if (m < 60) return `${m} min`;
  if (m < 1440) {
    const h = m / 60;
    const rounded = Number.isInteger(h) ? h : Math.round(h * 10) / 10;
    return `${rounded} hour${rounded === 1 ? '' : 's'}`;
  }
  const d = m / 1440;
  const rounded = Number.isInteger(d) ? d : Math.round(d * 10) / 10;
  return `${rounded} day${rounded === 1 ? '' : 's'}`;
}
