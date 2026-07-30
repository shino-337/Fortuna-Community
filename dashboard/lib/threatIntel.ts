/**
 * Parse Core `insights.evidence` JSON (EPSS / CISA KEV from CVE matcher).
 */
export function parseThreatIntelEvidence(
  evidence: string | Record<string, unknown> | undefined | null
): {
  epss?: number;
  epssPercentile?: number;
  cisaKev?: boolean;
  epssSource?: string;
} {
  if (evidence == null) return {};
  let obj: Record<string, unknown>;
  if (typeof evidence === 'string') {
    const s = evidence.trim();
    if (!s) return {};
    try {
      obj = JSON.parse(s) as Record<string, unknown>;
    } catch {
      return {};
    }
  } else {
    obj = evidence;
  }
  const epss = typeof obj.epss === 'number' ? obj.epss : undefined;
  const epssPercentile =
    typeof obj.epss_percentile === 'number' ? obj.epss_percentile : undefined;
  const cisaKev = obj.cisa_kev === true;
  const epssSource = typeof obj.epss_source === 'string' ? obj.epss_source : undefined;
  return { epss, epssPercentile, cisaKev, epssSource };
}
