# AGENTS.md — Coding agent instructions for NPMS

Read `README.md` completely before editing code. This file is authoritative for agent workflow and guardrails; `README.md` is authoritative for product scope and acceptance. Communicate implementation status in Vietnamese.

## Mission

Implement NPMS v1: Go + SQLite + React, LAN SNMP-first printer management (<50 devices), runnable on Linux and Windows. Target HP M501dn, HP M404dn, Brother HL-L5100DN and Ricoh IM 2xxx through generic and extensible model profiles. Build **no USB agent, Windows agent, Linux agent, CUPS or IPP transport** in v1. Future compatibility is achieved through stable device IDs and transport-neutral observation contracts, not speculative infrastructure.

## Working protocol

1. Inspect repository state before editing. Do not assume files, tests, dependencies or real hardware exist.
2. State the current milestone, impacted packages, test plan and assumptions briefly.
3. Implement one vertical slice at a time; keep changes focused and reviewable.
4. Prefer working code, migrations, tests and documentation over elaborate abstractions.
5. Run relevant tests/lint/build after each slice; report actual results, never claim unrun tests passed.
6. Keep `README.md`, `docs/openapi.yaml`, migrations and `.env.example` synchronized with changes.
7. If printer-specific behavior cannot be verified, add fixture-backed generic support and mark it `experimental` or `unverified`; document what sample is needed.
8. Never fabricate SNMP results, OIDs, screenshots, performance figures or successful hardware tests.

## Priority execution order

- P0: repository bootstrap, SQLite configuration/migrations, logging and health checks.
- P1: `snmp-debug` CLI (GET/WALK, v2c/v3, timeout, JSON, credential redaction) with mocked transport tests.
- P2: profile YAML schema/validation/resolution and generic Printer-MIB profile.
- P3: SQL migrations, device registry, observations, polling runs, counter definitions/readings/epochs/events.
- P4: counter validation, reset/anomaly handling, idempotent daily usage.
- P5: bounded worker polling, discovery allowlist, job tracking and retry.
- P6: authenticated REST API + OpenAPI 3.1 and frontend.
- P7: operational docs, integration/e2e tests, real-device verification checklist.

Do not start with a decorative dashboard. The first demonstrable feature is a reliable SNMP diagnostic CLI.

## Architecture rules (MUST)

- Modular monolith with API and worker as separate executables, one Go module initially.
- Domain packages must not depend on HTTP, database drivers, SNMP transport or React.
- SNMP adapter handles transport and decoding only; profile handles device-specific interpretation; counter engine validates and computes; repository persists.
- Profiles do not write database or call external commands. No arbitrary executable YAML.
- Device identity uses UUID; never IP or hostname as primary key.
- Keep endpoint/transport separate from device; counter definitions include source, scope, unit, OID and instance.
- Raw readings are immutable; do not overwrite with deltas. Store UTC and report in configured timezone.
- Use explicit capability/quality states (`valid`, `unverified`, `unsupported`, `unavailable`, `suspicious`), not magic zero.
- Keep observations transport-neutral so future agents can reuse ingestion without schema rewrite.
- Do not add message brokers, plugin `.so` loading, distributed agents, dynamic scripting or microservices without a documented need.

## SNMP correctness (MUST)

- SNMP v2c and v3; prefer v3 authPriv; credentials must never be logged or returned.
- WALK column OIDs and join marker life count to counter unit by **identical instance**.
- Never assume first marker is total, every marker is pages, or sum marker rows without validated rules.
- Do not equate impressions with physical sheets, or copy/print/scan totals without model evidence.
- Vendor OIDs require documented model/firmware fixture plus cross-check with printer panel/config page.
- Profile selection order: explicit > exact sysObjectID > model/vendor > vendor generic > generic. Equal match => explicit ambiguity error.
- Timeouts must not produce zero counter readings or automatically mean printer powered off.
- Poll retry must not duplicate a reading. Limit discovery to approved subnet/IP targets and bounded concurrency.

## Counter invariants (MUST)

- Valid monotonic increase: delta = new - previous, only within the same verified epoch and sane limits.
- Equal value: delta 0. Missing reading: no delta. Decrease: suspected reset/wrap event, not negative usage.
- Spike: suspicious flag and excluded from trusted aggregates pending policy/review.
- Preserve original raw value, unit, source, timestamp and poll run for traceability.
- Recalculation of daily usage must be deterministic/idempotent. Intervals spanning midnight are estimated unless exact boundary data exists.
- Counter schema must support different scope (engine/print/copy), unit (impressions/sheets), and sources without merging incompatible metrics.

## Database and API standards

- SQLite migrations are versioned, idempotent where practical and tested on clean DB.
- Use parameterized queries; transactionally persist poll run + readings + events where appropriate.
- Index by counter definition/time and device/poll time. Add uniqueness for polling idempotency.
- API prefix `/api/v1`; use UUID, pagination, structured error codes, 202 for async jobs.
- Authenticate all management endpoints; authorization for writes and credentials; redact secrets from responses.
- Generate/maintain `docs/openapi.yaml` alongside handler changes. Validate request input and discovery CIDRs.
- Never hard-delete historical counters as a side effect of removing a printer; use soft deletion.

## API contract and agent coding rules (MUST)

- Use `net/http` as the runtime and Huma as the typed API boundary when adding or migrating REST endpoints; do not introduce a larger web framework without a documented need.
- Define each API operation's request, response, validation, errors and operation ID in Go typed API code. Keep business logic in application services and persistence in repositories; API handlers must not contain SQL.
- Treat generated OpenAPI and TypeScript client/types as artifacts. Do not hand-edit generated files or maintain duplicate frontend DTOs.
- Generate `docs/openapi.yaml` and frontend API types/client from the typed API contract. A contract change is incomplete until generated artifacts, frontend usage, tests and documentation are synchronized.
- CI/deployment gates MUST run generation, fail on a dirty generated diff, validate OpenAPI, run Go tests/vet, run frontend typecheck/lint/build, and run API contract tests against real handler responses.
- Migrate endpoint groups incrementally (cartridge, printer/counter, discovery/jobs); do not rewrite the entire API or mix framework migration with unrelated SNMP/domain changes.
- Before deployment, verify database migrations on a clean SQLite database and an existing database, then run the generated-contract check and the complete automated test suite. Report real command results.

## Coding and tests

- Go: idiomatic small interfaces defined near consumers; `context.Context` for I/O; wrap errors; structured `slog`; `gofmt`, `go vet`, `go test ./...`.
- React: TypeScript strict, typed API clients, TanStack Query for server state, loading/error/empty states; accessible tables/forms.
- Unit tests: SNMP decoding/index joins, profile matching/ambiguity, counter monotonic/reset/spike/wrap, timezone boundaries, auth validation.
- Integration tests: SQLite migrations and repositories, polling job idempotency, fake SNMP devices and failure paths.
- Fixture tests: anonymized SNMP JSON for HP M501dn/M404dn, Brother HL-L5100DN and exact Ricoh model when available; clearly label absent fixtures.
- CI should run Go tests/vet, frontend lint/typecheck/build and migration checks.
- Never require real printers to run the default automated test suite.

## Secrets and safety

- Do not commit `.env`, community strings, SNMPv3 credentials, database passwords, encryption keys or real production device exports.
- Store SNMP secrets encrypted at rest; load encryption key from deployment environment/secret store.
- Do not log credentials or full raw dumps containing secrets. Mask addresses/serials in shared fixtures when appropriate.
- Scan only explicitly authorized local networks. Never create unrestricted public network scanner APIs.
- No destructive migrations or bulk deletes without explicit approval and backup guidance.

## Definition of Done per feature

A feature is complete only when it has: working implementation, unit tests, relevant integration tests, validation/errors, docs/API updates, and a short note of limitations. For profile support, additionally require raw sample + device panel/config-page verification before declaring `verified`.

## Expected delivery report (Vietnamese)

At the end of each milestone provide:

1. Files created/changed and key decisions.
2. Commands actually executed and pass/fail results.
3. Remaining limitations, unverified device assumptions, security concerns.
4. Next recommended milestone.

## Suggested first agent task

> Read README.md and AGENTS.md. Bootstrap Go backend, standalone SQLite, .env.example and Makefile. Then implement `snmp-debug` with SNMP v2c/v3 GET/WALK, strict timeout, safe credential handling, JSON export and mocked tests. Add generic Printer-MIB marker life/unit WALK with index matching. Do not implement UI, agent/USB, or unverified vendor-specific OIDs yet. Run tests and report results.
