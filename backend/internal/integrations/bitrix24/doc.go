// Package bitrix24 is a documented stub (not built in the MVP, see docs/INTEGRATIONS.md).
//
// Planned adapters:
//   - DirectoryProvider: REST user.get / department.get via incoming webhook or OAuth app,
//     nightly + on-demand sync into users(external_id, source='bitrix24') and departments.
//   - Notifier: im.notify / im.message.add with a deep link to /requests/{id}.
//   - TaskSink: tasks.task.add on the transition to in_progress; task completion polled or received by
//     webhook and mapped to request status implemented.
//   - IdentityProvider: Bitrix24 OAuth2 as SSO with local admin login kept as fallback.
//
// Delivery goes through the outbox table (at-least-once, exponential backoff, idempotency key
// = event id). Anonymous authorship is never exported.
package bitrix24
