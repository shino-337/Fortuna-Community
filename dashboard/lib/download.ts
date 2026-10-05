/**
 * Browser file downloads shared by every export in the dashboard.
 *
 * The anchor is attached before the click (Firefox ignores clicks on detached
 * anchors) and the object URL is released on the next tick, because revoking
 * it synchronously can cancel the download before the browser starts it.
 */
export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.rel = 'noopener';
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
}

export function downloadText(content: string, filename: string, type: string): void {
  downloadBlob(new Blob([content], { type }), filename);
}

/**
 * Quote one CSV cell. Values that start with a formula trigger are prefixed
 * with an apostrophe: finding titles, pod and package names come from cluster
 * objects, and a spreadsheet would otherwise evaluate "=HYPERLINK(...)".
 */
export function csvCell(value: unknown): string {
  let s = value == null ? '' : String(value);
  if (/^[=+\-@\t\r]/.test(s)) s = `'${s}`;
  return `"${s.replace(/"/g, '""')}"`;
}

export function toCsv(rows: unknown[][]): string {
  return rows.map((row) => row.map(csvCell).join(',')).join('\n');
}
