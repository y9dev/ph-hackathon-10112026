# CERTIFY run 1 (2026-11-10, fresh qa-engineer)

Commit under test: `f59fceb` (working tree: docs-only uncommitted edits in DEMO_SCRIPT.md, docs/ARCHITECTURE.md, docs/INTEGRATIONS.md, docs/UX_SPECIFICATION.md; no product code changes).
Running instance: Docker container `pavlodarhubhackathon1011-modus-voice-1` on http://localhost:8080 (DEMO_MODE, seeded).
Raw logs were kept in the certifier's temp directory; the excerpts below are copied from them.

## Level 1-2: static and unit/integration

| Command (cwd) | Exit | Result |
|---|---|---|
| `go vet ./...` (backend) | 0 | no output |
| `go test -count=1 ./...` (backend) | 0 | packages ok: cmd/server, internal/app, modules/identity, modules/insights, modules/requests, platform/migrate, platform/ratelimit; 41 top-level tests PASS (incl. TestAccessMatrix, TestAnonymity, TestAttachmentRejections, TestLifecycleTransitionsService, TestReportPermissionsAndStatuses, TestRoutingAndAdminRuleChange, TestSLAPauseAndOverdueTicker, TestCSVCellInjection, TestLifecycleMigrationFromAccepted). No tests in modules/notifications, modules/org, platform/httpx (covered indirectly by internal/app HTTP tests). |
| `gofmt -l backend` | 0 | no files listed |
| `npm run lint` (web; eslint + tsc -b) | 0 | no problems |
| `npm test` (web; Vitest) | 0 | Test Files 10 passed (10), Tests 72 passed (72) |
| `npm run build` (web) | 0 | 2282 modules transformed, built in 1.07s |
| `npm audit --omit=dev` (web) | 1 | 1 high: zod <4.6.6 GHSA-mrhx-c7xc-m3w5 (client-side form validation only; known, disclosed limitation) |
| `govulncheck` | NOT RUN | tool not installed |
| secret scan (`git grep` for key/token/password literals, AKIA/sk-/ghp_/PRIVATE KEY) | 0 hits | only intentional demo password `Demo1234!` (README, DEMO_SCRIPT, backend/README, seed/data.go), test password in `helpers_test.go`, mock password in `web/src/shared/api/mock/seed.ts`; `.env.example`/`.env.mock` contain no secrets |

## Level 3: E2E against the real backend (3 consecutive runs)

`E2E_BASE_URL=http://localhost:8080 npx playwright test -c playwright.real.config.ts e2e/journeys.spec.ts` (web)

| Run | Exit | Result |
|---|---|---|
| 1 | 0 | 6 passed (23.3s) |
| 2 | 0 | 6 passed (23.7s) |
| 3 | 0 | 6 passed (22.6s) |

Tests: J1 submit, J2 track, J3 lifecycle (take into work → decision → implementation → report → complete; author sees stepper, report, result), J5 close ("Закрыто"), J4 clarify, guards. Only the `real-desktop` project (1280x800) exists in this config: the `@mobile` tag on J1 is not exercised against the real backend.

## API smoke (curl, demo accounts)

Author a.petrova (employee), anonymous request in category 2 (IT):
- `POST /requests` → 201 `MV-0092`, routed `rule` to IT-разработка / Елена Кузнецова, first_response_due = created + 24 h.
- admin `GET /requests/92` → 200, `"author":{"name":"Автор (анонимно)"}`; grep for Петрова/petrova in detail, `scope=all` list, CSV export, and manager kuznetsova detail → 0 matches each.
- other employee i.sidorov `GET` → 404; out-of-scope manager s.volkov `GET` → 404; in-scope manager kuznetsova → 200.
- employee `POST /status` → 403; admin `new→in_review` → 200; `closed` without result → 400 `validation_failed` (result_text); `in_review→implemented` → 409; `report` comment → 201; `closed` with result → 200.
- author `GET` → status `closed`, result text and report present; author notifications include `status_changed` («Закрыто») and `report`.
- attachments: `evil.exe` → 400 «недопустимый тип»; HTML renamed `fake.png` → 400 «содержимое не соответствует расширению»; 11 MB `.txt` → 400 «больше 10 МБ».
- validation: short title/body → 400 with per-field messages.
- unrouted: category 9 («Другое», no rule) → 201 `routing_source: unrouted`, department/assignee null.
- unauthenticated list → 401; employee/manager `/admin/users` → 403; employee `/analytics/summary` → 403; POST with `Origin: https://evil.example` → 403.
- headers: CSP `default-src 'self'; script-src 'self'; frame-ancestors 'none' ...`, `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: same-origin`; session cookie `HttpOnly; SameSite=Lax` (no `Secure` over plain HTTP, by design).
- rate limit observed: after 3 E2E runs, i.sidorov `POST /requests` → 429 «Слишком много обращений за час» (RATE_CREATE_PER_HOUR=20). Correct behaviour; it means E2E cannot be run more than ~3 times per hour against one instance.
- analytics (admin): kpis total 93, reviewed_share 0.742, median first response 16.1 h, outcome_share 1, implemented_count 19, overdue 7, auto_routed 0.946; `tab=needs_action` list starts with overdue items (MV-0014, MV-0027, MV-0029, ...).

## Level 4: browser (Claude built-in browser pane)
- Login page with demo panel; one-click login as Кузнецова → `/inbox`, overdue rows first with «Просрочено на N дн.».
- `/analytics` at desktop: 7 KPI cards, period filter.
- 375x812: `/inbox` scrollWidth 375 = innerWidth (no horizontal overflow), card layout, bottom tab bar, FAB.
- 768x1024: `/requests/92` scrollWidth 753 < 768; stepper (Разработка · Принятие решения · Реализация · Закрыто), last report, result card, anonymous author in history.
- Console errors: one `401` on `/auth/me` from the pre-login session check (expected).

## Clean-checkout start
- `git clone` of the repo (HEAD f59fceb) into a temp dir → `docker build -t certify-modusvoice:clean .` → `BUILD_EXIT=0` (multi-stage: web `npm ci` + build, Go build).
- `docker run` with the docker-compose.yml environment (ADDR=:8080, DATA_DIR=/data, DEMO_MODE=true, SEED_DEMO=true, fresh named volume) on host port 18080 → `/healthz` 200 after 1 s, `/login` 200, `/api/v1/auth/demo-users` 200; log `overdue notifications emitted requests=7`.
- `docker compose up --build` itself was not run on port 8080 because the reviewed instance already occupies it; the image and environment are the same as the compose file.
- Against the fresh container: `E2E_BASE_URL=http://localhost:18080 npx playwright test -c playwright.real.config.ts e2e/journeys.spec.ts e2e/ux-audit.spec.ts` → 11 passed, 3 failed. Second run of `ux-audit.spec.ts` only → 6 passed, 2 failed:
  - `A3/A4 mobile` FAIL: `/analytics @375 horizontal overflow` received 721. Reproduced in the browser: after test A1 creates a theme with a 140-char title without spaces, `/analytics` (admin, 375 px) has `document.scrollWidth` 1096. The overflowing element is `<p class="text-sm font-medium text-text">` with the theme title (scrollWidth 1063). Product defect (missing wrapping/truncation of theme titles on analytics); the same title on the request detail page is handled.
  - `U11 mobile submit` FAIL: with the 9 real categories the title field is at y=1012 (test expects < 932). The test was tuned on mock data (fewer categories); on real data the first text field is below the fold on a 375x812 screen.
  - `A1 long theme` failed in the first run, passed in the second (flaky against shared real data).
- Mock suite `npx playwright test -c playwright.config.ts` (web) → exit 0, 15 passed, 3 skipped (screens, opt-in), including the mobile project J1.
- Throwaway container, volume, image and clone were removed afterwards.

## Side effects of this run
Created MV-0092 (closed), MV-0093 (unrouted) and the E2E requests from 3 runs in the demo database. Repeated E2E runs leave junk requests that form the top «suggested group» (16 requests, title «Вопрос про отпуск uhtar»). Reset before the demo: `docker compose down -v && docker compose up --build`.
