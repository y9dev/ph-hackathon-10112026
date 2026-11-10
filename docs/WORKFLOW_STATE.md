# Workflow state

- Mode: hackathon (48h). Start 2026-11-10 16:49 (+05:00), hard end 2026-11-12 16:49.
- Model policy: auto (high tier: opus). Docs language: English internal; UI and jury docs (README, ROADMAP, DEMO_SCRIPT): Russian.
- Current stage: 11 done (certification PASS WITH CONDITIONS, 4.05/5, no blockers). Remaining: user acceptance of conditions, demo data reset, pitch.
- Telegram: progress posts to team chat 5473304763 as zeril9 (user-authorized).

## Capabilities
Go 1.26.4, Node 24, git, Docker 29.5 (works), Playwright 1.65; subagents available; built-in browser pane renders unreliably (headless Playwright used by auditors).

## Stage log
0–5 inline. Arch review (3 High, resolved ARCH §11). Backend + web built in parallel against frozen docs/api/openapi.yaml. Stage 7: code review (0 Crit/High) and security review (0 Crit/High, 1 Medium fixed). Sponsor video input → lifecycle R10 + `closed` status (ARCH §12). Stage 9: usability + UI/a11y audits, all High fixed (commit f59fceb). Stage 11: QUALITY_SCORECARD.md, docs/evidence/certify-1.md.

## Open items
zod advisory (GHSA-mrhx-c7xc-m3w5, low in context) needs user acceptance or upgrade; mobile /new first text field below fold with 9 categories; duplicate notifications; notifications unscoped after reassignment; no pitch deck file; run `docker compose down -v && docker compose up --build` before the demo (E2E data pollutes suggested groups; create limit 20/h per user).
