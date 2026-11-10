# Quality scorecard

Product: Modus Voice (employee feedback & suggestions)   Mode: hackathon   Date: 2026-11-10   Certified by: fresh qa-engineer (CERTIFY)   Independence: full (separate agent, no implementation context)
Commit / build: `f59fceb` (uncommitted: docs-only edits in DEMO_SCRIPT.md, docs/ARCHITECTURE.md, docs/INTEGRATIONS.md, docs/UX_SPECIFICATION.md; this run also updated docs/TZ_COMPLIANCE.md and added docs/evidence/certify-1.md)
Evidence log: [docs/evidence/certify-1.md](evidence/certify-1.md)

## Blockers
| # | Blocker | Present? (yes/no) | Evidence |
|---|---------|-------------------|----------|
| 1 | Critical functional test failing | no | `go test -count=1 ./...` exit 0 (41 tests); `npm test` 72/72; real-backend `journeys.spec.ts` 6/6 in 3 consecutive runs (exit 0). The failing tests are `ux-audit.spec.ts` A3/A4 and U11 (layout checks, not critical ACs) |
| 2 | Critical user journey broken | no | J1 submit, J2 track, J3 lifecycle (3 stages + report + complete), J4 clarify, J5 close, guards: green 3x on :8080 and 1x on a fresh clean-build container; curl smoke of the same flow (certify-1 "API smoke") |
| 3 | Unresolved Critical UX finding | no | Browser pass at 375/768/1280 (certify-1 Level 4); no Critical found. Formal usability/UI/a11y audit reports are not in the repo (only test IDs in `web/e2e/ux-audit.spec.ts` and commit f59fceb) |
| 4 | Unresolved Critical (or unaccepted High) security finding | no (see note) | Authz, anonymity, CSRF, upload, headers verified (certify-1). `npm audit`: zod <4.6.6 GHSA-mrhx-c7xc-m3w5 rated **High by npm**. In this product zod only validates the user's own form input in their own browser (`web/src/features/*/schema.ts`), so the practical impact is self-DoS of one tab. Assessed Low, not a blocker. **No written user acceptance is on record**: get it or bump zod |
| 5 | App cannot start / deploy as required | no | Fresh `git clone` → `docker build` exit 0 → container with compose env: `/healthz` 200 in 1 s, seeded demo users served, E2E journeys pass against it. `docker compose up --build` itself not re-run on :8080 (port held by the instance under review) |
| 6 | Regression vs baseline (maintenance) | n/a | Greenfield (docs/WORKFLOW_STATE.md "Baseline: none") |
| 7 | Mandatory stage skipped without approval | no (indirect evidence) | Stage 9 (UI/UX/a11y audits): commit f59fceb "audit fixes" and `web/e2e/ux-audit.spec.ts` encode audit finding IDs (B1, A1, U4–U11, A3–A7, B3–B6). The audit reports are not committed. Screen-reader pass not done (disclosed in README:59) |

## Scores
| Category | Weight | Score (0-5) | Weighted | Evidence (tests, screenshots, reports, file:line) | Caps / notes |
|----------|--------|-------------|----------|---------------------------------------------------|--------------|
| Functional correctness | 25% | 4.5 | 1.125 | (1) real-backend E2E `web/e2e/journeys.spec.ts` 6/6 x3, exit 0; (2) Go `TestAccessMatrix`, `TestAnonymity`, `TestLifecycleTransitionsService`, `TestRoutingAndAdminRuleChange`, `TestSLAPauseAndOverdueTicker`, `TestAttachmentRejections` PASS; (3) curl smoke: AC-01..AC-11 behaviours (routing + SLA, unrouted, result required 400, invalid transition 409, anonymity masked in detail/list/CSV, cross-scope 404, attachment type/magic/size 400, notifications to author) | Not 5: AC-12 (contrast ≥4.5:1, keyboard) only partly verified; mobile journey runs only in mock mode (real config has no mobile project, `journeys.spec.ts:5` tag `@mobile` unused there) |
| UX / Usability | 25% | 4.0 | 1.000 | (1) E2E J1–J5 complete with short flows (J1: type → category → text → submit); (2) `ux-audit.spec.ts` B1 dialogs/focus return, U6 routing admin, B3–B6 landmarks pass on real backend; (3) browser: stepper, last report, result card, «Просрочено на N дн.», empty-state-free inbox at 375 (certify-1 Level 4) | Medium: on a 375x812 screen with the 9 real categories the first text field of «Новое обращение» is below the fold (U11 fails on real data); demo data gets polluted by E2E runs (top «suggested group» = 16 test requests). Screen-reader pass NOT RUN |
| Architecture / Maintainability | 20% | 4.0 | 0.800 | (1) Go tests on core rules: `TestTransitionTable`, `TestComputeSLA`, `TestResolveRouteFallback`, `TestAuthorView`, `TestLifecycleMigrationFromAccepted` PASS; (2) module layout matches ARCH §2 (`backend/internal/modules/{identity,org,requests,insights,notifications}`, routes registered per module `requests/http.go:298-308`); OpenAPI paths match the registered routes 1:1; (3) `go vet` 0, eslint+tsc 0 | Minor drift: ARCHITECTURE §2/§6/§7 describe endpoints and a webhook/outbox writer that do not exist (see doc inconsistencies); `modules/notifications`, `modules/org`, `platform/httpx` have no own tests (covered only via `internal/app` HTTP tests) |
| UI / Visual quality | 15% | 3.5 | 0.525 | (1) `ux-audit.spec.ts` A3/A4 FAIL on real backend: `/analytics @375 horizontal overflow` 721 px, reproduced in browser (theme title `<p class="text-sm font-medium text-text">`, scrollWidth 1063); (2) browser: 375 inbox scrollWidth = 375, 768 detail no overflow, 1280 inbox/analytics consistent tokens, dark theme; (3) screenshots `docs/evidence/screens/01..16-*-{1280,375}.png` | One broken viewport for long unbroken theme titles on /analytics (Medium). Contrast of all themes not measured (README:59). Minor: lone 7th KPI card on its own row at 1280 |
| Security / Reliability | 15% | 4.0 | 0.600 | (1) Go `TestAccessMatrix`, `TestAnonymity`, `TestLoginThrottlePerAccountAndIP`, `TestForwardedHeadersIgnoredWithoutTrustedProxy`, `TestCSVCellInjection`, `TestSniffAndUploadValidation`, `TestRateLimitsPerUser` PASS; (2) curl: 401 unauth, 403 role, 404 cross-scope, 403 cross-origin POST, CSP/nosniff/DENY/same-origin headers, HttpOnly SameSite=Lax cookie, 429 create rate limit; (3) secret scan: only intentional demo password | Open, all disclosed and Low in context: zod advisory (npm High, see blocker 4), notifications not re-scoped after reassignment, duplicate notifications, pseudonymous anonymity, throttle behind proxy. `govulncheck` NOT RUN |
| **Total** | 100% | | **4.05 / 5** | | threshold: 3.3 (hackathon) |

Weighted = (4.5x0.25 + 4.0x0.25 + 4.0x0.20 + 3.5x0.15 + 4.0x0.15) = 1.125 + 1.000 + 0.800 + 0.525 + 0.600 = **4.05**. No NOT RUN level caps apply: levels 1–4 were all executed; the NOT RUN items below are sub-checks.

## Verdict
**PASS WITH CONDITIONS**: no blockers and 4.05 ≥ 3.3. Conditions, each needing a fix or written user acceptance:
1. zod advisory GHSA-mrhx-c7xc-m3w5 (npm High, assessed Low): upgrade or accept in writing.
2. Fix wrapping of theme titles on /analytics (overflow at 375 px), or accept it.
3. Before the demo, reset the demo instance (`docker compose down -v && docker compose up --build`) so test requests do not show up as «Повторяющиеся проблемы».
4. Correct the false "webhook adapter (built)" claims in the docs (see list below) before jury review.

## Checks executed / NOT RUN
| Check | Run? | Result | Evidence |
|-------|------|--------|----------|
| go vet / gofmt | yes | 0 issues | certify-1 L1 |
| go test -count=1 ./... | yes | exit 0, 41 tests | certify-1 L2 |
| npm run lint (eslint + tsc) | yes | exit 0 | certify-1 L1 |
| npm test (Vitest) | yes | 72/72 | certify-1 L2 |
| npm run build | yes | exit 0 | certify-1 L1 |
| npm audit --omit=dev | yes | 1 high (zod) | certify-1 L1 |
| govulncheck | **NOT RUN** | tool not installed | n/a |
| Secret scan (git grep patterns) | yes | clean (demo password only) | certify-1 L1 |
| Real-backend E2E journeys x3 (:8080) | yes | 6/6 each, exit 0 | certify-1 L3 |
| Mock E2E (desktop + mobile project) | yes | 15 passed, 3 skipped | certify-1 clean start |
| ux-audit.spec.ts on real backend | yes | 2 fail (A3/A4 overflow, U11), A1 flaky | certify-1 clean start |
| API curl smoke (login, create, list, status, anonymity, cross-scope, uploads, CSRF, headers) | yes | all as specified | certify-1 API smoke |
| Clean clone → docker build → run | yes | build 0, healthz 200 | certify-1 clean start |
| `docker compose up --build` literally | **NOT RUN** | port 8080 held by the reviewed instance; same image/env tested on :18080 | — |
| Browser L4 (375/768/1280, console) | yes | one overflow defect; only console error is the expected pre-login 401 | certify-1 L4 |
| Screen-reader pass | **NOT RUN** | no tool; disclosed README:59 | — |
| Contrast measurement of all themes | **NOT RUN** | not measured by certifier; disclosed README:59 | — |
| Keyboard-only full journey | **NOT RUN** (partial) | focus return covered by ux-audit B1 only | — |
| Mobile journey against real backend | **NOT RUN** | real config has only `real-desktop` | `web/playwright.real.config.ts` |
| Pitch / presentation deliverable (TZ 4.1, 6) | **NOT RUN** | no pitch file found in repo | — |
| Performance / load | **NOT RUN** | out of scope for hackathon | — |

## README feature-status table (README.md:36-51), verified row by row
| Row | Verdict |
|---|---|
| Вход, роли, разграничение доступа | true: logins for all 5 demo users; 401/403/404 matrix (curl, TestAccessMatrix) |
| Подача: тип, категория, описание, решение, до 5 вложений | true: J1 E2E; upload checks; 5-file limit is covered by unit tests only (`TestSniffAndUploadValidation`), not re-run by curl |
| Псевдоанонимная подача | true: «Автор (анонимно)» for admin/manager in detail, list, CSV |
| Автомаршрутизация, сроки, очередь «не распределено» | true: rule routing + SLA, category 9 → `unrouted` |
| Статусы, обязательный результат, комментарии, уточнение, заметки | true: 400 without result, 409 bad transition, J4 clarify E2E; internal notes covered by tests only |
| Уведомления в системе, история | true: author notifications for report and «Закрыто»; timeline in UI |
| Поиск, фильтры, вкладки, CSV | true: `q=` search, `tab=needs_action`, CSV export 200 |
| Аналитика: 4 метрики ТЗ и др. | true: `/analytics/summary` kpis; UI cards |
| Повторяющиеся проблемы (эвристика) | true: `/analytics/suggested-groups`; quality degraded by E2E junk data on the shared instance |
| Администрирование | true: routing table UI (U6 test); users covered by `TestAdminUserManagement` |
| Интеграции — только проработка | true: only `lognotifier.go`, ports, stubs in `backend/internal/integrations` |
| Доставка событий наружу (outbox, webhook) — запланировано | true in README, **but contradicted by** docs/INTEGRATIONS.md:24, docs/PRD.md:38, docs/ARCHITECTURE.md:37 |
| Опросы/NPS — запланировано | true: ROADMAP.md present |

## Sponsor requirements (PRD §6a)
- Three stages Разработка → Принятие решения → Реализация: VERIFIED (E2E J3; stepper in the browser on MV-0092).
- Progress reports (`report`): VERIFIED (curl 201; author notified; shown as «Последний отчёт»).
- Status «Закрыто» (`closed`) from a non-terminal status, result required: VERIFIED (curl 400 without result, 200 with; E2E J5).

## Remaining limitations and deferred findings
- **M1 (UI, Medium)** /analytics overflows at 375 px when a theme title has a long unbroken word (`ux-audit.spec.ts` A3/A4 on real data).
- **M2 (UX, Medium)** «Новое обращение» at 375x812 with 9 categories: the first text field starts at y≈1012, below the fold (U11 on real data).
- **M3 (Demo hygiene, Medium)** E2E against the shared demo instance leaves junk requests that become the top suggested group. The create limit of 20/hour per user (429) also allows only about 3 E2E runs per hour per instance.
- **M4 (Docs, Medium)** False "webhook built" claims (see below), stale ARCHITECTURE API list.
- **L1** zod advisory (npm High, Low in context). **L2** E2E `ux-audit.spec.ts` depends on test order and shared data (A1 flaky; A1's theme breaks A3). **L3** Unknown `/api/*` paths outside `/api/v1` return the SPA (200 HTML), e.g. `/api/healthz`. **L4** Untracked Word lock file `spec/~$_на_разработку...docx`; `.gitignore` lacks `~$*`.
- Known and disclosed (not counted as undisclosed): notifications not re-scoped after reassignment, duplicate notifications, pseudonymous anonymity, no real integrations, login throttle behind a proxy, no screen-reader testing.

## Documentation inconsistencies found
- docs/INTEGRATIONS.md:24: "generic webhook adapter (built; points at any URL, HMAC-signed)". Not built (no adapter in `backend/internal/integrations/`; ARCH §11 R1 at docs/ARCHITECTURE.md:116 says not built; README:50 says planned).
- docs/PRD.md:38: "demo uses an in-app notifier and a webhook sink". No webhook sink exists.
- docs/ARCHITECTURE.md:37: "adapters: inapp, webhook (built)". Only `lognotifier.go` exists.
- docs/ARCHITECTURE.md:104: "outbox writer (same transaction → at-least-once webhook delivery)". Not built (R1).
- docs/ARCHITECTURE.md:92-96: `PATCH /requests/{id}/status`, `/duplicate`, `/similar-count`, `/analytics/timeseries|categories|themes` do not exist. Code and openapi use `POST /status`, `/theme`, `/analytics/suggested-groups`, `/themes`.
- docs/ARCHITECTURE.md:71: closing statuses list omits `closed`. docs/ARCHITECTURE.md:127 lists statuses without `closed` (added only at :129).
- docs/ARCHITECTURE.md:116: "Playwright covers J1–J4 smoke only". Outdated: J5 and ux-audit specs exist.
- backend/internal/integrations/ports.go:59 and backend/internal/integrations/bitrix24/doc.go:7: "on request.accepted". That status and event were removed by R10; docs/INTEGRATIONS.md:15 already says "transition to in_progress".
- docs/WORKFLOW_STATE.md:7, :30-35: still "Stage 6 started", "Open findings: none yet", "Next action: present scope". Stale.
- docs/evidence/web-frontend.md:8, :10: "36 tests", "6 passed". Now 72 unit tests and 15 mock E2E.
- README.md:26-32 account table omits «Игорь Громов (руководитель пусконаладки)», whom DEMO_SCRIPT.md:19 uses. He exists in the seed (`backend/internal/seed/data.go:38`) and in the demo panel.
- DEMO_SCRIPT.md:35: "4 предложенные группы". On the shared instance after E2E runs there are 6, including 2 from test data. On a fresh seed this was not verified.
- README.md:73-74 documents only the mock E2E (`npm run e2e`). The real-backend command (`npm run e2e:real`) is only in web/README / the config comment.
- docs/TZ_COMPLIANCE.md was entirely ⏳ before this run. Statuses are now updated with evidence from certify-1.

## Addendum: after change requests CR1–CR6 (2026-11-11)

The 4.05/5 score above predates CR1–CR6 and was not re-certified by an independent QA pass. Evidence for the merged tree:
`go vet`, `go test` (incl. telegram, anonymity, observer rules), `npm run lint`, 89 unit tests, build, mock E2E (27 passed),
real-backend E2E on the Docker stack (23 passed, 7 skipped: screenshots and mock-only Telegram checks).
Not verified: real Telegram token, QR scan on a phone, screen reader, contrast of new screens, fresh UX/UI audit of the token,
Telegram and theme-dialog screens.
Known open points: `/themes` is company-wide for managers while linking to a foreign theme returns 404; zod advisory not accepted yet.
