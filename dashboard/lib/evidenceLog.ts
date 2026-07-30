import type { Insight } from '../types';

export type EvidenceLogKind = 'fact' | 'rule' | 'capability' | 'signal' | 'event' | 'resource' | 'field';

export interface EvidenceLogEntry {
  id: string;
  kind: EvidenceLogKind;
  label: string;
  value: string;
  source: 'evidence' | 'violated_rules' | 'evidence_refs' | 'evidence_chain_refs';
}

function parseJsonish(raw: unknown): unknown {
  if (typeof raw !== 'string') return raw;
  const trimmed = raw.trim();
  if (!trimmed) return undefined;
  try {
    return JSON.parse(trimmed);
  } catch {
    return trimmed;
  }
}

function stringifyValue(value: unknown): string {
  if (value == null) return '';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function titleCaseKey(key: string): string {
  return key
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/[_-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (m) => m.toUpperCase());
}

function inferKind(key: string): EvidenceLogKind {
  const k = key.toLowerCase();
  if (k.includes('rule') || k === 'role' || k === 'rolekind') return 'rule';
  if (k.includes('capability')) return 'capability';
  if (k.includes('signal')) return 'signal';
  if (k.includes('event')) return 'event';
  if (k.includes('resource') || k.includes('pod') || k.includes('hostpath') || k.includes('serviceaccount')) return 'resource';
  if (k.includes('fact') || k.includes('evidence')) return 'fact';
  return 'field';
}

function pushEntry(
  entries: EvidenceLogEntry[],
  seen: Set<string>,
  kind: EvidenceLogKind,
  label: string,
  value: unknown,
  source: EvidenceLogEntry['source'],
): void {
  const text = stringifyValue(value).trim();
  if (!text) return;
  const key = `${source}:${kind}:${label}:${text}`;
  if (seen.has(key)) return;
  seen.add(key);
  entries.push({
    id: `${source}-${entries.length}`,
    kind,
    label,
    value: text,
    source,
  });
}

function addEvidenceObject(entries: EvidenceLogEntry[], seen: Set<string>, raw: unknown): void {
  const parsed = parseJsonish(raw);
  if (!parsed) return;
  if (typeof parsed === 'string') {
    pushEntry(entries, seen, 'field', 'Raw evidence', parsed, 'evidence');
    return;
  }
  if (Array.isArray(parsed)) {
    parsed.forEach((item, index) => pushEntry(entries, seen, 'fact', `Evidence ${index + 1}`, item, 'evidence'));
    return;
  }
  if (typeof parsed !== 'object') {
    pushEntry(entries, seen, 'field', 'Evidence', parsed, 'evidence');
    return;
  }
  Object.entries(parsed as Record<string, unknown>).forEach(([key, value]) => {
    if (Array.isArray(value)) {
      value.forEach((item, index) => {
        const suffix = value.length > 1 ? ` ${index + 1}` : '';
        pushEntry(entries, seen, inferKind(key), `${titleCaseKey(key)}${suffix}`, item, 'evidence');
      });
      return;
    }
    pushEntry(entries, seen, inferKind(key), titleCaseKey(key), value, 'evidence');
  });
}

function addViolatedRules(entries: EvidenceLogEntry[], seen: Set<string>, raw: unknown): void {
  const parsed = parseJsonish(raw);
  if (!parsed) return;
  const values = Array.isArray(parsed) ? parsed : [parsed];
  values.forEach((item, index) => {
    if (item && typeof item === 'object') {
      const obj = item as Record<string, unknown>;
      const label = stringifyValue(obj.ruleId ?? obj.rule_id ?? obj.id ?? `Rule ${index + 1}`);
      pushEntry(entries, seen, 'rule', label || `Rule ${index + 1}`, obj, 'violated_rules');
    } else {
      pushEntry(entries, seen, 'rule', `Rule ${index + 1}`, item, 'violated_rules');
    }
  });
}

function addEvidenceRefs(entries: EvidenceLogEntry[], seen: Set<string>, refs: Insight['evidence_refs']): void {
  if (!refs) return;
  const groups: Array<[EvidenceLogKind, string, unknown[] | undefined]> = [
    ['event', 'Event ID', refs.eventIds],
    ['fact', 'Fact ID', refs.factIds],
    ['signal', 'Signal type', refs.signalTypes],
    ['signal', 'Incident type', refs.incidentTypes],
    ['capability', 'Capability ID', refs.capabilityIds],
    ['rule', 'Rule ID', refs.ruleIds],
  ];
  groups.forEach(([kind, label, values]) => {
    values?.forEach((value) => pushEntry(entries, seen, kind, label, value, 'evidence_refs'));
  });
}

function addEvidenceChain(entries: EvidenceLogEntry[], seen: Set<string>, chain: Insight['evidence_chain_refs']): void {
  chain?.forEach((row) => {
    const layer = row.layer || 'reference';
    pushEntry(entries, seen, inferKind(layer), titleCaseKey(layer), row.ref, 'evidence_chain_refs');
  });
}

/** Build an array of evidence log entries from an insight. */
export function buildEvidenceLogEntries(insight: Pick<Insight, 'evidence' | 'violatedRules' | 'evidence_refs' | 'evidence_chain_refs'>): EvidenceLogEntry[] {
  const entries: EvidenceLogEntry[] = [];
  const seen = new Set<string>();
  addEvidenceRefs(entries, seen, insight.evidence_refs);
  addEvidenceChain(entries, seen, insight.evidence_chain_refs);
  addViolatedRules(entries, seen, insight.violatedRules);
  addEvidenceObject(entries, seen, insight.evidence);
  return entries;
}

/** Summarize evidence log entries as a pipe-delimited string. */
export function summarizeEvidenceLog(insight: Pick<Insight, 'evidence' | 'violatedRules' | 'evidence_refs' | 'evidence_chain_refs'>, max = 3): string {
  const entries = buildEvidenceLogEntries(insight);
  if (entries.length === 0) return 'No structured evidence';
  return entries
    .slice(0, max)
    .map((entry) => `${entry.label}: ${entry.value}`)
    .join(' | ');
}
