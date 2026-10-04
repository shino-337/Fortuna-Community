import { expect, test } from '@playwright/test';
const fixture = '/tests/runtime-signals/identity.html';
const identity = (uid: string, clusterId = 'cluster-a') => ({ uid, name: uid, namespace: 'team', clusterId, linkedPods: '[]' });
const permissions = (uid = 'sa-a', clusterId = 'cluster-a') => ({ serviceAccountId: 1, serviceAccountUid: uid, clusterId, roleBindings: [], clusterRoleBindings: [], effectiveRules: [{ scope: 'namespace', namespace: 'team', verbs: ['get'], apiGroups: [''], resources: ['pods'] }] });

for (const status of [403, 500]) {
  test(`identity HTTP ${status} is an error, not absence`, async ({ page }) => {
    await page.route(/\/inventory\/serviceaccounts\/sa-a(?:\?.*)?$/, r => r.fulfill({ status, json: { error: 'Identity lookup unavailable' } }));
    await page.goto(fixture);
    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page.getByText('Identity not found', { exact: true })).toHaveCount(0);
  });
}

test('permissions failure can be retried and shows grant namespace', async ({ page }) => {
  let attempts = 0;
  let detailCluster = '';
  let permissionsCluster = '';
  await page.route(/\/inventory\/serviceaccounts\/sa-a(?:\?.*)?$/, r => {
    detailCluster = new URL(r.request().url()).searchParams.get('clusterId') || '';
    return r.fulfill({ json: identity('sa-a') });
  });
  await page.route(/\/inventory\/serviceaccounts\/sa-a\/permissions(?:\?.*)?$/, r => {
    permissionsCluster = new URL(r.request().url()).searchParams.get('clusterId') || '';
    return r.fulfill(++attempts === 1 ? { status: 500, json: { error: 'Permission lookup failed' } } : { json: permissions() });
  });
  await page.goto(fixture);
  await expect(page.getByRole('alert')).toContainText('Please try again');
  await expect(page.getByText('No effective rules', { exact: false })).toHaveCount(0);
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByRole('cell', { name: 'Namespace: team', exact: true })).toBeVisible();
  await expect.poll(() => detailCluster).toBe('cluster-a');
  await expect.poll(() => permissionsCluster).toBe('cluster-a');
  await expect(page.getByRole('alert')).toHaveCount(0);
});

test('permission response from another cluster is rejected as unavailable', async ({ page }) => {
  await page.route(/\/inventory\/serviceaccounts\/sa-a(?:\?.*)?$/, r => r.fulfill({ json: identity('sa-a') }));
  await page.route(/\/inventory\/serviceaccounts\/sa-a\/permissions(?:\?.*)?$/, r =>
    r.fulfill({ json: permissions('sa-a', 'cluster-b') }),
  );
  await page.goto(fixture);
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page.getByText('No effective rules', { exact: false })).toHaveCount(0);
});

test('late permission response cannot replace another identity', async ({ page }) => {
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  let started = false;
  await page.route(/\/inventory\/serviceaccounts\/sa-a(?:\?.*)?$/, r => r.fulfill({ json: identity('sa-a') }));
  await page.route(/\/inventory\/serviceaccounts\/sa-b(?:\?.*)?$/, r => r.fulfill({ json: identity('sa-b', 'cluster-b') }));
  await page.route(/\/inventory\/serviceaccounts\/sa-a\/permissions(?:\?.*)?$/, async r => { started = true; await held; await r.fulfill({ json: { ...permissions('sa-a', 'cluster-a'), effectiveRules: [{ scope: 'namespace', namespace: 'OLD', verbs: ['delete'], resources: ['secrets'] }] } }); });
  await page.route(/\/inventory\/serviceaccounts\/sa-b\/permissions(?:\?.*)?$/, r => r.fulfill({ json: permissions('sa-b', 'cluster-b') }));
  await page.goto(fixture);
  await expect.poll(() => started).toBe(true);
  await page.getByRole('link', { name: 'Switch identity' }).click();
  await expect(page.getByRole('cell', { name: 'Namespace: team', exact: true })).toBeVisible();
  const lateResponse = page.waitForResponse(response => new URL(response.url()).pathname.endsWith('/sa-a/permissions'));
  release();
  await lateResponse;
  await page.waitForTimeout(100);
  await expect(page.getByRole('cell', { name: 'Namespace: OLD', exact: true })).toHaveCount(0);
});

test('reviewed revocation uses the server digest and resumes status after reload', async ({ page }) => {
  const digest = 'a'.repeat(64);
  let status = 'preview';
  let executeCount = 0;
  const plan = {
    version: 1,
    namespace: 'team',
    name: 'sa-a',
    steps: [{ kind: 'RoleBinding', namespace: 'team', name: 'admin', uid: 'binding-uid', resourceVersion: '9',
      before: [{ kind: 'ServiceAccount', namespace: 'team', name: 'sa-a' }, { kind: 'User', name: 'alice' }],
      after: [{ kind: 'User', name: 'alice' }] }],
    limitations: ['Bound tokens and existing Pods remain valid.'],
  };
  const response = () => ({ operation: { id: 'op-1', clusterId: 'cluster-a', uid: 'sa-a', action: 'revoke', digest,
    status, completedSteps: status === 'succeeded' ? 1 : 0, attempts: status === 'succeeded' ? 1 : 0,
    expiresAt: new Date(Date.now() + 600_000).toISOString(), retryAt: new Date().toISOString() }, plan });
  await page.route(/\/inventory\/serviceaccounts\/sa-a(?:\?.*)?$/, r => r.fulfill({ json: identity('sa-a') }));
  await page.route(/\/inventory\/serviceaccounts\/sa-a\/permissions(?:\?.*)?$/, r => r.fulfill({ json: permissions() }));
  await page.route(/\/inventory\/serviceaccounts\/sa-a\/mutations\/preview(?:\?.*)?$/, async r => {
    expect(new URL(r.request().url()).searchParams.get('clusterId')).toBe('cluster-a');
    expect(r.request().postDataJSON()).toEqual({ action: 'revoke' });
    await r.fulfill({ json: response() });
  });
  await page.route(/\/inventory\/serviceaccount-mutations\/op-1\/execute(?:\?.*)?$/, async r => {
    expect(new URL(r.request().url()).searchParams.get('clusterId')).toBe('cluster-a');
    expect(r.request().postDataJSON()).toEqual({ digest });
    executeCount++;
    status = 'queued';
    await r.fulfill({ status: 202, json: { operationId: 'op-1', status: 'queued' } });
  });
  await page.route(/\/inventory\/serviceaccount-mutations\/op-1(?:\?.*)?$/, r => r.fulfill({ json: response() }));

  await page.goto(fixture);
  await page.getByRole('button', { name: 'Preview direct grant revocation' }).click();
  await expect(page.getByText('RoleBinding team/admin')).toBeVisible();
  await expect(page.getByText('Before: ServiceAccount team/sa-a, User alice')).toBeVisible();
  await expect(page.getByText('After: User alice')).toBeVisible();
  await expect(page.getByText('Bound tokens and existing Pods remain valid.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Execute reviewed revoke' })).toBeDisabled();
  await page.getByRole('checkbox').check();
  await page.getByRole('button', { name: 'Execute reviewed revoke' }).click();
  await expect.poll(() => executeCount).toBe(1);
  status = 'succeeded';
  await page.getByRole('button', { name: 'Refresh operation' }).click();
  await expect(page.getByText('Completed steps: 1 / 1', { exact: false })).toBeVisible();
  await page.goto(`${fixture}?operationId=op-1`);
  await expect(page.getByText('Completed steps: 1 / 1', { exact: false })).toBeVisible();
  expect(executeCount).toBe(1);
});

test('mutation preview rejects a different cluster and never offers execute', async ({ page }) => {
  await page.route(/\/inventory\/serviceaccounts\/sa-a(?:\?.*)?$/, r => r.fulfill({ json: identity('sa-a') }));
  await page.route(/\/inventory\/serviceaccounts\/sa-a\/permissions(?:\?.*)?$/, r => r.fulfill({ json: permissions() }));
  await page.route(/\/inventory\/serviceaccounts\/sa-a\/mutations\/preview(?:\?.*)?$/, r => r.fulfill({ json: {
    operation: { id: 'foreign', clusterId: 'cluster-b', uid: 'sa-a', action: 'delete', digest: 'a'.repeat(64),
      status: 'preview', completedSteps: 0, attempts: 0, expiresAt: new Date(Date.now() + 600_000).toISOString() },
    plan: { version: 1, namespace: 'team', name: 'sa-a', steps: [], limitations: [] },
  } }));
  await page.goto(fixture);
  await page.getByRole('button', { name: 'Preview ServiceAccount deletion' }).click();
  await expect(page.getByRole('alert')).toContainText('Mutation response has incomplete or mismatched identity');
  await expect(page.getByRole('button', { name: /Execute reviewed/ })).toHaveCount(0);
});

test('read-only identity has no mutation controls', async ({ page }) => {
  await page.route(/\/inventory\/serviceaccounts\/sa-a(?:\?.*)?$/, r => r.fulfill({ json: identity('sa-a') }));
  await page.route(/\/inventory\/serviceaccounts\/sa-a\/permissions(?:\?.*)?$/, r => r.fulfill({ json: permissions() }));
  await page.goto(`${fixture}?readOnly=1`);
  await expect(page.getByText('Synchronized RBAC grants')).toBeVisible();
  await expect(page.getByText('Kubernetes identity action')).toHaveCount(0);
});
