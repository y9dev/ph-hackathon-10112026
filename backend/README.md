# Modus Voice — backend

Go modular monolith: REST API under `/api/v1` (contract: `../docs/api/openapi.yaml`) plus the embedded SPA.
SQLite (pure Go, no CGO), embedded migrations, server-side sessions.

## Run

```sh
cd backend
DEMO_MODE=true go run ./cmd/server --seed          # fresh ./data: migrate, seed demo data, serve on :8080
go run ./cmd/server                 # serve existing data
DEMO_MODE=true go run ./cmd/server --seed-only     # seed (if empty) and exit
DEMO_MODE=true go run ./cmd/server  # login screen lists demo accounts
```

Open http://localhost:8080 (UI) or http://localhost:8080/healthz. Reset demo data: stop the server, delete `DATA_DIR`, run with `--seed`.

### Demo data (CR1: dataset only)

The seed (`internal/seed`) contains ONLY `docs/DATASET.md`: exactly the 20 requests A-001…A-020 with their original
texts (`MV-0001` = A-001 … `MV-0020` = A-020; titles are short headings derived from the first clause), types
`problem`/`idea`/`feedback`, the dataset priorities, the 18 dataset topic labels as categories, and the dataset's
conditional roles as users (display name = role title, no personal names). Dataset status mapping: Новое=`new`,
На рассмотрении=`in_review`, В работе=`in_progress`, Ожидает уточнения=`needs_info` (with a staff `question`),
Реализовано=`implemented`, Отклонено=`declined`, Закрыто=`closed`. «Пример дальнейшего действия» is an internal note
of the responsible; «Пример результата / мотивированного ответа» is the `result_text` (A-018 has only an action, so its
result text is that action in past tense). Only test dates are added (deterministic, relative to the seeding time):
created over ~58 days, first responses 3–61 h (two of them after the deadline), 4 overdue open requests (A-001, A-003,
A-007, A-020), closed ones with `closed_at`.

Routing: one rule per category to the department and the dataset's conditional responsible (SLA 24–72 h first response,
120–336 h resolution); «Корпоративная среда» has an extra `feedback` rule to «HR / внутренние коммуникации» (A-018).
**«Хранение документов» has no rule on purpose**: A-019 landed in the unrouted admin queue and the admin assigned it
manually to «Администратор документов» (timeline shows it). Submit a new request in that category to show the queue.

Departments: Закупки, Продажи, Проектирование, Бухгалтерия, Управление проектами, IT (no head: the dataset has no
«Руководитель IT»), Административный отдел и HR, Руководство направлений.

Accounts, password `Demo1234!` for all (e-mail = `<login>@modus.demo`); the first six are the quick logins:

| Role | Name | Email |
|---|---|---|
| author (employee) | Сотрудник закупок | purchasing.employee@modus.demo |
| author (employee) | Менеджер по продажам | sales.manager@modus.demo |
| responsible (head of Закупки) | Руководитель закупок | purchasing.head@modus.demo |
| responsible (IT) | IT-специалист | it.specialist@modus.demo |
| manager of another department (observer demo) | Руководитель проектирования | design.head@modus.demo |
| admin | Администратор системы | admin@modus.demo |
| other authors | Сотрудник проектирования / бухгалтерии / офиса, Координатор проекта, Руководитель проекта | design.employee@, accounting.employee@, office.employee@, project.coordinator@, project.lead@ |
| other responsibles | Руководитель направления, Ответственный за номенклатуру / обучение / процессы / договоры, Руководитель бухгалтерии / подразделения / продаж / проектов, Административный руководитель, HR / внутренние коммуникации, Администратор документов | direction.head@, catalog.owner@, training.owner@, process.owner@, contracts.owner@, accounting.head@, unit.head@, sales.head@, projects.head@, admin.head@, hr.comms@, documents.admin@ |

**Anonymous demo requests (DEMO ONLY tokens, public; real tokens are random and shown once):**

| Request | Status | Token |
|---|---|---|
| A-005 / MV-0005 | needs_info (answer by token) | `DEM0-A005-7K3Q-9XWB-2HRT-M4ZP-C8NV` |
| A-017 / MV-0017 | declined (motivated refusal) | `DEM0-A017-5JPV-W2QX-8KTN-R6MD-H3ZB` |
| A-018 / MV-0018 | closed (positive feedback) | `DEM0-A018-Q9RT-4MXK-7CWP-N2JH-B5VZ` |

They are also printed in the log when seeding. After start the overdue ticker emits overdue notifications/escalations.

## Request lifecycle (ARCHITECTURE R10)

Statuses: `new`, `in_review` (В проработке), `needs_info`, `decision` (Принятие решения), `in_progress` (Реализация),
terminal `implemented`, `declined`, `duplicate`, `closed` (Закрыто). Terminal statuses require `result_text`;
`closed` is reachable from every open status. Staff in scope add progress reports (comment kind `report`) while the
request is `in_review`, `decision` or `in_progress`; a named author is notified (anonymous authors see it by token).
Migration `0002_lifecycle` maps legacy `accepted` to `in_progress`.

Priority (CR1): `low|medium|high`, optional on create (default `medium`), in summaries/details and CSV; list filters
`priority`, `request_type` (`problem|idea|feedback`), `sort=priority` (high first, then the deadline order).

## Full anonymity (CR2, migration `0003_dataset_anonymity`)

- An anonymous request stores **no author**: `requests.author_id IS NULL` (CHECK `is_anonymous=1 ⇔ author_id IS NULL`);
  author-side rows are marked `by_author=1` with `comments.author_id` / `request_events.actor_id` NULL;
  `attachments.uploaded_by` NULL; no notification rows for the author. The user id is seen only by the in-memory
  per-user create rate limiter; the access log never contains user ids (tested by capturing logs).
- The server generates a token: 28 Crockford base32 symbols from `crypto/rand` (140 bits), grouped
  `XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX`; only SHA-256 of the canonical form is stored (`anon_token_hash`, unique). It is
  returned once, as `anonymous_token` in the 201 of `POST /requests`, and never logged. Input is case/separator
  insensitive (`O→0`, `I/L→1`).
- Session-less endpoints, limited per client IP (10/min per endpoint, 429): `POST /anonymous/lookup {token}` (author
  view, no internal notes), `POST /anonymous/comments {token, body}` (answer while `needs_info` → `in_review` and
  owners notified, else comment; closed → 409), `GET /anonymous/attachments/{id}` with header `X-Anonymous-Token`.
  Wrong or malformed tokens get the identical 404; hash comparison is constant time.
- `scope=mine` never lists anonymous requests; the creator's session gets 404 on them (there is no link).
- **Migration of existing data:** anonymous rows lose the author link irreversibly — their former authors can no
  longer open them (there are no tokens for legacy rows) and their notifications about them are deleted. The
  `.down.sql` cannot restore the link (author-less rows are attributed to the first admin).
- A staff member who files an anonymous request about their own department sees it as staff (owner), because the
  system no longer knows they wrote it; the "cannot review own request" rule cannot apply to anonymous requests.

## Manager visibility (CR6)

Managers read every request (`scope=all`, detail, attachments, similar items, analytics and themes company-wide;
`department_id` filters analytics). `access_level` per request: `owner` (admin; manager heading the request's
department or assigned to it), `observer` (any other manager: may add `comment` and `internal` only; status, assign,
question, report, duplicate, close and theme return 403; observer comments are not a first response and notify the
named author and the owner, observer internal notes notify the owner), `author`. `scope=inbox` and
`scope=all&mine_scope=true` keep the owned requests only. Suggested groups stay in the owner scope (they lead to theme
linking, an owner action).

Analytics (CR5): `first_response_on_time_share` (answered by `first_response_due`, which already includes needs_info
pause shifts, among requests with a first response), `first_response_sample`, `median_first_response_hours`,
`median_resolution_hours` (created → closed of requests CLOSED in the period, i.e. `closed_at` within from..to; all
time when no period is given), `by_type` (`{type, count, closed}` for problem/idea/feedback, always all three, same
period = creation date, scope and department filter as the other blocks). p90 removed.

## Configuration (env or `.env`, see `.env.example`)

| Var | Default | Meaning |
|---|---|---|
| `ADDR` | `:8080` | listen address |
| `DATA_DIR` | `./data` | SQLite file `modusvoice.db` and `uploads/` |
| `DEMO_MODE` | `false` | expose `/auth/demo-users` |
| `SEED_DEMO` | `false` | seed on start if the DB is empty; refused unless `DEMO_MODE=true` (`--seed`/`--seed-only` exit non-zero without it) |
| `TRUSTED_PROXY` | `false` | client IP from `X-Forwarded-For`/`X-Real-IP` (only behind your own reverse proxy); otherwise the TCP peer |
| `ALLOWED_ORIGINS` | – | extra origins for unsafe methods (Vite dev: `http://localhost:5173`) |
| `DISPLAY_TZ` | `Asia/Almaty` | CSV dates and analytics day buckets |
| `BCRYPT_COST` | `10` | password hash cost |
| `RATE_CREATE_PER_HOUR` | `20` | per-user limit for creating requests (429 above it) |
| `RATE_COMMENT_PER_HOUR` | `60` | per-user limit for comments (429 above it) |
| `TELEGRAM_BOT_TOKEN` | – | enables the Telegram bot (see below); secret, never logged |
| `TELEGRAM_BOT_URL` | – | `https://t.me/<bot>` shown by the UI / QR (default derived from the bot username) |
| `APP_BASE_URL` | `http://localhost:8080` | deep links in bot messages |
| `TELEGRAM_API_BASE` | `https://api.telegram.org` | Bot API endpoint (tests) |

No secrets are needed: sessions are random tokens stored hashed (SHA-256) in the DB.

## Telegram bot (CR4)

Module `internal/modules/telegram`. It starts **only when `TELEGRAM_BOT_TOKEN` is set**; without it nothing runs (no
goroutines), `GET /telegram/status` answers `enabled:false`, `GET /telegram/link-code` is 404 and
`/meta` must report `features.telegram=false` (`Config.TelegramEnabled()`).

1. Create a bot with @BotFather, set `TELEGRAM_BOT_TOKEN` (and `TELEGRAM_BOT_URL`, `APP_BASE_URL`) in `.env`, restart.
   No public URL is needed: the bot uses `getUpdates` long polling (25 s) and stops with the server (graceful).
2. The user opens the Telegram page in the web UI (`GET /telegram/link-code`): a 6-character code (alphabet without
   `0 O 1 I L`) that rotates every 20 s; the previous code stays valid for 3 s more. Codes live in an in-memory TTL
   cache behind `telegram.Cache` (swap in Redis by implementing `Get/Set/Delete`; one node needs nothing).
3. The user sends the code to the bot (`/start CODE`, or `/start` and then the code, or just the code). The chat is bound
   to the user forever (`telegram_links`, one user = one chat, no unlink anywhere). Codes are single use; 5 wrong codes
   per minute per chat lead to a cooldown. All existing notifications are marked as delivered, so no history is sent.
4. A worker (every 2 s) mirrors every unread-by-Telegram notification of linked users (`notifications.telegram_sent_at
   IS NULL`) in order, as plain Russian text with an «Открыть» button to `APP_BASE_URL/requests/{id}` (Telegram rejects
   localhost URLs in buttons, so for local runs the link is added to the text). Delivery is at-least-once and restart
   safe; a message that fails 5 times is marked sent with an error log. Admins get the admin notifications
   (unrouted requests, overdue escalations) through the same path.
5. Commands (Russian replies, same services and authorization as the web API with the linked user as `Actor`):
   `/start /help /me /my /inbox` (staff) `/request MV-0001 /answer MV-0001 text /admin` (admin). Anonymous requests are
   not reachable from the bot. Sends are limited to 25 msg/s overall and 1 msg/s per chat with `retry_after` handling.

Security: the token is only used inside request URLs and is stripped from every error; chat ids are never returned by
the API; every update is handled under panic recovery; message and argument lengths are capped.
Tests use a fake Telegram API (`TELEGRAM_API_BASE`); a real token is not needed.

## Test

```sh
go vet ./...
go test ./...            # unit + integration (temp SQLite) + HTTP (httptest)
go test -race ./...      # needs CGO (gcc) on Windows
```

Coverage map: `modules/requests/domain_test.go` (transitions, SLA/pause, routing fallback, AuthorView, upload sniffing),
`modules/insights/similarity_test.go`, `modules/identity/throttle_test.go`,
`app/access_test.go` (every cell of the access matrix, routing/AC-10, SLA pause, overdue ticker),
`app/http_test.go` (journeys J1–J4, anonymity across all endpoints, attachment rejections, login throttle, 401/403/404, CSV, admin),
`app/cr_*_test.go` (CR1 seed vs DATASET.md, CR2 DB/log scan + token flow + throttle + migration from a pre-0003 DB,
CR6 observer restrictions per endpoint, CR1 priority/type filters, CR5 KPIs), `modules/insights/kpi_test.go`.

## Layout

```
cmd/server            entrypoint (flags --seed, --seed-only)
migrations            ordered *.sql (embedded), *.down.sql for manual rollback
internal/app          config, composition root (modules.go = the only module registry), router
internal/platform     db, migrate, httpx (JSON, error envelope, middleware), authz (Actor, scope), eventbus, apperr
internal/modules      identity · org · requests · insights · notifications · telegram
internal/integrations ports (Notifier, DirectoryProvider, TaskSink, IdentityProvider), LogNotifier; bitrix24/onec/email stubs
internal/seed         synthetic demo data
internal/webui        go:embed of dist/ (copy web/dist here before `go build`), SPA history fallback
```

Rules: handlers parse/serialize only; business rules and authorization live in services (every method takes an
`authz.Actor`); stores are interfaces with a SQLite implementation; SQL is parameterized; author identity leaves the
requests module only through `requests.AuthorView`/`ActorView`; notifications are written in the same transaction
as the change (AC-08); the event bus is for non-critical subscribers only.

## How to add a module

1. Create `internal/modules/<name>/` with `domain.go`, `service.go`, `store.go` (interface), `store_sqlite.go`, `http.go`, `module.go`.
2. Put tables in a new `migrations/NNNN_<name>.sql` (next number; add a `.down.sql`).
3. In `service.go` take `authz.Actor` in every method; use `authz.RequireStaff/RequireAdmin/ScopeSQL` for access.
4. Return `apperr.*` errors (validation/forbidden/not found/conflict); handlers call `httpx.Error`.
5. Need another module? Declare a small interface in your package; never import another module's store.
6. In `module.go` expose `New(...) *Module`, `Name()`, `Routes(pub, priv chi.Router)`.
7. Register it in `internal/app/modules.go`: construct, wire interfaces, append to the returned list.
8. Subscribe to domain events with `a.Bus.Subscribe(topic, handler)` there if you react to request changes.
9. Add the paths to `docs/api/openapi.yaml` (contract change needs approval).
10. Add table-driven service tests and an `app` HTTP test for 200/400/401/403/404.

## Known limits

SQLite single node (PostgreSQL = new store implementations, see ARCHITECTURE §9). Overdue ticker runs in-process
every minute. Similarity is token Jaccard (no ML). Outbox table exists, dispatcher not built (R1). Login throttle is
in-memory per process and keyed by the TCP peer address (no `X-Forwarded-For` trust).
