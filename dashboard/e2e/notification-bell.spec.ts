import { expect, test } from '@playwright/test';

const baseURL = process.env.FORTUNA_E2E_BASE_URL ?? 'http://127.0.0.1:31336';

function appUrl(hashPath: string): string {
  return `${String(baseURL).replace(/\/$/, '')}/#${hashPath.startsWith('/') ? hashPath : `/${hashPath}`}`;
}

const adminUser = {
  id: '1',
  username: 'admin',
  role: 'admin',
  operationalScope: {
    clusters: [],
    namespaces: [],
    teams: [],
    environments: [],
    restricted: false,
  },
};

test('notification bell opens above page content and supports read actions', async ({ page }) => {
  let postReadAll = 0;
  let patchRead = 0;
  let riskSearch = '';

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());

    if (url.pathname.endsWith('/auth/me')) {
      await route.fulfill({ json: { user: adminUser } });
      return;
    }

    if (url.pathname.endsWith('/inventory/clusters') || url.pathname.endsWith('/inventory/clusters/stats')) {
      await route.fulfill({
        json: {
          clusters: [
            {
              id: 'cluster-a',
              name: 'cluster-a',
              status: 'connected',
              lastSync: new Date().toISOString(),
              podCount: 1,
              riskCount: 1,
            },
          ],
          total: 1,
        },
      });
      return;
    }

    if (url.pathname.endsWith('/notifications') && request.method() === 'GET') {
      await route.fulfill({
        json: {
          unreadCount: postReadAll > 0 ? 0 : patchRead > 0 ? 2 : 3,
          total: 3,
          notifications: [
            {
              id: 101,
              title: 'High finding: Privileged container',
              message: 'Pod fortuna/fortuna-core requires review.',
              severity: 'high',
              category: 'risk',
              source: 'risk-engine',
              route: '/risks/findings?search=fortuna-core',
              resourceName: 'fortuna/fortuna-core',
              timestamp: new Date().toISOString(),
              readAt: patchRead > 0 || postReadAll > 0 ? new Date().toISOString() : null,
            },
            {
              id: 102,
              title: 'Critical attack path detected',
              message: 'Path path-core-admin has risk 86 across 4 steps.',
              severity: 'critical',
              category: 'attack-path',
              source: 'attack-path',
              route: '/attack-paths?path=path-core-admin',
              resourceName: 'fortuna/fortuna-core',
              timestamp: new Date().toISOString(),
              readAt: patchRead > 0 || postReadAll > 0 ? new Date().toISOString() : null,
            },
            {
              id: 103,
              title: 'Critical CVE matched: CVE-2099-0001',
              message: 'openssl@1.0.0 in pod fortuna/fortuna-core.',
              severity: 'critical',
              category: 'cve',
              source: 'cve-matcher',
              route: '/resources?tab=Pod&search=fortuna-core',
              resourceName: 'fortuna/fortuna-core',
              timestamp: new Date().toISOString(),
              readAt: postReadAll > 0 ? new Date().toISOString() : null,
            },
          ],
        },
      });
      return;
    }

    if (url.pathname.endsWith('/risk/insights') && request.method() === 'GET') {
      riskSearch = url.searchParams.get('search') ?? '';
      await route.fulfill({
        json: {
          view: 'instance',
          total: 1,
          page: 1,
          pageSize: 20,
          insights: [
            {
              id: 501,
              title: 'Privileged container',
              description: 'Container runs with elevated privilege.',
              severity: 'high',
              status: 'active',
              insightType: 'misconfiguration',
              resourceType: 'pod',
              resourceUid: 'pod-core-uid',
              resourceName: 'fortuna-core',
              resourceNamespace: 'fortuna',
              detectedAt: new Date().toISOString(),
              finalScore: 76,
              finalLevel: 'high',
            },
          ],
        },
      });
      return;
    }

    if (url.pathname.endsWith('/risk/insights/summary') || url.pathname.endsWith('/risk/insights/summary/global')) {
      await route.fulfill({ json: { critical: 0, high: 1, medium: 0, low: 0, total: 1, new24h: 1 } });
      return;
    }

    if (url.pathname.endsWith('/risk/insights/summary/by-cluster')) {
      await route.fulfill({ json: { byCluster: [] } });
      return;
    }

    if (url.pathname.endsWith('/risk/histogram')) {
      await route.fulfill({ json: { bins: [], totalFindings: 1, averageScore: 76, p0Count: 1 } });
      return;
    }

    if (url.pathname.endsWith('/pod-capabilities/summary')) {
      await route.fulfill({ json: { namespaces: [], severities: [], total: 0 } });
      return;
    }

    if (url.pathname.endsWith('/runtime/signals')) {
      await route.fulfill({ json: { signals: [], count: 0, total: 0 } });
      return;
    }

    if (url.pathname.endsWith('/notifications/read-all') && request.method() === 'POST') {
      postReadAll += 1;
      await route.fulfill({ json: { updated: 3 } });
      return;
    }

    if (/\/notifications\/\d+\/read$/.test(url.pathname) && request.method() === 'PATCH') {
      patchRead += 1;
      await route.fulfill({ json: { updated: 1 } });
      return;
    }

    await route.fulfill({ status: 200, json: {} });
  });

  await page.addInitScript((user) => {
    window.sessionStorage.setItem(
      'fortuna-auth',
      JSON.stringify({
        state: { user, token: 'test-token', isAuthenticated: true },
        version: 0,
      }),
    );
  }, adminUser);

  await page.goto(appUrl('/notifications'));
  await expect(page.locator('main h1').filter({ hasText: /^Notifications$/ })).toBeVisible();

  const bell = page.getByRole('button', { name: /security notifications/i }).first();
  await expect(bell).toBeVisible();
  await bell.click();

  const popup = page.getByText('Security notifications').locator('xpath=ancestor::div[contains(@class,"absolute")][1]');
  await expect(popup).toBeVisible();
  await expect(popup.getByText('fortuna/fortuna-core').first()).toBeVisible();
  const popupBox = await popup.boundingBox();
  const headerBox = await page.locator('header').first().boundingBox();
  expect(popupBox?.y ?? 0).toBeGreaterThan(headerBox?.y ?? -1);
  expect(popupBox?.height ?? 0).toBeGreaterThan(120);

  await popup.getByRole('button', { name: /high finding: privileged container/i }).click();
  await expect(page).toHaveURL(/#\/risks\/findings\?search=fortuna-core/);
  await expect.poll(() => riskSearch).toBe('fortuna-core');
  const resourceLink = page.getByRole('link', { name: /fortuna\/fortuna-core/i }).first();
  await expect(resourceLink).toBeVisible();
  await resourceLink.click();
  await expect(page).toHaveURL(/#\/resources\/pods\/uid\/pod-core-uid/);
  expect(patchRead).toBe(1);

  await page.goto(appUrl('/notifications'));
  await expect(page.locator('main h1').filter({ hasText: /^Notifications$/ })).toBeVisible();
  await bell.click();
  await page.getByRole('button', { name: /read all/i }).click();
  await expect(page.getByText('No unread alerts')).toBeVisible();
  expect(postReadAll).toBe(1);
});
