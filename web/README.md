# Modus Voice — web UI

React 19 + TypeScript (strict) + Vite, Tailwind CSS v4 (design tokens from `docs/UX_SPECIFICATION.md` §5),
Radix UI primitives wrapped in `src/shared/ui`, TanStack Query, React Router, react-hook-form + zod,
lucide-react, Inter (self-hosted, cyrillic). Charts are small hand-made SVG components (no chart library).
UI language: Russian; every string lives in `src/shared/i18n/ru.ts`.

## Run

```bash
cd web
npm install

# 1) Against the in-browser mock API (no backend needed)
npm run dev:mock            # http://localhost:5173  (VITE_MOCK=1 via .env.mock)

# 2) Against the real backend (proxies /api -> http://localhost:8080)
#    cd backend && DEMO_MODE=true go run ./cmd/server --seed
npm run dev                 # http://localhost:5173
```

Demo accounts appear on the login page when the backend runs with `DEMO_MODE=true`
(`GET /api/v1/auth/demo-users`; the panel hides itself on 404). In mock mode the password for all demo
accounts is `demo12345`.

## Scripts

| Command | What it does |
|---|---|
| `npm run build` | type-check + production build into `../backend/internal/webui/dist` (embedded by Go). `scripts/clean-dist.mjs` removes the old build but keeps `.gitkeep`; Vite runs with `emptyOutDir: false`. |
| `npm run lint` | ESLint (incl. react-hooks rules) + `tsc -b` |
| `npm test` | Vitest: formatting, SLA, filters⇄URL, form schemas, file checks, mock contract tests, FileDrop component |
| `npm run e2e` | Playwright journeys J1–J4 + guards against the mock (starts Vite on :5174), desktop 1280 and Pixel 7 |
| `npm run e2e:real` | Same journeys against a running seeded backend (`E2E_BASE_URL`, default `http://localhost:8080`) |
| `SCREENS=1 npx playwright test e2e/screens.spec.ts --project=desktop` | Screenshots of all main screens at 1280 and 375 px into `docs/evidence/screens/` |

## Mock mode

`src/shared/api/mock/` is a fetch interceptor installed only when `VITE_MOCK=1` (it is code-split and never
loaded otherwise). It implements `docs/api/openapi.yaml` exactly: error envelope, scopes and tabs with counters,
the AuthorView masking for anonymous requests, `allowed_statuses` / `allowed_comment_kinds` (transition table
from ARCHITECTURE §11 R4), SLA states incl. pause in `needs_info`, routing rules, similarity groups,
notifications and admin CRUD with 409s. State lives in `sessionStorage` (survives reloads, resets in a new tab);
`window.__mvMockReset()` restores the seed. `?mockLatency=0` in the URL removes the artificial delay.
`handlers.test.ts` pins the contract-critical behaviour so the UI is never tested against a wrong mock.

## Structure

```
src/
  app/            App (providers, router, guards), AppShell (sidebar / top bar / mobile tab bar), routes.tsx
  features/
    auth/          AuthProvider (session, 401 → /login), LoginPage with demo accounts
    submit/        Новое обращение: schema, draft autosave, FileDrop, routing preview, success page
    my-requests/   Мои обращения
    inbox/         Входящие: tabs + counts, filters ⇄ URL, table → cards, CSV export
    request-detail/ Карточка: timeline, composer, staff actions + dialogs
    analytics/     KPI tiles, charts, suggested groups → themes, overdue list
    admin/         Маршрутизация · Категории · Пользователи
    notifications/ bell popover (polls every 30 s) and full page
  shared/
    api/          client.ts (typed fetch, ApiError), types.ts (mirror of openapi), queries.ts, mock/
    ui/           design-system components (Button, FormField, Select, Dialog, Tabs, Toast, chips, …)
    lib/          format, sla, status, inboxFilters, theme, download, hooks
    i18n/ru.ts    all user-facing strings
  styles/         tokens.css (light + dark), index.css (Tailwind theme mapping)
e2e/              Playwright specs and helpers
```

## Adding a feature

1. Create `src/features/<name>/<Name>Page.tsx` (default export).
2. Add one entry to `appRoutes` in `src/app/routes.tsx`: path, lazy component, allowed roles and, if it needs a
   menu item, `nav` (label, icon, mobile roles, order). The sidebar, bottom bar and route guard pick it up.
3. Put strings in `src/shared/i18n/ru.ts`, API calls in `src/shared/api/client.ts` (+ types in `types.ts`, + a
   mock handler if the endpoint is new).

## Conventions

- Colours, radii, shadows only through tokens (`bg-surface`, `text-muted`, `border-border`, `--status-*`).
  Brand teal is reserved for primary actions and active states; status colours only for status.
- Spacing on the 4-pt scale (Tailwind 1, 2, 3, 4, 6, 8, 12). Type scale 12/14/16/20/24/32.
- Every data view handles loading (skeletons), empty, error (retry) and 401/403/404.
- Status and SLA are always icon + text + colour. Motion is 150–200 ms and disabled by `prefers-reduced-motion`.
- Theme: system by default; a manual choice (user menu) is stored in `localStorage` (`mv-theme`).
