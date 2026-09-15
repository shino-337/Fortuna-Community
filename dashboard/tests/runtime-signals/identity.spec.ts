import { expect, test } from '@playwright/test';
const fixture = '/tests/runtime-signals/identity.html';
const identity = (uid: string) => ({ uid, name: uid, namespace: 'team', clusterId: 'cluster-a', linkedPods: '[]' });
const permissions = { roleBindings: [], clusterRoleBindings: [], effectiveRules: [{ scope: 'namespace', namespace: 'team', verbs: ['get'], apiGroups: [''], resources: ['pods'] }] };

for (const status of [403, 500]) {
  test(`identity HTTP ${status} is an error, not absence`, async ({ page }) => {
    await page.route('**/inventory/serviceaccounts/sa-a', r => r.fulfill({ status, json: { error: 'Identity lookup unavailable' } }));
    await page.goto(fixture);
    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page.getByText('Identity not found', { exact: true })).toHaveCount(0);
  });
}

test('permissions failure can be retried and shows grant namespace', async ({ page }) => {
  let attempts = 0;
  await page.route('**/inventory/serviceaccounts/sa-a', r => r.fulfill({ json: identity('sa-a') }));
  await page.route('**/inventory/serviceaccounts/sa-a/permissions', r => r.fulfill(++attempts === 1 ? { status: 500, json: { error: 'Permission lookup failed' } } : { json: permissions }));
  await page.goto(fixture);
  await expect(page.getByRole('alert')).toContainText('Please try again');
  await expect(page.getByText('No effective rules', { exact: false })).toHaveCount(0);
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('cell', { name: 'Namespace: team', exact: true })).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
});

test('late permission response cannot replace another identity', async ({ page }) => {
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  let started = false;
  await page.route('**/inventory/serviceaccounts/sa-a', r => r.fulfill({ json: identity('sa-a') }));
  await page.route('**/inventory/serviceaccounts/sa-b', r => r.fulfill({ json: identity('sa-b') }));
  await page.route('**/inventory/serviceaccounts/sa-a/permissions', async r => { started = true; await held; await r.fulfill({ json: { ...permissions, effectiveRules: [{ scope: 'namespace', namespace: 'OLD', verbs: ['delete'], resources: ['secrets'] }] } }); });
  await page.route('**/inventory/serviceaccounts/sa-b/permissions', r => r.fulfill({ json: permissions }));
  await page.goto(fixture);
  await expect.poll(() => started).toBe(true);
  await page.getByRole('link', { name: 'Switch identity' }).click();
  await expect(page.getByRole('cell', { name: 'Namespace: team', exact: true })).toBeVisible();
  const lateResponse = page.waitForResponse(response => response.url().endsWith("/sa-a/permissions"));
  release();
  await lateResponse;
  await page.waitForTimeout(100);
  await expect(page.getByRole('cell', { name: 'Namespace: OLD', exact: true })).toHaveCount(0);
});
