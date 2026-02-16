# kay-vee: Admin Console Implementation Plan

## Objective
Add a first-class `kay-vee` surface to `admin-console` so operators can inspect and manage local Parameter Store + Secrets Manager emulator state without leaving the consolidated UI.

## Current Baseline
- `kay-vee` exposes:
  - AWS JSON operations for SSM/Secrets on `POST /` via `X-Amz-Target`
  - Admin endpoints: `/admin/api/summary`, `/admin/api/activity`, `/admin/api/export`, `/admin/api/import`
- `admin-console` currently supports:
  - Full action UX for `ess-queue-ess` and `ess-enn-ess`
  - Informational summary UX for `essthree` and `cloudfauxnt`
  - Per-view SSE refresh (`/api/events?view=...`)

## Scope (Phase-Oriented)

### Phase 0 — Stack Wiring
- Add `kay-vee` service to root `docker-compose.yml` and shared network.
- Add `kay-vee` dependency for `admin-console` startup ordering.
- Ensure `admin-console` can reach `http://kay-vee:9350`.

**Acceptance criteria**
- `make up` starts `kay-vee` and `admin-console` together.
- `admin-console` can successfully call `kay-vee /health` and `/admin/api/summary`.

### Phase 1 — Informational Kay-Vee View (Read-Only)
Backend (`services/admin-console/internal/server/server.go`):
- Add dashboard stats integration for `kay-vee` (parameters, secrets total/active/deleted).
- Add `/api/services/kay-vee/summary` proxy endpoint (from `kay-vee /admin/api/summary`).
- Add `/api/services/kay-vee/activity` endpoint with pass-through pagination (`maxResults`, `nextToken`).
- Add `/api/services/kay-vee/export` pass-through endpoint for snapshot download.

Frontend (`services/admin-console/web/index.html`, `web/app.js`):
- Add left-nav menu item for `kay-vee`.
- Add `switchView('kay-vee')` branch and renderer.
- Render summary cards + activity table + export action.
- Hook SSE support for `view=kay-vee` to refresh summary/activity read-only data.

**Acceptance criteria**
- New `kay-vee` menu renders summary and recent activity.
- Export download works from UI.
- Dashboard includes a `kay-vee` service card.

### Phase 2 — Parameter Admin Actions
Backend:
- Add admin-console action routes for common parameter workflows:
  - list/describe/by-path
  - put/update
  - delete single/bulk
  - label version
- Implement an internal helper to call `kay-vee` AWS JSON target API and normalize errors.

Frontend:
- Add parameter list/table with path + filter controls.
- Add create/update form.
- Add delete + label actions.
- Add clear success/error status UX consistent with existing admin-console patterns.

**Acceptance criteria**
- Parameter CRUD+label operations are executable entirely through admin-console.
- Validation and backend errors surface clearly in UI.

### Phase 3 — Secret Admin Actions
Backend:
- Add admin-console action routes for common secret workflows:
  - list/describe/get value
  - create/update/put value
  - delete/restore
  - update version stage

Frontend:
- Add secrets table + details panel.
- Add create/update/delete/restore/stage actions.
- Add optional secret value reveal toggle (default hidden).

**Acceptance criteria**
- Secret lifecycle operations are executable through admin-console.
- Deleted/active state transitions are reflected correctly in UI.

### Phase 4 — State Portability + Import UX
Backend:
- Add `/api/services/kay-vee/import` route to proxy `POST /admin/api/import`.
- Add payload validation and error mapping for import failures.

Frontend:
- Add import file/paste workflow with confirmation.
- Add post-import refresh of summary, parameter, and secret views.

**Acceptance criteria**
- Export from one environment and import into another from admin-console.
- UI reflects imported state without page reload.

## API Contract (Admin Console ↔ Kay-Vee)

### Direct pass-through endpoints
- `GET /health`
- `GET /admin/api/summary`
- `GET /admin/api/activity`
- `GET /admin/api/export`
- `POST /admin/api/import`

### AWS JSON target calls from admin-console
- Route through helper that sets:
  - `Content-Type: application/x-amz-json-1.1`
  - `X-Amz-Target: <service target>`
- Initial target set for admin console actions:
  - `AmazonSSM.*` (Put/Get/Describe/GetByPath/Delete/Label/History)
  - `secretsmanager.*` (Create/Get/Put/Update/List/Describe/Delete/Restore/UpdateStage)

## Error Handling Strategy
- Preserve upstream `__type` and `message` from `kay-vee` responses where available.
- Admin-console should return stable JSON error shape:
  - `{ "error": "...", "type": "ValidationException" }`
- Map network/upstream failures to `502` consistently.

## SSE & Refresh Strategy
- Extend `/api/events` validation allow-list with `kay-vee` view.
- For `kay-vee` SSE payloads:
  - Phase 1: summary + latest activity page
  - Phase 2/3+: include view-specific list payloads only for active tab
- Keep current active-view-only policy to limit noisy updates.

## Testing Plan

### Backend (Go)
- Add tests for new admin-console routes:
  - summary/activity/export/import proxies
  - parameter and secret action wrappers
  - error mapping behavior
- Use `httptest` upstream stubs for deterministic contract tests.

### Frontend
- Add lightweight behavior checks for:
  - menu/view switching
  - rendering empty/loading/error states
  - action form validation and optimistic refresh behavior

### End-to-End
- Add stack-level smoke tests:
  - create parameter via console → confirm in `kay-vee`
  - create secret via console → confirm list/details
  - export/import roundtrip via console

## Delivery Sequence (Recommended)
1. Phase 0 + Phase 1 in one PR (read-only value quickly).
2. Phase 2 parameters in one PR.
3. Phase 3 secrets in one PR.
4. Phase 4 import UX + docs polish in final PR.

## Risks & Mitigations
- **Risk:** AWS JSON target complexity in admin-console backend.
  - **Mitigation:** implement one generic `callKayVeeTarget(target, payload)` helper early.
- **Risk:** UI scope expansion becomes too broad.
  - **Mitigation:** ship read-only first, then parameters, then secrets.
- **Risk:** SSE churn from high activity volume.
  - **Mitigation:** keep bounded activity pages and active-view-only streams.

## Definition of Done
- `kay-vee` appears as a first-class service in dashboard and side nav.
- Core parameter + secret operator workflows work end-to-end.
- Export/import available in console.
- Route and UI docs updated in `services/admin-console/README.md` and `services/kay-vee/README.md`.
- New functionality covered by automated tests.
