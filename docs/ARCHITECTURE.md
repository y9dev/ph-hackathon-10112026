# Architecture — Modus Voice

## 1. Stack (and why)
| Layer | Choice | Reason |
|---|---|---|
| Backend | **Go 1.26**, `net/http` + `chi` router, `database/sql` | Required by user. Stdlib-first, few deps, single static binary. |
| DB | **SQLite** (pure-Go `modernc.org/sqlite`, WAL) behind per-module store interfaces | Zero setup for jury (`go run`/one binary/one container), no CGO. Schema is portable SQL; PostgreSQL swap = new store impls, documented in §9. |
| Auth | Server-side sessions in DB, httpOnly `SameSite=Lax` cookie, bcrypt | Simple, revocable, no JWT pitfalls. CSRF: SameSite + JSON-only + Origin check. |
| Frontend | **React 19 + TypeScript + Vite**, **Tailwind CSS v4** + **Radix UI** primitives (shadcn-style local components), **TanStack Query**, React Router, react-hook-form + zod, lucide-react, Recharts | Best fit for an interaction-heavy internal tool built in 48 h: fastest UI iteration, accessible primitives (dialog, select, tabs, toast) out of the box, typed data layer, rich ecosystem, easy to extend by any web team. |
| Delivery | Go embeds the built SPA (`go:embed`); one binary serves `/api/*` and the UI. Dockerfile (multi-stage) + `Makefile` + `docker-compose.yml`. | One-command demo; same origin → no CORS. |
| Contract | `docs/api/openapi.yaml` (source of truth for external integrators) + typed TS client | Integration-friendly. |
| Tests | Go `testing`+`httptest` (service + HTTP), Vitest (UI logic), Playwright (E2E journeys) | |

Rejected: microservices/queues/Redis (no need at this scale), ORM (hidden SQL, extra dep), Next.js (SSR unnecessary behind login), PostgreSQL as the only DB (adds setup friction to a jury demo; kept as documented migration).

## 2. Repository layout (extension-oriented)
```
/
├─ README.md                 run/demo instructions (RU), limits
├─ Makefile                  dev, build, test, seed, e2e
├─ Dockerfile, docker-compose.yml
├─ docs/                     PRD, ARCHITECTURE, UX_SPECIFICATION, INTEGRATIONS, TZ_COMPLIANCE, QUALITY_SCORECARD, api/openapi.yaml
├─ spec/                     original assignment (read-only)
├─ backend/
│  ├─ go.mod
│  ├─ cmd/server/main.go     entrypoint: config → app.New → run (flags: --seed)
│  ├─ migrations/*.sql       ordered, embedded, tiny in-repo runner
│  └─ internal/
│     ├─ app/                composition root: config.go, app.go, modules.go (the ONE place modules are registered)
│     ├─ platform/           shared infra, no business rules: db, migrate, httpx (json, errors, middleware), eventbus, clock, ids, logger
│     ├─ modules/            one folder per bounded context, identical shape:
│     │  ├─ identity/        users, sessions, RBAC, login
│     │  ├─ org/             departments, categories, routing rules
│     │  ├─ requests/        core: requests, workflow, comments, history, attachments, routing engine, SLA
│     │  ├─ insights/        similarity/themes + analytics queries
│     │  └─ notifications/   in-app notifications + subscriber to domain events
│     ├─ integrations/       PORTS (interfaces) + adapters: inapp, webhook (built); bitrix24, onec, email (documented stubs)
│     └─ seed/               synthetic demo data
│     └─ webui/              embed.go (+ dist/ built SPA)
│       module shape: domain.go (types+rules) · service.go (use cases) · store.go (interface) · store_sqlite.go · http.go (handlers) · module.go (Register(deps) → routes, subscriptions) · *_test.go
└─ web/
   ├─ src/app/               router, providers, layout shell
   ├─ src/shared/            ui/ (design-system components), api/ (client + types), lib/, i18n/ (ru strings)
   ├─ src/features/          auth, submit, my-requests, inbox (review), request-detail, analytics, admin, notifications
   └─ e2e/                   Playwright specs
```
**Extension rules:** a new capability = new folder under `modules/` (backend) and `features/` (frontend) + one line in `app/modules.go` / router. Modules talk through (a) service interfaces injected at composition and (b) domain events on the in-process bus. No module imports another module's store.

## 3. Domain model (SQLite)
```
departments(id, name, head_user_id)
users(id, email UNIQUE, name, password_hash, role[employee|manager|admin], department_id, active, created_at)
sessions(token_hash PK, user_id, expires_at, created_at)
categories(id, name, description, icon, sort, active)
routing_rules(id, category_id, request_type NULL, department_id, assignee_user_id, first_response_hours, resolution_hours, active)
themes(id, title, created_at)
requests(id, number UNIQUE 'MV-0001', type[problem|idea], category_id, title, body, proposed_solution,
         author_id, is_anonymous, department_id NULL, assignee_id NULL, status,
         routing_source[rule|manual|unrouted], first_response_due, resolution_due, first_responded_at NULL,
         closed_at NULL, result_text NULL, duplicate_of NULL, theme_id NULL, search_tokens, created_at, updated_at)
comments(id, request_id, author_id, kind[comment|question|answer|internal], body, created_at)
request_events(id, request_id, actor_id NULL, kind, payload_json, created_at)   -- immutable audit/timeline
attachments(id, request_id, comment_id NULL, filename, stored_name, mime, size, uploaded_by, created_at)
notifications(id, user_id, kind, request_id, title, body, read_at NULL, created_at)
outbox(id, topic, payload_json, status[pending|sent|failed], attempts, next_attempt_at, created_at)
```
Indexes: requests(status, assignee_id), (department_id, status), (author_id), (first_response_due), (theme_id), (created_at); notifications(user_id, read_at).

## 4. Workflow
Statuses: `new → in_review ⇄ needs_info → accepted → implemented`; terminal: `implemented`, `declined`, `duplicate`.
Allowed transitions enforced in `requests/domain.go` (table-driven, unit-tested). Closing (`implemented|declined|duplicate`) requires `result_text` (duplicate requires `duplicate_of`). Author's answer to a `question` auto-moves `needs_info → in_review`. `first_responded_at` = first manager action (comment, question or status change) by someone other than the author.
**Routing engine** (on create, one transaction): find active rule by (category, type) → else (category) → else unrouted. Sets department, assignee, deadlines; unrouted ⇒ admin queue, `routing_source=unrouted`. Manual reassign writes an event.
**SLA/overdue:** computed at read time from deadlines (`now > due && !first_responded_at`); a 1-minute ticker in-process emits `request.overdue` once per request (idempotent via event table) for notification/escalation.

## 5. Access matrix
| Action | Employee | Manager | Admin |
|---|---|---|---|
| Create request | ✔ | ✔ | ✔ |
| Read request | own | routed to a department they head, or assigned to them | all |
| See author identity | n/a (own) | only if not anonymous | only if not anonymous |
| Comment / answer | own request | in scope (internal notes ✔) | ✔ |
| Change status / assign / merge | ✘ | in scope | ✔ |
| Analytics | ✘ | own scope | all |
| Categories / rules / users | ✘ | ✘ | ✔ |
Enforced in service layer (not handlers); every service method takes an `Actor`. Authorization tests per cell of this matrix.
Anonymity: `requests.author_id` is internal; DTO mappers drop `author` for non-authors when `is_anonymous`; only the author's own list endpoint resolves ownership. Tested.

## 6. API (v1, JSON, cookie session) — full schema in `docs/api/openapi.yaml`
```
POST /api/v1/auth/login · POST /logout · GET /me
GET  /meta                          categories, departments, enums (for forms)
POST /requests (multipart: json+files)   GET /requests?scope=mine|inbox|all&status&category&q&overdue&page
GET  /requests/{id}    PATCH /requests/{id}/status   POST /requests/{id}/assign   POST /requests/{id}/duplicate
GET/POST /requests/{id}/comments    GET /attachments/{id}
GET  /requests/{id}/similar-count   (aggregate only for authors; full list for managers)
GET  /analytics/summary|timeseries|categories|themes
GET/POST/PATCH /admin/categories, /admin/routing-rules, /admin/users, /admin/departments
GET  /notifications   POST /notifications/read
GET  /healthz
```
Errors: `{ "error": { "code": "...", "message": "ru text", "fields": {...} } }`. Pagination: `page`,`page_size`. All inputs validated server-side (length, enum, size, file allowlist png/jpg/pdf/docx/xlsx/txt, ≤10 MB total).

## 7. Events & integration seams
In-process bus publishes: `request.created|assigned|status_changed|commented|question_asked|answered|overdue|closed`. Subscribers: notifications (in-app), outbox writer (same transaction → at-least-once webhook delivery with retry/backoff). Ports in `integrations/`: `Notifier`, `DirectoryProvider`, `TaskSink`, `IdentityProvider`. See `INTEGRATIONS.md`.

## 8. Security basics (hackathon floor)
No secrets in repo (`.env.example`; secret generated at first run for cookies); bcrypt; login rate limit; per-request authorization in services; parameterized SQL only; upload allowlist + random stored names + `Content-Disposition: attachment` + size cap; security headers (CSP, X-Content-Type-Options, Referrer-Policy); demo-account quick login only when `DEMO_MODE=true`; output is JSON (React escapes HTML).

## 9. Scaling path (phased)
Phase 1 (this MVP): SQLite single node. Phase 2: PostgreSQL stores (same interfaces), SSO/Bitrix24 directory sync, e-mail, Bitrix24 tasks via outbox, object storage for files. Phase 3: customer surveys/NPS as new `modules/surveys` + public tokens; full-text search (Postgres FTS/Meili); LLM-assisted classification/clustering (needs separate privacy review).

## 11. Review resolutions (independent review, 2026-11-10; overrides earlier sections where they conflict)
- **R1 Scope/build order (F1):** slice 1 submit→route→inbox→status→close-with-result→my-requests+timeline+in-app notifications; slice 2 comments/clarification, attachments (≤5 files, 10 MB, magic-byte check), filters/search; slice 3 dashboard (4 TZ KPIs + by category) and suggested groups; slice 4 admin (rules/categories/users as compact tables), CSV export, polish. **Not built:** outbox dispatcher + webhook adapter (documented in INTEGRATIONS with port signatures; only the in-process event bus and the `outbox` table schema are kept). Hand-written short `openapi.yaml`. Playwright covers J1–J4 smoke only.
- **R2 Anonymity (F2):** it is *pseudonymous*, not anonymous: the DB keeps `author_id` so the author can track/answer; every outgoing view (DTOs, timeline, notifications, CSV export, event payloads) goes through one `AuthorView` mapper that replaces author and actor with "Автор (анонимно)". Automated test serializes every endpoint for an anonymous request and asserts no author id/name/email. UI states the limits (admin with DB access, content/attachments/timing may reveal identity; EXIF not stripped → warning). Similar-request lists for managers show only a score + title of requests already in their scope.
- **R3 SLA (F3):** two deadlines — first response and resolution (`resolution_due`); both flagged overdue; clock paused in `needs_info` (due dates shift by paused duration). If assignee inactive/unset → department head → unrouted (admin queue). Overdue notice: owner immediately, admins after +24 h.
- **R4 Transitions (F4):** `new→{in_review,needs_info,accepted,declined,duplicate}`, `in_review→{needs_info,accepted,declined,duplicate}`, `needs_info→in_review` (author answer, also for anonymous authors via own-request endpoint), `accepted→{implemented,declined}`; terminal: implemented/declined/duplicate; any terminal needs result text; no reopen in MVP (author may file a new request referencing the old number). Any staff action on `new` counts as first response.
- **R5 Data (F5):** drop `search_tokens` (compute similarity on the fly; scale is small); `theme_id` kept as manager-confirmed grouping, `duplicate_of` closes a request; UNIQUE(category_id, request_type) among active rules, type NULL = any; add `users.external_id`, `users.source`; `internal` comments never returned to authors (tested); request number from a counter row updated in the same transaction.
- **R6 Layout (F6):** notification creation is a synchronous interface call inside the request transaction (AC-08); the bus serves non-critical subscribers; routing/SLA in `requests/routing.go`, `requests/sla.go`; `insights` reads through a read-only query interface; `webui/` sits directly under `internal/`.
- **R7 Security (F7):** `Secure` cookie flag when HTTPS, `DEMO_MODE` default off (compose turns it on, UI banner), per-account login throttle, upload magic-byte sniffing.
- **R8 Themes (F9):** presented as "suggested groups" with similarity score; a manager confirms. Seed has deliberate clusters.
- **R9 Docs (F10):** add `ROADMAP.md`, `DEMO_SCRIPT.md` at the end.

## 10. Decisions (short ADRs)
- D1 SQLite first, portable SQL, store interfaces. · D2 Server-side sessions. · D3 Modular monolith with event bus, no network boundaries. · D4 Similarity = Jaccard over normalized tokens (RU stemming-lite: lowercase, strip punctuation, stop-words, 5-char prefix), no ML; threshold tuned on seed data. · D5 Anonymity by contract, tested. · D6 SPA embedded in Go binary. · D7 UI strings centralized (`shared/i18n/ru.ts`) for future languages.
