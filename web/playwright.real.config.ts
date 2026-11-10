import { defineConfig, devices } from '@playwright/test';

// E2E against the real seeded backend (DEMO_MODE=true). Start it first, e.g.:
//   cd backend && DEMO_MODE=true go run ./cmd/server --seed
// The Go server serves the built SPA on :8080 (run `npm run build` before), or
// point E2E_BASE_URL at the Vite dev server (:5173, proxies /api to :8080).
const baseURL = process.env.E2E_BASE_URL ?? 'http://localhost:8080';

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list']],
  use: { baseURL, locale: 'ru-RU', timezoneId: 'Asia/Almaty', trace: 'retain-on-failure' },
  projects: [{ name: 'real-desktop', use: { ...devices['Desktop Chrome'], viewport: { width: 1280, height: 800 } } }],
});
