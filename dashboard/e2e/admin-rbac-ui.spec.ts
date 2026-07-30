import { expect, Page, test } from '@playwright/test';

const adminUsername = process.env.FORTUNA_E2E_ADMIN_USER ?? 'admin';
const adminPassword = process.env.FORTUNA_E2E_ADMIN_PASSWORD ?? 'Fortuna_DevOnly_P@ssw0rd';
const baseURL = process.env.FORTUNA_E2E_BASE_URL ?? 'http://127.0.0.1:31336';

function appUrl(_page: Page, hashPath: string): string {
  return `${String(baseURL).replace(/\/$/, '')}/#${hashPath.startsWith('/') ? hashPath : `/${hashPath}`}`;
}

async function loginAsAdmin(page: Page): Promise<string[]> {
  const consoleMessages: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error' || message.type() === 'warning') {
      consoleMessages.push(`${message.type()}: ${message.text()}`);
    }
  });

  let payload: { user?: unknown; token?: string } | null = null;
  let lastStatus = 0;
  for (let attempt = 0; attempt < 4; attempt += 1) {
    const response = await page.request.post(`${baseURL.replace(/\/$/, '')}/api/v1/auth/login`, {
      data: { username: adminUsername, password: adminPassword },
    });
    lastStatus = response.status();
    if (response.ok()) {
      payload = await response.json();
      break;
    }
    if (response.status() !== 429) break;
    await page.waitForTimeout(1000 * (attempt + 1));
  }
  expect(payload?.token, `admin API login failed with status ${lastStatus}`).toBeTruthy();
  await page.addInitScript((authPayload) => {
    window.sessionStorage.setItem(
      'fortuna-auth',
      JSON.stringify({
        state: {
          user: authPayload.user,
          token: authPayload.token,
          isAuthenticated: true,
        },
        version: 0,
      }),
    );
  }, payload);
  await page.goto(appUrl(page, '/dashboard'));
  await page.waitForLoadState('networkidle');

  await expect(page.locator('main h1').filter({ hasText: /^Dashboard$/ })).toBeVisible();
  return consoleMessages;
}

async function expectNoRedundantContextStrips(page: Page): Promise<void> {
  await expect(page.getByText(/^Experience:/)).toHaveCount(0);
  await expect(page.getByText(/^Data scope:/)).toHaveCount(0);
  await expect(page.locator('[aria-label="Operational context"]')).toHaveCount(0);
}

async function expectNoForbiddenShell(page: Page): Promise<void> {
  await expect(page.getByText(/Access denied/i)).toHaveCount(0);
  await expect(page.getByText(/Pod identifier missing/i)).toHaveCount(0);
  await expect(page.getByText(/Could not load pod detail/i)).toHaveCount(0);
}

async function expectNoShellPageTitle(page: Page): Promise<void> {
  await expect(page.locator('header h1, header h2')).toHaveCount(0);
}

async function getAdminToken(page: Page): Promise<string> {
  const authRaw = await page.evaluate(() => window.sessionStorage.getItem('fortuna-auth'));
  expect(authRaw).toBeTruthy();
  const auth = JSON.parse(authRaw || '{}') as { state?: { token?: string } };
  expect(auth.state?.token).toBeTruthy();
  return auth.state?.token || '';
}

async function apiGet<T>(page: Page, path: string, token: string): Promise<T> {
  const response = await page.request.get(`${baseURL.replace(/\/$/, '')}/api/v1${path}`, {
    headers: { authorization: `Bearer ${token}` },
  });
  expect(response.ok(), `${response.status()} ${path}`).toBeTruthy();
  return response.json() as Promise<T>;
}

function responseList<T>(payload: any, key: string): T[] {
  if (Array.isArray(payload)) return payload;
  if (Array.isArray(payload?.[key])) return payload[key];
  if (Array.isArray(payload?.data?.[key])) return payload.data[key];
  if (Array.isArray(payload?.data)) return payload.data;
  return [];
}

function attackPathSourceUid(path: any): string {
  return String(path?.nodes?.[0]?.id || path?.nodes?.[0]?.uid || path?.source_uid || '').toLowerCase();
}

async function openNetworkActivity(page: Page): Promise<void> {
  const networkResponse = page.waitForResponse(
    (response) => response.url().includes('/api/v1/runtime/network-activity') && response.status() === 200,
    { timeout: 15_000 },
  );
  await page.goto(appUrl(page, '/network-activity'));
  await networkResponse;
  await page.waitForLoadState('networkidle');
}

test.describe.configure({ mode: 'serial' });

test.describe('admin RBAC UI smoke', () => {
  test('top-level page titles match shell header and sidebar labels', async ({ page }) => {
    await loginAsAdmin(page);

    const routes = [
      ['/', 'Platform Integrity'],
      ['/dashboard', 'Dashboard'],
      ['/risks/findings', 'Risk Operations'],
      ['/investigation', 'Investigations'],
      ['/network-activity', 'Network Activity'],
      ['/attack-paths', 'Attack Analysis'],
      ['/rules', 'Policy Rules'],
      ['/clusters', 'Clusters'],
      ['/resources', 'Resources'],
      ['/capabilities', 'Capabilities'],
      ['/reports', 'Reports'],
      ['/monitoring', 'Monitoring'],
      ['/governance', 'Governance'],
      ['/settings', 'Settings'],
    ] as const;

    for (const [route, title] of routes) {
      await page.goto(appUrl(page, route));
      await page.waitForLoadState('networkidle');
      await expectNoShellPageTitle(page);
      await expect(page.locator('main h1').filter({ hasText: new RegExp(`^${title}$`) })).toBeVisible();
      await expect(page.locator('aside nav').getByText(title, { exact: true }).first()).toBeVisible();
    }
  });

  test('dashboard renders admin navigation and data widgets', async ({ page }) => {
    const dashboardStats = page.waitForResponse(
      (response) => response.url().includes('/api/v1/dashboard/stats') && response.status() === 200,
    );
    const consoleMessages = await loginAsAdmin(page);
    await dashboardStats;

    await expect(page.getByText('Resources')).toBeVisible();
    await expect(page.getByText('Network Activity')).toBeVisible();
    await expect(page.getByText('Settings')).toBeVisible();
    await expect(page.getByText(/Pods \(cluster\)/i).first()).toBeVisible();
    await expect(page.getByText(/Attack paths/i).first()).toBeVisible();
    await expectNoForbiddenShell(page);
    await expectNoRedundantContextStrips(page);

    const chartSizingWarnings = consoleMessages.filter((message) =>
      message.includes('The width(-1) and height(-1) of chart should be greater than 0'),
    );
    expect(chartSizingWarnings).toEqual([]);
  });

  test('resources opens pod detail route with Kubernetes pod UID', async ({ page }) => {
    await loginAsAdmin(page);

    const podsList = page.waitForResponse(
      (response) =>
        response.url().includes('/api/v1/inventory/pods') &&
        !/\/api\/v1\/inventory\/pods\/[0-9a-f-]{36}/.test(response.url()) &&
        response.status() === 200,
    );
    await page.goto(appUrl(page, '/resources'));
    await podsList;
    await expect(page.locator('main h1').filter({ hasText: /^Resources$/ })).toBeVisible();
    await expect(page.getByText(/Pods in scope/i)).toBeVisible();

    await page.getByRole('button', { name: /^Pod detail$/ }).first().click();
    await page.waitForURL(/#\/resources\/pods\/uid\/[0-9a-f-]{36}/, { timeout: 15_000 });

    const detailResponse = await page.waitForResponse(
      (response) => /\/api\/v1\/inventory\/pods\/[0-9a-f-]{36}$/.test(response.url()) && response.status() === 200,
      { timeout: 15_000 },
    );
    const detail = (await detailResponse.json()) as { uid?: string; name?: string };

    expect(page.url()).toContain(`/resources/pods/uid/${detail.uid}`);
    await expectNoShellPageTitle(page);
    await expect(page.getByRole('heading', { name: detail.name ?? '' })).toBeVisible();
    await expect(page.getByText(detail.uid ?? '')).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Overview' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Runtime' })).toBeVisible();
    await expect(page.getByRole('tab', { name: 'Network' })).toBeVisible();
    await expectNoForbiddenShell(page);
    await expectNoRedundantContextStrips(page);
    await expect(page.getByText(/Loading pod detail/i)).toHaveCount(0);
  });

  test('capability detail opens from capability metadata id', async ({ page }) => {
    await loginAsAdmin(page);
    const token = await getAdminToken(page);
    const metadataPayload = await apiGet<any>(page, '/capability-metadata?limit=1&offset=0', token);
    const metadata = responseList<any>(metadataPayload, 'metadata');
    expect(metadata.length).toBeGreaterThan(0);

    const capabilityId = String(metadata[0].capabilityId || metadata[0].capability_id || '');
    expect(capabilityId).toBeTruthy();

    const detailResponse = page.waitForResponse(
      (response) =>
        response.url().includes(`/api/v1/capability-metadata/${encodeURIComponent(capabilityId)}`) &&
        response.status() === 200,
      { timeout: 15_000 },
    );
    await page.goto(appUrl(page, `/capabilities/${encodeURIComponent(capabilityId)}`));
    await detailResponse;
    await page.waitForLoadState('networkidle');

    await expectNoShellPageTitle(page);
    await expect(page.getByText(capabilityId).first()).toBeVisible();
    await expect(page.getByText(/Loading capability/i)).toHaveCount(0);
    await expect(page.getByText(/Capability not found/i)).toHaveCount(0);
    await expectNoForbiddenShell(page);
    await expectNoRedundantContextStrips(page);
  });

  test('resources attack-path panel matches attack analysis pod scope', async ({ page }) => {
    await loginAsAdmin(page);
    const token = await getAdminToken(page);
    const podsPayload = await apiGet<any>(page, '/inventory/pods?page=1&pageSize=100&sortBy=risk_desc', token);
    const pods = responseList<any>(podsPayload, 'pods');
    expect(pods.length).toBeGreaterThan(0);

    const clusterId = pods[0].clusterId || pods[0].cluster_id || '';
    const clusterQuery = clusterId ? `?cluster=${encodeURIComponent(clusterId)}` : '';
    const bundlePayload = await apiGet<any>(page, `/graph/attack-paths/bundle${clusterQuery}`, token);
    const bundle = bundlePayload.data || bundlePayload;
    const bundlePaths = responseList<any>(bundle, 'paths').length
      ? responseList<any>(bundle, 'paths')
      : responseList<any>(bundle, 'primitive_paths');
    const summaryPayload = await apiGet<any>(page, `/graph/attack-paths/summary${clusterQuery}`, token);
    const summary = summaryPayload.data || summaryPayload;

    const candidate = pods
      .map((pod: any) => ({
        pod,
        sourcePaths: bundlePaths.filter((path: any) => attackPathSourceUid(path) === String(pod.uid).toLowerCase()),
      }))
      .find((item: any) => item.pod.uid && item.sourcePaths.length > 0);
    expect(candidate, 'expected at least one pod to be a source of an attack path').toBeTruthy();

    const endpointPayload = await apiGet<any>(
      page,
      `/graph/attack-paths/${encodeURIComponent(candidate.pod.uid)}`,
      token,
    );
    const endpointPaths = responseList<any>(endpointPayload, 'paths');
    const expectedPathIds = candidate.sourcePaths.map((path: any) => path.path_id || path.pathId).sort();
    const endpointPathIds = endpointPaths.map((path: any) => path.path_id || path.pathId).sort();
    expect(endpointPathIds).toEqual(expectedPathIds);

    const podsList = page.waitForResponse(
      (response) => response.url().includes('/api/v1/inventory/pods') && response.status() === 200,
      { timeout: 15_000 },
    );
    await page.goto(appUrl(page, '/resources'));
    await podsList;

    await page.getByPlaceholder('Search pods (name, namespace, UID, node)').fill(candidate.pod.name);
    await page.waitForResponse(
      (response) =>
        response.url().includes('/api/v1/inventory/pods') &&
        response.url().includes('search=') &&
        response.status() === 200,
      { timeout: 15_000 },
    );

    const row = page.getByRole('row').filter({ hasText: candidate.pod.name }).first();
    await expect(row).toBeVisible();
    await row.getByRole('button', { name: /show attack paths/i }).click();

    const inspector = page.locator('#resource-inspector').locator('..');
    await expect(inspector).toContainText(candidate.pod.name);
    await expect(inspector).toContainText('Source paths');
    await expect(inspector).toContainText(
      `${endpointPaths.length.toLocaleString('en-US')} / ${Number(summary.totalPaths ?? summary.total_paths ?? bundlePaths.length).toLocaleString('en-US')}`,
    );
    await expect(inspector).toContainText('Matches Attack Analysis pod scope');
    await expect(inspector).toContainText(expectedPathIds[0]);

    await inspector.getByRole('button', { name: /open same pod scope in attack analysis/i }).click();
    await page.waitForURL(new RegExp(`#\\/attack-paths\\?podUid=${candidate.pod.uid}`), { timeout: 15_000 });
    await expect(page.getByText(`podUid=${candidate.pod.uid}`)).toBeVisible();
    await expect(page.getByText(`${endpointPaths.length} path(s) returned for this workload.`)).toBeVisible();
    await expect(page.getByText(expectedPathIds[0]).first()).toBeVisible();
    await expectNoForbiddenShell(page);
    await expectNoRedundantContextStrips(page);
  });

  test('network activity is accessible and backed by runtime API data', async ({ page }) => {
    await loginAsAdmin(page);

    await openNetworkActivity(page);

    await expect(page.locator('main h1').filter({ hasText: /^Network Activity$/ })).toBeVisible();
    await expect(page.getByRole('img', { name: /Network topology graph/i })).toBeVisible();
    await expect(page.locator('main')).toContainText(/\d+\s+src\s+·\s+\d+\s+dest\s+·\s+\d+\s+edges/i);
    await expect(page.locator('main')).toContainText('Pod');
    await expect(page.locator('main')).toContainText('Internal');
    await expect(page.locator('main')).toContainText('External');
    await expect(page.locator('main')).toContainText('Flow count');
    await expect(page.locator('main')).toContainText(/Showing top .* entities|Not enough data to show topology/i);
    await expect(page.getByText(/destinations/i).first()).toBeVisible();
    await expectNoForbiddenShell(page);
    await expectNoRedundantContextStrips(page);
    await expect(page.getByText(/^Loading/i)).toHaveCount(0);
  });

  test('network activity filters follow desktop and mobile grid order', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await loginAsAdmin(page);
    await openNetworkActivity(page);

    const searchDesktop = await page.locator('#na-search-input').boundingBox();
    const namespaceDesktop = await page.locator('#na-namespace-input').boundingBox();
    const timeDesktop = await page.locator('#na-since-select').boundingBox();
    expect(searchDesktop).not.toBeNull();
    expect(namespaceDesktop).not.toBeNull();
    expect(timeDesktop).not.toBeNull();
    expect(searchDesktop!.x).toBeLessThan(namespaceDesktop!.x);
    expect(namespaceDesktop!.x).toBeLessThan(timeDesktop!.x);
    expect(Math.abs(searchDesktop!.y - namespaceDesktop!.y)).toBeLessThan(8);
    expect(Math.abs(namespaceDesktop!.y - timeDesktop!.y)).toBeLessThan(8);
    await expect(page.locator('#na-poduid-input')).toBeHidden();

    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload({ waitUntil: 'networkidle' });

    const searchMobile = await page.locator('#na-search-input').boundingBox();
    const namespaceMobile = await page.locator('#na-namespace-input').boundingBox();
    const timeMobile = await page.locator('#na-since-select').boundingBox();
    expect(searchMobile).not.toBeNull();
    expect(namespaceMobile).not.toBeNull();
    expect(timeMobile).not.toBeNull();
    expect(searchMobile!.y).toBeLessThan(namespaceMobile!.y);
    expect(namespaceMobile!.y).toBeLessThan(timeMobile!.y);
    await expect(page.getByText(/^Advanced$/)).toBeVisible();
    await expect(page.locator('#na-poduid-input')).toBeHidden();

    await page.getByText(/^Advanced$/).click();
    const podUidMobile = await page.locator('#na-poduid-input').boundingBox();
    expect(podUidMobile).not.toBeNull();
    expect(timeMobile!.y).toBeLessThan(podUidMobile!.y);
    await expect(page.getByText(/Numeric search terms filter by port/i)).toBeVisible();
  });
});
