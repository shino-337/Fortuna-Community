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
    serviceAccountCount: 1,
    roleCount: 1,
    clusterRoleCount: 1,
    roleBindingCount: 1,
    clusterRoleBindingCount: 1,
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

test('cluster inventory malformed 200 is unavailable, not empty', async ({ page }) => {
  await page.route('**/api/v1/inventory/clusters/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/clusters/cluster-a') return route.fulfill({ json: cluster });
    if (path === '/api/v1/inventory/clusters/stats') return route.fulfill({ json: stats });
    if (path === '/api/v1/inventory/clusters/cluster-a/overview') return route.fulfill({ json: overview });
    if (path === '/api/v1/inventory/clusters/cluster-a/inventory') {
      return route.fulfill({ json: {} });
    }
    return route.fulfill({ status: 404, json: { error: 'not found' } });
  });

  await page.goto(`${fixture}?path=/clusters/cluster-a`);
  await page.getByRole('tab', { name: 'Inventory', exact: true }).click();
  await expect(page.getByText('Cluster inventory temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('No inventory data', { exact: true })).toHaveCount(0);
});

test('cluster tab issue never leaks to a different active tab', async ({ page }) => {
  await page.route('**/api/v1/inventory/clusters/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/clusters/cluster-a') return route.fulfill({ json: cluster });
    if (path === '/api/v1/inventory/clusters/stats') return route.fulfill({ json: stats });
    if (path === '/api/v1/inventory/clusters/cluster-a/overview') return route.fulfill({ json: overview });
    if (path === '/api/v1/inventory/clusters/cluster-a/inventory') {
      return route.fulfill({ json: { nodes: ['node-a'], namespaces: ['default'] } });
    }
    if (path === '/api/v1/inventory/clusters/cluster-a/agents') {
      return route.fulfill(unavailable('cluster_agents_unavailable', 'Cluster Agent inventory query failed'));
    }
    return route.fulfill({ status: 404, json: { error: 'not found' } });
  });

  await page.goto(`${fixture}?path=/clusters/cluster-a`);
  await page.getByRole('tab', { name: 'Inventory', exact: true }).click();
  await expect(page.getByText('node-a', { exact: true })).toBeVisible();

  await page.getByRole('tab', { name: 'Agents', exact: true }).click();
  await expect(page.getByText('Cluster agents temporarily unavailable', { exact: true })).toBeVisible();

  await page.getByRole('tab', { name: 'Inventory', exact: true }).click();
  await expect(page.getByText('node-a', { exact: true })).toBeVisible();
  await expect(page.getByText('Cluster agents temporarily unavailable', { exact: true })).toHaveCount(0);
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


test('malformed linked-rule success is unavailable instead of no linked rules', async ({ page }) => {
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
    route.fulfill({ json: {
      rules: [{ id: 'RULE-1', name: 'Broken contract', severity: 'high', enabled: true }],
      total: 1,
    } }),
  );

  await page.goto(`${fixture}?path=/capabilities/CAP_TEST`);
  await expect(page.getByText('Linked policy rules temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText(/No rules reference this capability/)).toHaveCount(0);
});

test('node 404 is rendered as genuine not-found, not unavailable', async ({ page }) => {
  await page.route('**/api/v1/inventory/clusters/cluster-a/nodes/missing-node?*', route =>
    route.fulfill({ status: 404, json: { error: 'Node not found' } }),
  );

  await page.goto(`${fixture}?path=/clusters/cluster-a/nodes/missing-node`);
  await expect(page.getByText('Node not found', { exact: true })).toBeVisible();
  await expect(page.getByText('Node detail temporarily unavailable', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Retry node', exact: true })).toHaveCount(0);
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


const sbomSummary = (clusterId: string, podName: string) => ({
  clusterId,
  podId: 'dup-pod',
  podName,
  namespace: 'default',
  image: `${clusterId}:latest`,
  lastScan: '2026-09-24T00:00:00Z',
  activePod: true,
  lifecycleState: 'current',
  vulnerabilitySummary: { critical: 0, high: 0, medium: 0, low: 0 },
});

const sbomDetail = (clusterId: string, packageName: string) => ({
  clusterId,
  podId: 'dup-pod',
  podName: `${clusterId}-pod`,
  namespace: 'default',
  image: `${clusterId}:latest`,
  container: 'app',
  generatedAt: '2026-09-24T00:00:00Z',
  packageCount: 1,
  vulnerablePackageCount: 0,
  vulnerabilitySummary: { critical: 0, high: 0, medium: 0, low: 0 },
  activePod: true,
  lifecycleState: 'current',
  components: [{
    id: 1,
    name: packageName,
    version: '1.0.0',
    type: 'library',
    vulnerabilities: [],
    cveCount: 0,
    maxSeverity: '',
    maxCvss: 0,
    fixVersion: '',
  }],
});

const cleanThreatSummary = (clusterId: string) => ({
  clusterId,
  podUid: 'dup-pod',
  totalThreats: 0,
  malwareCount: 0,
  telemetryCount: 0,
  protestwareCount: 0,
  highestSeverity: '',
  affectedPackages: [],
  requiresAction: false,
});

test('SBOM list refresh 503 preserves last-known-good inventory instead of rendering empty', async ({ page }) => {
  let listAttempts = 0;
  await page.route('**/api/v1/**', route => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/v1/inventory/sbom') {
      listAttempts++;
      if (listAttempts === 1) {
        return route.fulfill({ json: { sboms: [sbomSummary('cluster-a', 'pod-a')], total: 1, limit: 100, offset: 0 } });
      }
      return route.fulfill(unavailable('sbom_list_query_unavailable', 'SBOM inventory could not be loaded'));
    }
    if (url.pathname === '/api/v1/inventory/pods/dup-pod/sbom') {
      return route.fulfill({ json: sbomDetail(url.searchParams.get('clusterId') || 'cluster-a', 'pkg-a') });
    }
    if (url.pathname === '/api/v1/malware/threats/dup-pod') {
      return route.fulfill({ json: cleanThreatSummary(url.searchParams.get('clusterId') || 'cluster-a') });
    }
    return route.fulfill({ status: 404, json: { error: 'not found' } });
  });

  await page.goto(`${fixture}?path=/sbom`);
  await expect(page.getByText('pod-a', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByText('pod-a', { exact: true })).toBeVisible();
  await expect(page.getByText(/No pods with SBOM data yet/)).toHaveCount(0);
  await expect(page.getByText(/SBOM inventory could not be loaded/)).toBeVisible();
});

test('SBOM duplicate Pod UID uses cluster-qualified selection and detail requests', async ({ page }) => {
  const detailClusters: string[] = [];
  await page.route('**/api/v1/**', route => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/v1/inventory/sbom') {
      return route.fulfill({
        json: {
          sboms: [sbomSummary('cluster-a', 'pod-a'), sbomSummary('cluster-b', 'pod-b')],
          total: 2,
          limit: 100,
          offset: 0,
        },
      });
    }
    if (url.pathname === '/api/v1/inventory/pods/dup-pod/sbom') {
      const clusterId = url.searchParams.get('clusterId') || '';
      detailClusters.push(clusterId);
      return route.fulfill({ json: sbomDetail(clusterId, clusterId === 'cluster-b' ? 'pkg-b' : 'pkg-a') });
    }
    if (url.pathname === '/api/v1/malware/threats/dup-pod') {
      const clusterId = url.searchParams.get('clusterId') || '';
      return route.fulfill({ json: cleanThreatSummary(clusterId) });
    }
    return route.fulfill({ status: 404, json: { error: 'not found' } });
  });

  await page.goto(`${fixture}?path=/sbom`);
  await expect(page.getByText('pkg-a', { exact: true })).toBeVisible();
  await page.getByText('pod-b', { exact: true }).click();
  await expect(page.getByText('pkg-b', { exact: true })).toBeVisible();
  expect(detailClusters).toContain('cluster-a');
  expect(detailClusters).toContain('cluster-b');
});

test('SBOM detail 503 is unavailable and is never projected as zero components', async ({ page }) => {
  await page.route('**/api/v1/**', route => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/v1/inventory/sbom') {
      return route.fulfill({ json: { sboms: [sbomSummary('cluster-a', 'pod-a')], total: 1, limit: 100, offset: 0 } });
    }
    if (url.pathname === '/api/v1/inventory/pods/dup-pod/sbom') {
      return route.fulfill(unavailable('pod_sbom_components_unavailable', 'Pod SBOM components could not be loaded'));
    }
    if (url.pathname === '/api/v1/malware/threats/dup-pod') {
      return route.fulfill({ json: cleanThreatSummary('cluster-a') });
    }
    return route.fulfill({ status: 404, json: { error: 'not found' } });
  });

  await page.goto(`${fixture}?path=/sbom`);
  await expect(page.getByText('SBOM detail temporarily unavailable', { exact: true })).toBeVisible();
  await expect(page.getByText('Components').locator('..')).toContainText('—');
  await expect(page.getByText(/No components found matching your search/)).toHaveCount(0);
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
    return route.fulfill({ json: { sinceMinutes: 60, emittedEvents: 0, uniqueKeys: 0, maxRatio: 0, perKey: {} } });
  }
  if (path.includes('/runtime/pods/pod-a/signals')) return route.fulfill({ json: { podUid: 'pod-a', signals: [], count: 0 } });
  if (path.includes('/runtime/pods/pod-a/facts')) return route.fulfill({ json: { podUid: 'pod-a', facts: [], total: 0 } });
  if (path.includes('/runtime/pods/pod-a/incidents')) return route.fulfill({ json: { podUid: 'pod-a', incidents: [], total: 0 } });
  if (path.includes('/runtime/pods/pod-a/network/top-destinations')) {
    return route.fulfill({ json: { podUid: 'pod-a', sinceMinutes: 1440, items: [] } });
  }
  if (path.includes('/runtime/pods/pod-a/metrics') ||
      path.includes('/runtime/pods/pod-a/processes') ||
      path === '/api/v1/runtime/pods/pod-a/network' ||
      path.includes('/runtime/pods/pod-a/events')) {
    return route.fulfill({ json: { podUid: 'pod-a', items: [] } });
  }
  if (path.includes('/risk/pods/pod-a/runtime/events')) return route.fulfill({ json: { podUid: 'pod-a', events: [], total: 0 } });
  if (path.includes('/runtime/pods/pod-a/capabilities')) return route.fulfill({ json: { podUid: 'pod-a', capabilities: [] } });
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

test('pod SBOM 404 is authoritative and never falls back to the unqualified SBOM list', async ({ page }) => {
  let sbomListCalls = 0;
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') return route.fulfill({ json: pod });
    if (path === '/api/v1/inventory/pods/pod-a/sbom') {
      return route.fulfill({ status: 404, json: { error: 'sbom not found for pod' } });
    }
    if (path === '/api/v1/inventory/sbom') {
      sbomListCalls++;
      return route.fulfill({
        json: {
          sboms: [{
            clusterId: 'cluster-b',
            podId: 'pod-b',
            podName: 'pod-a',
            namespace: 'default',
            image: 'foreign:latest',
            lastScan: '2026-09-24T00:00:00Z',
            activePod: true,
          }],
        },
      });
    }
    return fulfillPodSupportingApis(route);
  });

  await page.goto(`${fixture}?path=/resources/pods/uid/pod-a`);
  await expect(page.getByText('pod-a', { exact: true }).first()).toBeVisible();
  await expect.poll(() => sbomListCalls).toBe(0);
  await expect(page.getByText('foreign:latest', { exact: true })).toHaveCount(0);
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


test('pod route change never reuses last-known-good data from another pod', async ({ page }) => {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') {
      return route.fulfill({ json: pod });
    }
    if (path === '/api/v1/inventory/pods/pod-b') {
      return route.fulfill(unavailable('pod_detail_unavailable', 'Pod B inventory query failed'));
    }
    return fulfillPodSupportingApis(route);
  });

  await page.goto(`${fixture}?path=/resources/pods/uid/pod-a`);
  await expect(page.getByText('10.0.0.10', { exact: true })).toBeVisible();

  await page.evaluate(() => {
    const target = window as typeof window & { __availabilityNavigate?: (to: string) => void };
    target.__availabilityNavigate?.('/resources/pods/uid/pod-b');
  });

  await expect(page.getByRole('heading', { name: 'Could not load pod detail', exact: true })).toBeVisible();
  await expect(page.getByText('10.0.0.10', { exact: true })).toHaveCount(0);
});

test('malformed successful pod risk report is unavailable, not empty evidence', async ({ page }) => {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') {
      return route.fulfill({ json: pod });
    }
    if (path.endsWith('/report')) {
      return route.fulfill({ json: {
        podUid: 'pod-a',
        podName: 'pod-a',
        namespace: 'default',
        clusterId: 'cluster-a',
      } });
    }
    return fulfillPodSupportingApis(route);
  });

  await page.goto(`${fixture}?path=/resources/pods/uid/pod-a`);
  const failureSummary = page.getByText(/Failed to load:/);
  await expect(failureSummary).toBeVisible();
  await expect(failureSummary).toContainText('risk-report');
});

test('network connections remain usable when top-destination aggregation is unavailable', async ({ page }) => {
  await page.route('**/api/**', route => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    if (path === '/api/v1/inventory/pods/pod-a') return route.fulfill({ json: pod });
    if (path.includes('/runtime/pods/pod-a/network/top-destinations')) {
      return route.fulfill(unavailable('pod_network_top_destinations_unavailable', 'Top destinations query failed'));
    }
    if (path === '/api/v1/runtime/pods/pod-a/network') {
      return route.fulfill({ json: {
        podUid: 'pod-a',
        items: [{
          id: 1,
          sourceIp: '10.0.0.10',
          sourcePort: 45678,
          destIp: '10.0.0.20',
          destPort: 443,
          protocol: 'tcp',
          state: 'ESTABLISHED',
          observedAt: '2026-09-24T01:00:00Z',
        }],
      } });
    }
    return fulfillPodSupportingApis(route);
  });

  const detailPath = encodeURIComponent('/resources/pods/uid/pod-a?tab=network');
  await page.goto(`${fixture}?path=${detailPath}`);
  await expect(page.getByText('Observed sockets', { exact: true })).toBeVisible();
  await expect(page.getByText('1', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('Top remote endpoints are temporarily unavailable.', { exact: true })).toBeVisible();
  await expect(page.getByText('No network data', { exact: true })).toHaveCount(0);
});

test('runtime evidence identity mismatch is unavailable and never accepted as empty', async ({ page }) => {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') return route.fulfill({ json: pod });
    if (path.includes('/runtime/pods/pod-a/signals')) {
      return route.fulfill({ json: { podUid: 'pod-b', signals: [], count: 0 } });
    }
    return fulfillPodSupportingApis(route);
  });

  const detailPath = encodeURIComponent('/resources/pods/uid/pod-a?tab=events');
  await page.goto(`${fixture}?path=${detailPath}`);
  const failureSummary = page.getByText(/Failed to load:/);
  await expect(failureSummary).toBeVisible();
  await expect(failureSummary).toContainText('signals');
  await expect(page.getByText('No runtime signals.', { exact: true })).toHaveCount(0);
});

test('malformed suppression statistics are unavailable, not a successful zero state', async ({ page }) => {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') return route.fulfill({ json: pod });
    if (path.includes('/runtime/signals/suppression-stats')) {
      return route.fulfill({ json: { podUid: 'pod-a', suppressed: 0, total: 0 } });
    }
    return fulfillPodSupportingApis(route);
  });

  const detailPath = encodeURIComponent('/resources/pods/uid/pod-a?tab=events');
  await page.goto(`${fixture}?path=${detailPath}`);
  const failureSummary = page.getByText(/Failed to load:/);
  await expect(failureSummary).toBeVisible();
  await expect(failureSummary).toContainText('signal-stats');
  await expect(page.getByText(/Network anomaly events \(60m\):/)).toHaveCount(0);
});

test('malformed runtime security event payload is unavailable, not empty evidence', async ({ page }) => {
  await page.route('**/api/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/api/v1/inventory/pods/pod-a') return route.fulfill({ json: pod });
    if (path.includes('/risk/pods/pod-a/runtime/events')) {
      return route.fulfill({ json: { podUid: 'wrong-pod', events: [], total: 0 } });
    }
    return fulfillPodSupportingApis(route);
  });

  const detailPath = encodeURIComponent('/resources/pods/uid/pod-a?tab=events');
  await page.goto(`${fixture}?path=${detailPath}`);
  const failureSummary = page.getByText(/Failed to load:/);
  await expect(failureSummary).toBeVisible();
  await expect(failureSummary).toContainText('security-events');
  await expect(page.getByText('Security runtime events is temporarily unavailable.', { exact: false })).toBeVisible();
  await expect(page.getByText('No security runtime events', { exact: true })).toHaveCount(0);
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
  const failureSummary = page.getByText(/Failed to load:/);
  await expect(failureSummary).toBeVisible();
  await expect(failureSummary).toContainText('signals');
  await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible();
  await expect(page.getByText('Runtime signals is temporarily unavailable.', { exact: false })).toBeVisible();
  await expect(page.getByText('No runtime signals.', { exact: true })).toHaveCount(0);
  await expect(page.getByText(/No runtime signals in lookback window/)).toHaveCount(0);
});
