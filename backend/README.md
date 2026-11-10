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

Demo accounts (synthetic), password `Demo1234!` for all:

| Role | Email |
|---|---|
| Admin (Руководство) | admin@modus.demo |
| Manager, IT-разработка | e.kuznetsova@modus.demo |
| Manager, Сервис | s.volkov@modus.demo |
| Manager, Продажи / Закупки / Проектирование / Пусконаладка | a.melnikov@ / o.nazarova@ / d.lebedev@ / i.gromov@modus.demo |
| Employee | i.sidorov@modus.demo, a.petrova@modus.demo (+12 more) |

The seed is deterministic: ~75 requests over the last 90 days, all statuses, 4 similarity clusters
(double entry 1С/Битрикс24, late supplier shipments, no commissioning checklist, slow 1С reports),
~12% anonymous, ~8 overdue, category «Другое» has no routing rule (unrouted admin queue).
After start the overdue ticker emits overdue notifications/escalations for the seeded data.

## Request lifecycle (ARCHITECTURE R10)

Statuses: `new`, `in_review` (В проработке), `needs_info`, `decision` (Принятие решения), `in_progress` (Реализация),
terminal `implemented`, `declined`, `duplicate`, `closed` (Закрыто). Terminal statuses require `result_text`;
`closed` is reachable from every open status. Staff in scope add progress reports (comment kind `report`) while the
request is `in_review`, `decision` or `in_progress`; the author is notified (also when anonymous).
Migration `0002_lifecycle` maps legacy `accepted` to `in_progress`.

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

No secrets are needed: sessions are random tokens stored hashed (SHA-256) in the DB.

## Test

```sh
go vet ./...
go test ./...            # unit + integration (temp SQLite) + HTTP (httptest)
go test -race ./...      # needs CGO (gcc) on Windows
```

Coverage map: `modules/requests/domain_test.go` (transitions, SLA/pause, routing fallback, AuthorView, upload sniffing),
`modules/insights/similarity_test.go`, `modules/identity/throttle_test.go`,
`app/access_test.go` (every cell of the access matrix, routing/AC-10, SLA pause, overdue ticker),
`app/http_test.go` (journeys J1–J4, anonymity across all endpoints, attachment rejections, login throttle, 401/403/404, CSV, admin, seed).

## Layout

```
cmd/server            entrypoint (flags --seed, --seed-only)
migrations            ordered *.sql (embedded), *.down.sql for manual rollback
internal/app          config, composition root (modules.go = the only module registry), router
internal/platform     db, migrate, httpx (JSON, error envelope, middleware), authz (Actor, scope), eventbus, apperr
internal/modules      identity · org · requests · insights · notifications
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
