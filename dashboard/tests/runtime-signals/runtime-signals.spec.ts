import { expect, test } from '@playwright/test';

const fixture = '/tests/runtime-signals/index.html';
for (const [status, message] of [
  [401, 'Your session has expired'],
  [403, 'You do not have access'],
  [500, 'Runtime evidence could not be loaded'],
] as const) {
  test(`HTTP ${status} is not presented as empty evidence`, async ({ page }) => {
    await page.route('**/runtime/signals?*', route => route.fulfill({ status, json: { error: 'test failure' } }));
    await page.goto(fixture);
    await expect(page.getByRole('alert')).toContainText(message);
    await expect(page.getByText('No runtime evidence found', { exact: true })).toHaveCount(0);
    await expect(page.getByText('0 runtime events', { exact: false })).toHaveCount(0);
  });
}

test('retry can recover to a genuine empty result', async ({ page }) => {
  let attempts = 0;
  await page.route('**/runtime/signals?*', route => {
    attempts++;
    return route.fulfill(attempts === 1
      ? { status: 500, json: { error: 'unavailable' } }
      : { json: { signals: [], count: 0, total: 0 } });
  });
  await page.goto(fixture);
  await expect(page.getByRole('alert')).toBeVisible();
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(page.getByText('No runtime evidence found', { exact: true })).toBeVisible();
});

test('network failure is not presented as empty evidence', async ({ page }) => {
  await page.route('**/runtime/signals?*', route => route.abort('failed'));
  await page.goto(fixture);
  await expect(page.getByRole('alert')).toContainText('could not be loaded');
  await expect(page.getByText('No runtime evidence found', { exact: true })).toHaveCount(0);
});

test('successful evidence is rendered', async ({ page }) => {
  await page.route('**/runtime/signals?*', route => route.fulfill({ json: {
    signals: [{ id: 1, signalType: 'PROC_ROOT_PIVOT', category: 'PROCESS', podUid: 'lab-pod', confidence: 0.8, createdAt: '2026-01-01T00:00:00Z' }],
    count: 1, total: 1,
  } }));
  await page.goto(fixture);
  await expect(page.getByRole('cell').filter({ hasText: 'PROC_ROOT_PIVOT' })).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(page.getByText('No runtime evidence found', { exact: true })).toHaveCount(0);
});
