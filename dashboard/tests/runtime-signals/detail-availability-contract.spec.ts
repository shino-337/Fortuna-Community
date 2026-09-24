import { expect, test } from '@playwright/test';

const fixture = '/tests/runtime-signals/availability-details.html';

const cluster = {
  id: 'cluster-a',
  name: 'Alpha',
  source: 'env',
  status: 'active',
  lastSync: '2026-09-24T00:00:00Z',
  k8sVersion: 'v1.29.15',
};
const stats = {
  clusters: [{
    ...cluster,
    connectionStatus: 'connected',
    podCount: 3,
    deploymentCount: 1,
    riskCount: 2,
    agentCount: 1,
  }],
  total: 1,
};
const overview = { podCount: 3, nodeCount: 1, namespaceCount: 2 };

function unavailable(code: string, error: string, retryable = true) {
  return {
    status: 503,
    json: { status: 'unavailable', code, error, retryable },
  };
}

test('cluster primary 503 is unavailable, not not-found', async ({ page }) => {
  await page.route('**/api/v1/inventory/clusters/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/clusters/cluster-a') {
      return route.fulfill(unavailable('cluster_detail_unavailable', 'Cluster record query failed'));
    }
    if (path === '/api/v1/inventory/clusters/stats') return route.fulfill({ json: stats });
    if (path === '/api/v1/inventory/clusters/cluster-a/overview') return route.fulfill({ json: overview });
    return route.fulfill({ status: 404, json: { error: 'not found' } });
  });

  await page.goto(`${fixture}?path=/clusters/cluster-a`);
  await expect(page.getByText('Cluster detail unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('Cluster not found', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Retry cluster', exact: true })).toBeVisible();
});

test('cluster enrichment 503 preserves the primary detail', async ({ page }) => {
  await page.route('**/api/v1/inventory/clusters/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/clusters/cluster-a') return route.fulfill({ json: cluster });
    if (path === '/api/v1/inventory/clusters/stats') {
      return route.fulfill(unavailable('cluster_stats_unavailable', 'Cluster statistics query failed'));
    }
    if (path === '/api/v1/inventory/clusters/cluster-a/overview') {
      return route.fulfill(unavailable('cluster_overview_unavailable', 'Cluster overview query failed'));
    }
    return route.fulfill({ status: 404, json: { error: 'not found' } });
  });

  await page.goto(`${fixture}?path=/clusters/cluster-a`);
  await expect(page.getByText('Alpha', { exact: true })).toBeVisible();
  await expect(page.getByText('Cluster statistics temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('Cluster overview temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('Cluster not found', { exact: true })).toHaveCount(0);
});

test('cluster inventory 503 is not rendered as empty inventory', async ({ page }) => {
  await page.route('**/api/v1/inventory/clusters/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/clusters/cluster-a') return route.fulfill({ json: cluster });
    if (path === '/api/v1/inventory/clusters/stats') return route.fulfill({ json: stats });
    if (path === '/api/v1/inventory/clusters/cluster-a/overview') return route.fulfill({ json: overview });
    if (path === '/api/v1/inventory/clusters/cluster-a/inventory') {
      return route.fulfill(unavailable('cluster_inventory_unavailable', 'Cluster inventory query failed'));
    }
    return route.fulfill({ status: 404, json: { error: 'not found' } });
  });

  await page.goto(`${fixture}?path=/clusters/cluster-a`);
  await page.getByRole('tab', { name: 'Inventory', exact: true }).click();
  await expect(page.getByText('Cluster inventory temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('No inventory data', { exact: true })).toHaveCount(0);
});

test('capability metadata 503 is unavailable, not not-found', async ({ page }) => {
  await page.route('**/api/v1/capability-metadata/CAP_TEST', route =>
    route.fulfill(unavailable('capability_metadata_unavailable', 'Capability metadata query failed')),
  );
  await page.route('**/api/v1/policy/rules?*', route => route.fulfill({ json: { rules: [], total: 0 } }));

  await page.goto(`${fixture}?path=/capabilities/CAP_TEST`);
  await expect(page.getByText('Capability metadata temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('Capability not found', { exact: true })).toHaveCount(0);
});

test('linked-rule failure is not presented as no rules', async ({ page }) => {
  await page.route('**/api/v1/capability-metadata/CAP_TEST', route =>
    route.fulfill({ json: {
      capabilityId: 'CAP_TEST',
      name: 'Test capability',
      summary: 'Fixture capability',
      severityBase: 'medium',
      domain: 'runtime',
      category: 'test',
      confidenceBase: 0.8,
    } }),
  );
  await page.route('**/api/v1/policy/rules?*', route =>
    route.fulfill(unavailable('policy_rules_unavailable', 'Policy rule query failed')),
  );

  await page.goto(`${fixture}?path=/capabilities/CAP_TEST`);
  await expect(page.getByText('Test capability', { exact: true })).toBeVisible();
  await expect(page.getByText('Linked policy rules temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText(/No rules reference this capability/)).toHaveCount(0);
});

test('node refresh 503 preserves last-known-good node detail', async ({ page }) => {
  let attempts = 0;
  await page.route('**/api/v1/inventory/clusters/cluster-a/nodes/node-a?*', route => {
    attempts++;
    if (attempts === 1) {
      return route.fulfill({ json: {
        nodeName: 'node-a',
        clusterId: 'cluster-a',
        podCount: 1,
        kubeletVersion: 'v1.29.15',
        pods: [{ uid: 'pod-a', name: 'pod-a', namespace: 'default', riskCount: 0 }],
      } });
    }
    return route.fulfill(unavailable('cluster_node_unavailable', 'Node query failed'));
  });

  await page.goto(`${fixture}?path=/clusters/cluster-a/nodes/node-a`);
  await expect(page.getByText('v1.29.15', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByText('Node detail temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('v1.29.15', { exact: true })).toBeVisible();
  await expect(page.getByText('Node not found', { exact: true })).toHaveCount(0);
});
