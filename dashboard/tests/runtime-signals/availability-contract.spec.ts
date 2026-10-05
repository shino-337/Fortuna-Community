import { expect, test } from '@playwright/test';

const fixture = '/tests/runtime-signals/availability-clusters.html';
const success = {
  clusters: [
    {
      id: 'cluster-a',
      name: 'Alpha',
      source: 'env',
      status: 'active',
      connectionStatus: 'connected',
      riskCount: 2,
      agentCount: 1,
      podCount: 3,
      deploymentCount: 1,
      serviceAccountCount: 1,
      roleCount: 1,
      clusterRoleCount: 1,
      roleBindingCount: 1,
      clusterRoleBindingCount: 1,
    },
  ],
  total: 1,
  dataStatus: 'available',
};

test('retryable 503 preserves last-known cluster data and exposes Retry', async ({ page }) => {
  let attempts = 0;
  await page.route('**/api/v1/inventory/clusters/stats', route => {
    attempts++;
    if (attempts === 1 || attempts >= 3) {
      return route.fulfill({ json: success });
    }
    return route.fulfill({
      status: 503,
      json: {
        status: 'unavailable',
        code: 'cluster_stats_pods_unavailable',
        retryable: true,
        error: 'Cluster statistics query is temporarily unavailable',
      },
    });
  });

  await page.goto(fixture);
  await expect(page.getByText('Alpha', { exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByText('Cluster inventory temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('Alpha', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByText('Cluster inventory temporarily unavailable', { exact: true })).toHaveCount(0);
  await expect(page.getByText('Alpha', { exact: true })).toBeVisible();
});

test('non-retryable schema 503 shows migration and operator guidance', async ({ page }) => {
  await page.route('**/api/v1/inventory/clusters/stats', route =>
    route.fulfill({
      status: 503,
      json: {
        status: 'unavailable',
        code: 'cluster_stats_schema_unavailable',
        retryable: false,
        error: 'Cluster statistics require the inventory schema',
      },
    }),
  );

  await page.goto(fixture);
  await expect(page.getByText('Cluster inventory requires operator action', { exact: true })).toBeVisible();
  await expect(page.getByText(/Apply the required migration\/deployment repair or contact the platform operator/).first()).toBeVisible();
  await expect(page.getByText('No clusters match current filters', { exact: true })).toHaveCount(0);
  await expect(page.getByText('No clusters registered', { exact: true })).toHaveCount(0);
});

test('successful 200 empty cluster inventory keeps the normal empty state', async ({ page }) => {
  await page.route('**/api/v1/inventory/clusters/stats', route =>
    route.fulfill({ json: { clusters: [], total: 0, dataStatus: 'available' } }),
  );

  await page.goto(fixture);
  await expect(page.getByText('No clusters registered', { exact: true })).toBeVisible();
  await expect(page.getByText(/temporarily unavailable|requires operator action/)).toHaveCount(0);
});

test('resource and capability adapters keep unavailable distinct from empty', async ({ page }) => {
  let mode: 'unavailable' | 'malformed' | 'malformed-row' | 'empty' = 'unavailable';
  await page.route('**/api/v1/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/clusters/stats') {
      return route.fulfill({ json: { clusters: [], total: 0, dataStatus: 'available' } });
    }
    if (mode === 'unavailable') {
      return route.fulfill({ status: 503, json: { status: 'unavailable', code: 'inventory_query_failed', error: 'Query failed', retryable: true } });
    }
    if (mode === 'malformed') return route.fulfill({ json: {} });
    if (mode === 'malformed-row' && path.endsWith('/trends')) {
      return route.fulfill({ json: { points: [{ critical: 0, high: 0, medium: 0, low: 0 }], total: 1 } });
    }
    if (mode === 'malformed-row' && path === '/api/v1/resources') {
      return route.fulfill({ json: { resources: [{ kind: 'RoleBinding', name: 'rb', uid: 'uid' }], total: 1 } });
    }
    if (mode === 'malformed-row' && path.includes('/summary/')) {
      return route.fulfill({ json: { summary: [{ capabilityId: 'CAP_SYS_ADMIN' }], total: 1 } });
    }
    if (path === '/api/v1/resources') return route.fulfill({ json: { resources: [], total: 0 } });
    if (path.endsWith('/trends')) return route.fulfill({ json: { points: [], total: 0 } });
    if (path.endsWith('/pod-capabilities')) return route.fulfill({ json: { capabilities: [], total: 0 } });
    return route.fulfill({ json: { summary: [], total: 0 } });
  });

  await page.goto(fixture);
  const callAdapters = () => page.evaluate(async () => {
    const { api } = await import('../../lib/api.ts');
    const calls = [
      api.getResources('RoleBinding'),
      api.getPceSummaryByCapability(),
      api.getPceCapabilities(),
      api.getPceTrend(),
    ];
    return Promise.all(calls.map(async call => {
      try { return { value: await call }; }
      catch (error) { return { status: (error as { status?: number }).status }; }
    }));
  });

  expect((await callAdapters()).map(result => result.status)).toEqual([503, 503, 503, 503]);
  mode = 'malformed';
  expect((await callAdapters()).map(result => result.status)).toEqual([502, 502, 502, 502]);
  mode = 'malformed-row';
  const malformedRows = await callAdapters();
  expect(malformedRows[0].status).toBe(502);
  expect(malformedRows[1].status).toBe(502);
  expect(malformedRows[3].status).toBe(502);
  mode = 'empty';
  const empty = await callAdapters();
  expect(empty.map(result => result.status)).toEqual([undefined, undefined, undefined, undefined]);
  expect(empty[0].value).toEqual([]);
  expect(empty[1].value).toEqual([]);
  expect(empty[2].value).toEqual({ capabilities: [], total: 0 });
  expect(empty[3].value).toEqual([]);
});
