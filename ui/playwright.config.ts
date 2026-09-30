import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 180_000,
  expect: { timeout: 30_000 },
  fullyParallel: false,
  workers: 1,
  use: { baseURL: 'http://127.0.0.1:8799', trace: 'retain-on-failure', locale: 'tr-TR' },
  webServer: {
    // Built first so the startup timeout does not include compilation. The stale
    // fleet description is removed so a failed start cannot leave old data behind.
    command: 'rm -f e2e/.bin/fleet.json && go build -o e2e/.bin/fakefleet ../test/e2e/fakefleet && exec e2e/.bin/fakefleet -port 8799 -info e2e/.bin/fleet.json',
    url: 'http://127.0.0.1:8799/api/workspace',
    reuseExistingServer: false,
    // Let fakefleet run its cleanup (temp dir with fixture passwords) instead of being SIGKILLed.
    gracefulShutdown: { signal: 'SIGTERM', timeout: 5000 },
    timeout: 180_000,
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
})
