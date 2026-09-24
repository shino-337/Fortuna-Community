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


const pod = {
  id: 1,
  uid: 'pod-a',
  clusterId: 'cluster-a',
  name: 'pod-a',
  namespace: 'default',
  nodeName: 'node-a',
  phase: 'Running',
  status: 'Running',
  riskCount: 1,
  restartCount: 0,
  podIP: '10.0.0.10',
  startTime: '2026-09-24T00:00:00Z',
};

function fulfillPodSupportingApis(route: import('@playwright/test').Route) {
  const url = new URL(route.request().url());
  const path = url.pathname;
  if (path.endsWith('/sbom')) return route.fulfill({ status: 404, json: { error: 'no sbom' } });
  if (path === '/api/v1/inventory/sbom') return route.fulfill({ json: { sboms: [] } });
  if (path.endsWith('/report')) {
    return route.fulfill({ json: {
      podUid: 'pod-a',
      podName: 'pod-a',
      namespace: 'default',
      clusterId: 'cluster-a',
      insights: [],
      bindings: [],
      roles: [],
      summary: { runtimeSignals24h: 0, insightsInReport: 0 },
    } });
  }
  if (path.includes('/runtime/signals/suppression-stats')) {
    return route.fulfill({ json: { podUid: 'pod-a', suppressed: 0, total: 0 } });
  }
  if (path.includes('/runtime/pods/pod-a/signals')) return route.fulfill({ json: { podUid: 'pod-a', signals: [], count: 0 } });
  if (path.includes('/runtime/pods/pod-a/metrics') ||
      path.includes('/runtime/pods/pod-a/processes') ||
      path.includes('/runtime/pods/pod-a/network') ||
      path.includes('/runtime/pods/pod-a/events')) {
    return route.fulfill({ json: { items: [] } });
  }
  if (path.includes('/risk/pods/pod-a/runtime/events')) return route.fulfill({ json: { events: [] } });
  if (path.includes('/runtime/pods/pod-a/capabilities')) return route.fulfill({ json: { capabilities: [] } });
  if (path.includes('/risk/scores/')) return route.fulfill({ status: 404, json: { error: 'not found' } });
  if (path.includes('/inventory/serviceaccounts')) return route.fulfill({ json: { serviceAccounts: [], total: 0 } });
  return route.fulfill({ status: 404, json: { error: 'not found' } });
}

test('pod primary 503 is unavailable, not not-found', async ({ page }) => {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') {
      return route.fulfill(unavailable('pod_detail_unavailable', 'Pod inventory query failed'));
    }
    return fulfillPodSupportingApis(route);
  });

  await page.goto(`${fixture}?path=/resources/pods/uid/pod-a`);
  await expect(page.getByRole('heading', { name: 'Could not load pod detail', exact: true })).toBeVisible();
  await expect(page.getByText('Pod not found', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Retry pod', exact: true })).toBeVisible();
});

test('pod 404 remains a genuine not-found state', async ({ page }) => {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') {
      return route.fulfill({ status: 404, json: { error: 'not found' } });
    }
    return fulfillPodSupportingApis(route);
  });

  await page.goto(`${fixture}?path=/resources/pods/uid/pod-a`);
  await expect(page.getByText('Pod not found', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('Could not load pod detail', { exact: true })).toHaveCount(0);
});

test('pod refresh 503 preserves last-known-good pod detail', async ({ page }) => {
  let podAttempts = 0;
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') {
      podAttempts++;
      return route.fulfill(
        podAttempts === 1
          ? { json: pod }
          : unavailable('pod_detail_unavailable', 'Pod inventory query failed'),
      );
    }
    return fulfillPodSupportingApis(route);
  });

  await page.goto(`${fixture}?path=/resources/pods/uid/pod-a`);
  await expect(page.getByText('pod-a', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('10.0.0.10', { exact: true })).toBeVisible();

  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByText('Pod detail temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('pod-a', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('10.0.0.10', { exact: true })).toBeVisible();
  await expect(page.getByText('Pod not found', { exact: true })).toHaveCount(0);
});


test('pod runtime evidence 503 remains unavailable instead of empty', async ({ page }) => {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') {
      return route.fulfill({ json: pod });
    }
    if (path.includes('/runtime/pods/pod-a/signals')) {
      return route.fulfill(unavailable('runtime_signals_unavailable', 'Runtime signal query failed'));
    }
    if (path.includes('/runtime/signals/suppression-stats')) {
      return route.fulfill(unavailable('runtime_signal_stats_unavailable', 'Runtime signal statistics query failed'));
    }
    return fulfillPodSupportingApis(route);
  });

  const detailPath = encodeURIComponent('/resources/pods/uid/pod-a?tab=events');
  await page.goto(`${fixture}?path=${detailPath}`);
  await expect(page.getByText(/Failed to load:/)).toBeVisible();
  await expect(page.getByText(/signals/)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible();
});
