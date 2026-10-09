# Frontend

The frontend is a strict TypeScript React/Vite app using TanStack Query for
server state. The first screen is the printer registry and deliberately has
explicit loading, API error and empty states. It does not invent printer data
when the backend API is unavailable.

The **Add printer** form uses the atomic `POST /api/v1/printers/register`
operation to create the device, an encrypted SNMP v2c credential and a primary
SNMP endpoint in one SQLite transaction. If a later step fails, the whole
registration is rolled back. The community string is write-only from the UI
and is never returned by the API. SNMPv3 credentials can be
provisioned through the authenticated API endpoint documented in
`docs/openapi.yaml`.

Printer details include an SVG counter trend chart. It plots the raw values of
the first counter definition returned for the device, keeps unit and quality
visible, and keeps incompatible counters separate. The detail page also shows
derived daily usage from trusted monotonic intervals; midnight-spanning
intervals are marked `unverified` because their exact boundary allocation is
estimated. Those daily deltas are also grouped by counter and calendar month,
without mixing incompatible counter definitions.

Use Node.js 20.19+ LTS on Linux or Windows. Dependencies are pinned in
`pnpm-lock.yaml`; use pnpm 12.8.1 with a frozen lockfile for a clean
reproducible install.

Linux/macOS:

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm run api:check
pnpm run typecheck
pnpm run format:check
pnpm run lint
pnpm run build
```

Windows PowerShell uses the same commands after `Set-Location frontend`.
`api:check` regenerates `src/api/generated.ts` from the Huma OpenAPI document
and fails if the generated file is not committed.
For the normal standalone deployment, the frontend is served by the Go
binary. Set `server.listen`, `server.allowed_origin` and a generated
`server.api_token` in the root `config.yaml`. When the browser receives its
first `401`, it asks for this token and keeps it only in the current browser
session. The token is never committed to frontend source or static assets.

When the Vite development server is hosted separately, `VITE_API_BASE_URL`
and `VITE_API_TOKEN` may be provided as build-time variables; do not commit a
file containing a real token. The production runtime configuration remains
`config.yaml`.

Run the standalone API from `backend/` with
`go run ./cmd/api --config ../config.yaml`. Its listen address and required
API token are read from `config.yaml`.

Manual status polling is asynchronous: `POST /api/v1/printers/{id}/poll`
returns `202` with a job ID; query `/api/v1/jobs/{job_id}` for the result.
Job state is persisted in SQLite and is therefore restart-safe.
