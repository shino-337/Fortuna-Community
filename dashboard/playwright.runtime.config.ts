import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/runtime-signals',
  timeout: 30_000,
  use: { baseURL: 'http://127.0.0.1:4175', browserName: 'chromium' },
  webServer: {
    command: 'npm run dev -- --host 127.0.0.1 --port 4175 --strictPort',
    url: 'http://127.0.0.1:4175/tests/runtime-signals/index.html',
    reuseExistingServer: !process.env.CI,
  },
});
