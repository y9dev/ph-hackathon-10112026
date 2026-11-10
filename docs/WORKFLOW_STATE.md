# Workflow state

- Mode: hackathon (48h)
- Start: 2026-11-10 16:49 (+05:00). Hard end: 2026-11-12 16:49.
- Planned clock: skeleton by 2026-11-11 04:49 (25%), feature freeze 2026-11-12 04:49 (75%), final gate + demo materials 2026-11-12 10:00–16:49.
- Model policy: auto (high tier alias: opus). Routine implementation one tier lower per hackathon rule; reviewers/security/gate never lowered.
- Docs language: English (internal). UI language: Russian. Jury-facing README/pitch: Russian (decide at gate).
- Current stage: 1–2 (merged approval gate)

## Capabilities (probed 2026-11-10)
- Go 1.26.4, Node 24.16, npm 11.16, git 2.54, Docker 29.5, Playwright 1.65 (npx, browsers not yet verified)
- Browser tools: built-in browser pane (mcp__Claude_Browser__*), Claude-in-Chrome
- Subagents: Agent tool available; installed types sew-* present (sew-backend-engineer, sew-frontend-engineer, sew-code-reviewer, sew-security-reviewer, sew-qa-engineer, sew-usability-auditor, sew-ui-critic, sew-accessibility-reviewer, sew-architecture-reviewer)
- OMC deep-interview: present, not used (hackathon: <=2 interview rounds, assignment fully read)
- Repo: greenfield, not a git repo (git init at skeleton)

## Doc mapping (hackathon profile)
PRD.md (personas, journeys, acceptance), ARCHITECTURE.md (data model, API, decisions), UX_SPECIFICATION.md (IA, tokens), TZ_COMPLIANCE.md (spec traceability), INTEGRATIONS.md (future integration design), QUALITY_SCORECARD.md.

## Baseline
Greenfield: none.

## Decisions log
(see ARCHITECTURE.md §Decisions)

## Open findings
none yet

## Next action
Present merged requirements + MVP scope + architecture for user approval.
