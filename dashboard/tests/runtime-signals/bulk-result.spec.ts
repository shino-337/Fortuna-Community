import { expect, test } from '@playwright/test';
import { summarizeBulkFindingResult } from '../../lib/bulkFindingResult';

for (const scenario of [
  { name: 'all succeeded', result: { success_count: 2, failed_count: 0 }, remaining: [], complete: true },
  { name: 'partial failure', result: { success_count: 1, failed_count: 1, errors: [{ id: 'b', error: 'not found' }] }, remaining: ['b'], complete: false },
  { name: 'all failed', result: { success_count: 0, failed_count: 2, errors: [{ id: 'a', error: 'failed' }, { id: 'b', error: 'failed' }] }, remaining: ['a', 'b'], complete: false },
  { name: 'missing error IDs', result: { success_count: 1, failed_count: 1 }, remaining: ['a', 'b'], complete: false },
  { name: 'inconsistent zero counts', result: { success_count: 0, failed_count: 0 }, remaining: ['a', 'b'], complete: false },
]) {
  test(`bulk feedback: ${scenario.name}`, () => {
    const outcome = summarizeBulkFindingResult({ action: 'resolve', ...scenario.result }, ['a', 'b']);
    expect(outcome.complete).toBe(scenario.complete);
    expect(outcome.remainingIds).toEqual(scenario.remaining);
    if (!scenario.complete) expect(outcome.message).toContain('failed');
  });
}
