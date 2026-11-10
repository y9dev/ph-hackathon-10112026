# PRD — "Modus Voice" (working name): employee feedback & suggestions system

Source: `spec/ТЗ_на_разработку_системы_обратной_связи_и_предложений_сотрудников.docx` (GK «Модус Трейд»). Traceability: `TZ_COMPLIANCE.md`.

## 1. Problem
Employees of a multi-function group (sales, procurement, design, technical/commissioning, IT, service, management) have no single, trusted channel to report problems or suggest improvements. Today ideas get lost in chats, calls and e-mail; nobody knows who owns them; authors never learn the outcome; recurring problems are invisible.

## 2. Value proposition (2 sentences)
One place where any employee can report a problem or idea in under a minute, have it routed automatically to the right owner with a deadline, and see the decision. Every request and decision is stored, so management can see recurring problems and how many improvements were really implemented.

## 3. Personas
| Persona | Needs | Pain today |
|---|---|---|
| **Employee** (e.g. commissioning engineer) | Report fast from phone/desktop, optionally anonymously, attach photo/file, know the status, get an answer | Ideas vanish, no feedback, fear of exposing a boss's mistakes |
| **Manager** (head of department/direction) | One queue of what needs review, deadlines, ability to ask author for details, record decision, delegate | Requests arrive in random channels, no deadlines, no history |
| **Administrator** | Manage categories, routing rules, SLAs, users/roles | Hard-coded processes, manual forwarding |
| Client (future) | Surveys, NPS | Out of MVP, see roadmap |

## 4. Core journeys (6-field format in `UX_SPECIFICATION.md`)
- **J1 Submit** — employee picks type (problem/idea) → category → writes title+description (+ proposed solution, attachments, anonymity toggle) → gets confirmation with tracking number and who owns it.
- **J2 Track** — employee opens "My requests", sees status timeline, deadline, comments, final decision; gets in-app notification on every change.
- **J3 Review** — manager opens inbox (new/overdue first), reads request, sets status, comments, asks for clarification, reassigns, records decision/actions, closes with a result.
- **J4 Clarify** — manager asks author a question (status `needs_info`) → author answers (even if anonymous) → request returns to review.
- **J5 Analyse** — manager/admin opens dashboard: KPIs from TZ (share reviewed, time to first response, share with recorded outcome, implemented count), by category/department, recurring problems, overdue list.
- **J6 Configure** — admin edits categories, routing rules (category → department → default owner → SLA), manages users/roles.

## 5. MVP scope (MoSCoW)
**Must (working end-to-end):** auth + 3 roles with RBAC; submit (type, category, text, proposed solution, up to 5 attachments, anonymous option); automatic routing by rule + SLA deadlines; status workflow with required result on close; comments + clarification requests; in-app notifications; my-requests list with timeline; manager inbox with filters/search/history; analytics dashboard (TZ metrics); recurring-problem detection (similarity grouping, duplicate linking); admin config of categories/routing/SLA/users; audit history per request; seeded synthetic demo data; Russian UI, responsive.
**Should:** overdue escalation flag + admin "unassigned/overdue" queue; CSV export of requests; dark theme; keyboard-friendly forms.
**Could (documented, not built):** Bitrix24 task/notification adapter, 1C/Bitrix24 directory sync (users, org structure), e-mail notifications, SSO, voting/public idea board, client surveys & NPS.
**Won't (MVP):** real integrations, mobile app, ML, multi-tenant.

## 6. Key product decisions
1. **Anonymous submission is supported, with explicit limits.** Author identity is stored only as a hidden link to allow the author to track and answer clarifications; it is never returned by the API to managers or admins (privacy by API contract, tested). Limits shown in UI: the owner cannot contact the author outside the system; attachments may reveal identity (warning shown).
2. **No lost requests (TZ 2.5):** every request is routed at creation; if no rule matches it goes to the admin "unrouted" queue; every request has a first-response deadline; overdue ones are flagged, shown first, and notify the owner and admins; nothing can be closed without a recorded result.
3. **Recurring problems:** token-similarity (normalized Jaccard over title+description, same category boost) groups requests into "themes"; managers can confirm/merge duplicates; dashboard ranks themes by count. Employees only see the aggregate "N colleagues reported similar", never others' content (privacy).
4. **Visibility model (updated CR6):** employee sees own requests; manager sees ALL requests (owner of own departments' ones, observer elsewhere: message/note only); admin sees all (author hidden if anonymous). Fine-grained matrix in `ARCHITECTURE.md`.
5. **Integrations are designed, not built** (ports & adapters + transactional outbox, `INTEGRATIONS.md`); demo uses the in-app notifier only; outbox and webhook delivery are planned (Phase 2).

## 6b. Change requests 2026-11-10 (evening) — see ARCHITECTURE §13
Data: demo data only from the hackathon dataset (20 requests, role-named users, `feedback` type, priority). Anonymity: **full** — an anonymous request stores no author in the DB; the author follows it by a secret token (shown once, kept in the browser); lost token = no access by design. Telegram bot (optional, enabled by `TELEGRAM_BOT_TOKEN`): link by rotating 20-second code, permanent link, notifications mirrored (admins get admin ones), commands /my /inbox /request /answer /admin. Analytics: «Ответили вовремя» replaces the median/p90 first-response tile; «Время решения» and counts by type added. Managers see all requests; outside their own departments they may only add a message or an internal note.

## 6a. Sponsor input (video from the customer, 2026-11-10)
**Confirmed by the customer:** Modus Trade is a Kazakhstan security-systems integrator (CCTV, fire, perimeter, ACS) working on state-critical objects; feedback today is scattered across e-mail, meetings, oral talk and notebooks; pain points: no single store, ideas lost, history unrecoverable, employees believe a problem is already handled, owners forget, no visibility, employees stop reporting. Wanted: one module for collecting problems/suggestions, assigning owners, discussing, tracking implementation. Three named stages: development, decision, implementation; the responsible person can ask questions and report on progress. FR-01…FR-08 (create, registry, manager view, status tracking, assign owner, clarifying questions, progress reports, implementation tracking).
**Mapping to the product:** FR-01 submit ✔ · FR-02 registry ✔ · FR-03 inbox ✔ · FR-04 status/timeline ✔ · FR-05 routing + assign ✔ · FR-06 `question` ✔ · FR-07 progress `report` comment (added, ARCH §12) · FR-08 stages `decision`/`in_progress` + stepper (added).
**Pain point → feature:** lost ideas → registry + SLA/overdue + unrouted queue; "already handled?" → request status visible to author + "similar requests" count + suggested groups; forgotten owners → deadlines, overdue flags, escalation to admin; no visibility → stepper, timeline, notifications, results; employees stop reporting → visible result card on closing and "implemented improvements" KPI; no history → immutable timeline + search.
**Context implications:** state-critical customers ⇒ data stays on-premises (single binary / Docker, no external cloud services, no third-party analytics) and Kazakhstan data-localisation rules apply to real deployments; UI strings are centralised so a Kazakh locale can be added; Telegram/e-mail are likely notification channels (adapters in INTEGRATIONS.md).
**Open questions to the customer (not decided by us):** login method / corporate directory; anonymous submission wanted?; categories and priority; edit/delete of a submitted request; who may assign owners and change statuses; manager approval of decisions; SLA values; Telegram/e-mail notifications; analytics scope (per department, average time, implemented count, per-owner efficiency). Current MVP defaults: local accounts, pseudonymous option, categories yes, priority no, no edit/delete after submit (new request instead), admin/head assign, heads decide, SLA per routing rule, in-app notifications, analytics as in §7.

## 7. Success measures (TZ 2.7; targets TBD with the company)
Share of requests reviewed; time to first response (median/p90); share of closed requests with recorded outcome (target 100% by construction); number of implemented improvements; number of overdue requests; share of requests assigned automatically.

## 8. Validation approach (TZ 3.1)
Pilot in 1–2 departments for 4 weeks; success if ≥80% of requests receive a first response within SLA and ≥60% of surveyed pilot users say they would use it again; compare lost/unanswered request count against the current chat-based baseline.

## 9. Assumptions & constraints
Synthetic data only; no integrations; Russian-language UI; single-company deployment; users ≤ ~1000, requests ≤ ~50k/year (SQLite adequate; PostgreSQL path documented); org structure is flat (department → head), not a full hierarchy.

## 10. Acceptance criteria (selected, testable)
- AC-01 Employee can submit a request in ≤ 4 interactions after login (type, category, text, submit) and receives a tracking number.
- AC-02 A submitted request in category C appears in the inbox of the owner defined by C's routing rule with deadline = created_at + SLA.
- AC-03 Manager cannot close a request without a result text; author sees the result.
- AC-04 Anonymous request: no API response to manager/admin contains author identity; author still sees it in "My requests" and can answer clarifications.
- AC-05 Employee cannot read another employee's request (403/404); manager cannot read requests outside their scope.
- AC-06 Request without matching rule lands in the admin unrouted queue.
- AC-07 Request past its first-response deadline is flagged overdue and listed first in the inbox.
- AC-08 Status change/comment/clarification creates a notification for the counterpart within the same transaction.
- AC-09 Dashboard shows the four TZ metrics and top recurring themes, scoped by role.
- AC-10 Admin can change a routing rule and the next new request follows it.
- AC-11 Attachments > 10 MB or disallowed types are rejected with a clear message.
- AC-12 All primary screens usable at 375 px and 1280 px width, keyboard-navigable, contrast ≥ 4.5:1.
