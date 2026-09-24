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
});

test('successful 200 empty cluster inventory keeps the normal empty state', async ({ page }) => {
  await page.route('**/api/v1/inventory/clusters/stats', route =>
    route.fulfill({ json: { clusters: [], total: 0, dataStatus: 'available' } }),
  );

  await page.goto(fixture);
  await expect(page.getByText('No clusters match current filters', { exact: true })).toBeVisible();
  await expect(page.getByText(/temporarily unavailable|requires operator action/)).toHaveCount(0);
});
