# Web UI evidence (2026-11-10)

Commands run in `web/`:

| Command | Result |
|---|---|
| `npm run lint` (eslint + `tsc -b`) | 0 problems |
| `npm test` (Vitest) | 4 files, 36 tests passed |
| `npm run build` | built into `backend/internal/webui/dist`; mock code absent from the production bundle |
| `npm run e2e` (mock, desktop 1280 + Pixel 7) | 6 passed: J1 submit (+mobile), J2 track, J3 review+close with result, J4 clarify, role guards |
| `E2E_BASE_URL=http://localhost:5175 npm run e2e:real` (real backend `DEMO_MODE=true go run ./cmd/server --seed`, Vite in real mode) | 5 passed |
| `SCREENS=1 npx playwright test e2e/screens.spec.ts --project=desktop` | 2 passed; asserts no horizontal overflow at 1280 and 375 |

Screenshots (`screens/`, mock data, light theme): `01-login`, `02-my-requests`, `03-submit`, `04-success`,
`05-detail-author`, `06-inbox`, `07-detail-staff`, `08-analytics`, `09-notifications`, `10-admin-routing`,
`11-admin-categories`, `12-admin-users` — each at `-1280.png` and `-375.png`, plus `13-close-dialog-validation-1280.png`.
Mobile full-page captures show fixed bars (bottom tab bar, sticky submit) in the middle of the stitched image;
that is a capture artefact, not the layout.
