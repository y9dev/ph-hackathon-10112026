# Future integration design (TZ 1.2, 3.4)

Nothing here is required for the MVP; the MVP ships the seams (ports, events, outbox, OpenAPI) so each item below is an adapter, not a rewrite.

## Principles
1. **Ports & adapters:** `backend/internal/integrations/` defines interfaces; adapters live beside them. Business modules never import an adapter.
2. **Events out, commands in:** domain events (`request.created`, `…assigned`, `…status_changed`, `…closed`, `…overdue`) go to a transactional **outbox**; a dispatcher delivers at-least-once with exponential backoff and idempotency key `event_id`. Inbound integrations call the public REST API (OpenAPI) with a service token.
3. **Minimum data:** external systems get ids, titles, status and routing info; anonymous authorship is never exported.

## Candidate integrations
| # | System | Purpose | Direction & data | Method (to confirm) | Open questions for the company |
|---|---|---|---|---|---|
| 1 | **Bitrix24** — users & org structure | Replace manual user/department admin | In: employees, departments, heads, active flag. Out: none | Bitrix24 REST (`user.get`, `department.get`) via `DirectoryProvider`, nightly + on-demand sync | Cloud or on-prem? Webhook vs OAuth app? Which field marks department head? |
| 2 | **Bitrix24** — SSO | One login for employees | In: identity | OAuth2/OIDC via `IdentityProvider`; keep local admin fallback | Is Bitrix24 the identity source or AD/LDAP? |
| 3 | **Bitrix24** — tasks | Turn accepted improvements into tracked work | Out: title, link, deadline, assignee. In: task status → request status | `TaskSink` adapter: `tasks.task.add` on `request.accepted`; webhook/poll back for completion | Which project/group? Who owns created tasks? |
| 4 | **Bitrix24** — notifications | Reach people where they already work | Out: notification text + deep link | `Notifier` adapter: `im.notify` / `im.message.add` | Bot or personal notification? |
| 5 | **E-mail (SMTP)** | Notify when away from Bitrix24 | Out: same events | `Notifier` adapter, digest option | Corporate SMTP relay? |
| 6 | **1C УТ** | Link equipment/product/supplier to a problem (e.g. recurring defect on item X) | In: nomenclature/counterparty references (read-only) | `ReferenceProvider` via 1C HTTP service/OData | Which entities matter? Who may expose them? |
| 7 | **BI / data warehouse** | Deeper analytics | Out: anonymized request facts | Read-only export endpoint / CSV (built) → scheduled ETL | Existing BI tool? |

## Integration-readiness in the MVP
- OpenAPI contract in `docs/api/openapi.yaml`; versioned `/api/v1`.
- Outbox table + generic **webhook adapter** (built; points at any URL, HMAC-signed) — the demo can show events landing in a webhook inspector.
- `Notifier` and `DirectoryProvider` ports with in-app/seed implementations.
- Users keep an optional `external_id` + `source` column (added in migration 0001) for directory sync mapping.
- Service-token auth path reserved in `identity` for machine clients.

## Risks / to clarify before implementation
Data ownership and retention rules; whether anonymity is legally/organizationally acceptable; Bitrix24 edition limits; rate limits; error-handling policy when the external system is down (outbox retries, then dead-letter list for admins).
