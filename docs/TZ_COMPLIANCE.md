# TZ compliance matrix

Legend: ✅ built & verified · 🟡 designed/documented only (allowed by TZ) · ⏳ pending. Status updated at each stage gate. Verified = evidence in `docs/evidence/`.

| TZ § | Requirement | Where addressed | Status |
|---|---|---|---|
| 1.2 | Integrations not required; may propose with purpose & data | `INTEGRATIONS.md`, outbox/webhook seam | 🟡⏳ |
| 1.3 | Accessible channel for suggestions/problems | Submit flow J1, responsive UI | ⏳ |
| 1.3 | Reduce lost requests & manual ops | Auto-routing, unrouted queue, SLA/overdue | ⏳ |
| 1.3 | Store & systematize suggestions and decisions | requests + history + required result | ⏳ |
| 1.3 | Transparency & accountability | Timeline, owner, deadlines, notifications | ⏳ |
| 1.3 | Role-based access | RBAC matrix (ARCH §5) + tests | ⏳ |
| 1.3 | Phased development | ARCH §9, PRD §5 | ✅ docs |
| 1.3 | Find recurring problems | Themes/similarity, dashboard | ⏳ |
| 2.2 | Roles: employee / manager / admin; client = future | Seeded users per role; roadmap | ⏳ |
| 2.3 | Main scenario steps 1–6 | J1–J4 E2E test | ⏳ |
| 2.4 | Functional directions | PRD §5 Must/Should list | ⏳ |
| 2.4 | MVP minimal set chosen by team | PRD §5 | ✅ docs |
| 2.5 | Describe: routing, status/result, author awareness, no-loss, recurring | PRD §6, ARCH §4 | ✅ docs |
| 2.5 | Simplified routing & test data allowed | Rules table + seed | ⏳ |
| 2.6 | Define who creates/views/edits/handles; what each sees | ARCH §5 | ✅ docs |
| 2.6 | Anonymity: purpose, limits, effect | PRD §6.1, UI copy | ✅ docs ⏳ UI |
| 2.7 | Value + metrics (reviewed share, time to first response, outcome share, implemented count) | Analytics module | ⏳ |
| 3.1 | Problem, users, scenarios, MVP rationale, limits/assumptions, validation method | PRD §1–9 | ✅ docs |
| 3.2 | Interface: screens + transitions; interactive prototype of key scenarios | Working UI + `UX_SPECIFICATION.md` flow map | ⏳ |
| 3.3 | Technical concept: components, storage, tech + reasons, integrations, access, limits, growth | ARCHITECTURE | ✅ docs |
| 3.4 | Integration: purpose, data, direction, method, open questions | INTEGRATIONS | ✅ docs |
| 3.5 | Test/synthetic data; protection approach | Seed; ARCH §8 | ⏳ |
| 3.6 | MVP vs extensions vs separate-research items | ARCH §9 / PRD §5 | ✅ docs |
| 4.1 | Deliverables 1–6 (concept, users/scenarios, prototype, tech concept, roadmap, demo with limits) | PRD, UX spec, ARCH, running app, README + pitch | ⏳ |
| 4.2 | MVP extras: repo, run instructions, features & limits list, test data, e2e demo | README, seed, Playwright | ⏳ |
| 4.2 | Demo distinguishes implemented / simulated / planned | README "Status of features" table + demo script | ⏳ |
| 5 | Record assumptions and their impact | PRD §9 | ✅ docs |
| 6 | Answer 3 key questions (value; how it works; how to develop/integrate) | Pitch + README | ⏳ |
