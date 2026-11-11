# TZ compliance matrix

Legend: ✅ built & verified · 🟡 designed/documented only (allowed by TZ) · ⏳ pending · ⚠️ partially verified. Verified = evidence in `docs/evidence/`.
Last status update: 2026-11-11 (spam moderation, unowned-request visibility, TZ 4.1 deliverables; the 2026-11-10 certification run is in `docs/evidence/certify-1.md`, scorecard `docs/QUALITY_SCORECARD.md`).

| TZ § | Requirement | Where addressed | Status |
|---|---|---|---|
| 1.2 | Integrations not required; may propose with purpose & data | `INTEGRATIONS.md`, ports in `backend/internal/integrations/ports.go` (outbox dispatcher/webhook not built) | 🟡 |
| 1.3 | Accessible channel for suggestions/problems | Submit flow J1, responsive UI | ✅ E2E J1 x3, 375/768 px browser check (certify-1) |
| 1.3 | Reduce lost requests & manual ops | Auto-routing, unrouted queue, SLA/overdue | ✅ API smoke: rule routing + unrouted category; TestRoutingAndAdminRuleChange, TestSLAPauseAndOverdueTicker |
| 1.3 | Store & systematize suggestions and decisions | requests + history + required result | ✅ API smoke: close without result → 400; timeline in E2E J3 |
| 1.3 | Transparency & accountability | Timeline, owner, deadlines, notifications | ✅ E2E J3/J5; author notifications in API smoke |
| 1.3 | Role-based access | RBAC matrix (ARCH §5) + tests | ✅ TestAccessMatrix; API smoke 404/403 cross-scope |
| 1.3 | Phased development | ARCH §9, PRD §5 | ✅ docs |
| 1.3 | Find recurring problems | Suggested groups, themes, dashboard | ✅ `/analytics/suggested-groups` returns groups; TestClusterFindsGroupsAndIgnoresNoise (heuristic, not ML) |
| 2.2 | Roles: employee / manager / admin; client = future | Seeded users per role; roadmap | ✅ demo logins for all 5 accounts (certify-1) |
| 2.3 | Main scenario steps 1–6 | J1–J5 E2E test | ✅ real-backend E2E 6/6 passed, 3 runs |
| Sponsor video | Customer stages Разработка → Принятие решения → Реализация; progress reports (FR-07/08) | ARCH §12 (R10), statuses `decision`/`in_progress`/`closed`, comment kind `report`, stepper | ✅ E2E J3 lifecycle + J5 close; API smoke report/closed; browser stepper check |
| Sponsor video | Pain points (lost ideas, "already handled?", forgotten owners, no visibility, no history) | PRD §6a mapping table | ✅ docs, ✅ UI (stepper, timeline, similar count, overdue) |
| Sponsor video | Open customer questions (auth, anonymity, edit/delete, notifications channel, analytics scope) | PRD §6a list; defaults chosen and flagged | ✅ docs |
| 2.4 | Functional directions | PRD §5 Must/Should list | ✅ (Must list built; Could items documented only) |
| 2.4 | MVP minimal set chosen by team | PRD §5 | ✅ docs |
| 2.5 | Describe: routing, status/result, author awareness, no-loss, recurring | PRD §6, ARCH §4, §12 | ✅ docs |
| 2.5 | Simplified routing & test data allowed | Rules table + seed | ✅ synthetic seed; rules via `/admin/routing-rules` |
| 2.6 | Define who creates/views/edits/handles; what each sees | ARCH §5 | ✅ docs |
| 2.6 | Anonymity: purpose, limits, effect; spam moderation | PRD §6.1, ARCH §14, DEMO_SCRIPT «Ограничения», UI copy | ✅ API smoke: no author identity in admin/manager detail, list, CSV; TestAnonymity; TestAnonymousSpamBlockAndBrowserStatus (block list stores hashes only, browser status check) |
| 2.7 | Value + metrics (reviewed share, time to first response, outcome share, implemented count) | Analytics module | ✅ `/analytics/summary` kpis + analytics page (certify-1) |
| 3.1 | Problem, users, scenarios, MVP rationale, limits/assumptions, validation method | PRD §1–9 | ✅ docs |
| 3.2 | Interface: screens + transitions; interactive prototype of key scenarios | Working UI + `UX_SPECIFICATION.md` flow map | ✅ running app, screenshots in `evidence/screens/` |
| 3.3 | Technical concept: components, storage, tech + reasons, integrations, access, limits, growth | ARCHITECTURE | ✅ docs (some stale lines, see scorecard) |
| 3.4 | Integration: purpose, data, direction, method, open questions | INTEGRATIONS | ✅ docs (webhook "built" claim is inaccurate, see scorecard) |
| 3.5 | Test/synthetic data; protection approach | Seed; ARCH §8 | ✅ synthetic seed; security headers, CSRF origin check, upload checks verified (certify-1) |
| 3.6 | MVP vs extensions vs separate-research items | ARCH §9 / PRD §5 | ✅ docs |
| 4.1 | Deliverables 1–6 (concept, users/scenarios, prototype, tech concept, roadmap, demo with limits) | PRD, UX spec, ARCH (incl. §14), running app, README, ROADMAP, DEMO_SCRIPT (with limits), pitch deck (slide 7 «Ограничения»), backup video `pitch/demo-backup.mp4` | ✅ present; the demo script and the limits are written for the current build |
| 4.2 | MVP extras: repo, run instructions, features & limits list, test data, e2e demo | README, seed, Playwright | ✅ clean-clone Docker build + E2E (see scorecard) |
| 4.2 | Demo distinguishes implemented / simulated / planned | README feature status table + demo script | ✅ README table verified row by row (scorecard) |
| 5 | Record assumptions and their impact | PRD §9 | ✅ docs |
| 6 | Answer 3 key questions (value; how it works; how to develop/integrate) | DEMO_SCRIPT «Ответы на три ключевых вопроса» + pitch slides 3–5 | ✅ present in DEMO_SCRIPT and in the pitch |
