# NPMS — Network Printer Management System

> Status: implementation specification v1.0 | Scope: SNMP-first LAN monitoring | Language: Vietnamese

## 1. Objective

### Implementation status

The repository is being implemented incrementally. The first slices are the
`snmp-debug` diagnostic CLI, generic Printer-MIB marker decoding, the versioned
profile loader/resolver, SQLite persistence, worker polling and the initial
typed frontend registry screen. API authentication, full reporting and
frontend mutation flows remain pending until their acceptance criteria and
tests are implemented.

Build a self-hosted system for **fewer than 50 network printers** in a LAN. Automatically discover devices, identify manufacturer/model/serial, collect reliable lifetime counters using SNMP, retain historical readings, calculate usage, and display monitoring and reports.

Initial test fleet: HP LaserJet Enterprise M501dn, HP LaserJet Pro M404dn, Brother HL-L5100DN (model to verify), and Ricoh IM 2xxx (exact model to verify). Design must accommodate other manufacturers and models through versioned profiles.

**v1 in scope:** device registry with department tracking, toner/cartridge inventory management (new stock, refilled stock, empty shells, replace & refill workflows), SNMP v2c/v3 credentials, bounded discovery, generic/vendor profiles, diagnostic CLI, polling, raw readings, counter validation, online/offline/unknown monitoring, toner where available, daily/monthly usage reports, authenticated REST API, React dashboard, standalone SQLite operation.

**Out of scope for v1:** Windows/Linux agent, USB, CUPS, IPP, print-job accounting, print server, print quotas, billing, multi-site distributed collector. Future extensions must not require replacing device IDs or counter schema.

## 2. Technology and topology

- Go backend (single Go module); standard `net/http`, GoSNMP, `database/sql` + SQLite, embedded SQL migrations, slog.
- SQLite file database, React + TypeScript + Vite, TanStack Query.
- API and worker run as local executables; no Docker, PostgreSQL server or message broker is required for standalone v1.
- Modular monolith: separate API and worker processes share domain/application packages; **no Redis, Kafka, RabbitMQ or Kubernetes**.
- Server worker talks directly to LAN printers via UDP/161. Database is private. Expose HTTPS only via configured reverse proxy.
- Poll counter every 15 minutes, status every 5 minutes, discovery on demand; max 5 concurrent device polls initially; jitter, timeout and bounded retry.

```text
Browser -> React -> HTTPS API -> Application Services -> SQLite file
                                      ^
                            Worker / Scheduler
                                      |
                       SNMP Transport + Profiles
                                      |
                          LAN printers (UDP 161)
```

## 3. Domain boundaries

1. **Device Registry**: stable UUID identity, asset name, serial/model, locations, connection endpoints, credentials references, assigned profile, last seen.
2. **Discovery**: authorized subnet/IP scan, SNMP probe, identify, match candidates, operator approval to import; do not automatically merge by model or IP.
3. **SNMP Transport**: Get/Walk/BulkWalk with typed results, timeouts, cancellation, v2c/v3 support; no business logic.
4. **Profile Engine**: versioned declarative YAML, matching and resolution, OID mappings, instance selection, unit validation, model-specific decoders via Go interfaces when required.
5. **Counter Engine**: typed observations, validation, raw persistence, reset/wrap/anomaly events, usage calculation with data quality flags.
6. **Polling/Monitoring**: bounded scheduler, durable poll runs, retries, last-success and health; SNMP timeout != definitive device power-off.
7. **Reporting**: query counter histories and materialized daily aggregates, time-zone-aware dates, CSV export (XLSX later).
8. **API/UI**: authenticated management, diagnosis and profile test, audit changes.

### Future-proofing (without implementing agents)

`device` is not an IP address. Keep `device_endpoints` and `counter_definitions` independent of SNMP. Normalize collector output into a transport-neutral `Observation` DTO containing source, counter semantic, unit, scope, raw value, time and quality. Do **not** build agent enrollment, agent endpoints, USB collectors or distributed messaging in v1.

## 4. Counter semantics and SNMP

SNMP standard OIDs (column OIDs require WALK and correct instance indexing):

| Metric | OID |
|---|---|
| sysDescr | `1.3.6.1.2.1.1.1.0` |
| sysObjectID | `1.3.6.1.2.1.1.2.0` |
| sysName | `1.3.6.1.2.1.1.5.0` |
| prtMarkerCounterUnit | `1.3.6.1.2.1.43.10.2.1.3` |
| prtMarkerLifeCount | `1.3.6.1.2.1.43.10.2.1.4` |
| prtGeneralSerialNumber | `1.3.6.1.2.1.43.5.1.1.17` |
| prtMarkerSuppliesLevel | `1.3.6.1.2.1.43.11.1.1.9` |
| prtMarkerSuppliesMaxCapacity | `1.3.6.1.2.1.43.11.1.1.8` |

**Never assume:** the first marker is total pages; all marker rows can be added; impressions = physical sheets; total = print + copy; a private OID works on every firmware; an unavailable metric is zero. Check `prtMarkerCounterUnit` and exact index for every marker. A marker life counter is not necessarily a sheet counter. Vendor-specific OIDs are experimental until verified against device panel/configuration page.

Counter keys: `total_impressions`, `mono_impressions`, `color_impressions`, `print_impressions`, `copy_impressions`, `total_sheets`, `scan_pages` (only emit when supported and verified). Preserve original OID, instance, raw type, unit and scope. Separate *capability not supported*, *poll failed*, and *value zero*.

For HP M501dn/M404dn start with generic Printer-MIB. For Brother HL-L5100DN start with generic and allow an opt-in vendor decoder after fixture validation. For Ricoh IM 2xxx start generic and add private profile only after exact model and real SNMP sample are obtained. Canon support through generic profile first.

### Profile schema (illustrative)

```yaml
schema_version: 1
id: generic-printer-mib
version: 1
manufacturer: generic
match:
  sys_object_id_prefixes: []
  model_patterns: []
counters:
  marker_life:
    value_column: "1.3.6.1.2.1.43.10.2.1.4"
    unit_column: "1.3.6.1.2.1.43.10.2.1.3"
    mode: walk
    selection: validated_marker_rows
    aggregation: none
    semantic_type: marker_life
    require_unit_validation: true
```

Profile precedence: explicit assignment > exact sysObjectID > vendor/model match > vendor generic > generic Printer-MIB. Ambiguous equal-priority matches must be surfaced, never randomly selected. Schema validation and versioning are mandatory. Profiles must not execute arbitrary code.

## 5. Database model

Use embedded SQLite SQL migrations and foreign keys. Core tables:

- `devices`: UUID PK, asset_code, display_name, manufacturer, model, serial, sys_object_id, location_id, status, first_seen_at, last_seen_at, created_at, updated_at.
- `device_identifiers`: device_id, kind, value, source, confidence, unique only where justified.
- `device_endpoints`: device_id, address (normalized IP/hostname TEXT for SNMP), protocol, port, credential_id, is_primary, last_success_at.
- `snmp_credentials`: UUID, version, encrypted secret material, security metadata; **never plaintext**.
- `snmp_profiles`: id, profile_key, version, schema version, content/checksum, verification status.
- `device_profile_assignments`: device_id, profile_id, assigned_at, explicit flag.
- `polling_runs`: UUID, device_id, job kind, start/end, result, error code, profile version, attempt count.
- `counter_definitions`: UUID, device_id, key, source protocol/OID/instance, unit, semantic_type, scope, verified flag.
- `counter_readings`: INTEGER identity PK, poll_run_id, counter_definition_id, raw_value INTEGER, collected_at, quality; unique `(poll_run_id, counter_definition_id)`.
- `counter_epochs`: counter_definition_id, start/end readings, reason and verification.
- `counter_events`: device_id, counter_definition_id, event_type, details JSON text, created_at.
- `daily_counter_usage`: device_id, counter_definition_id, local_date, delta, quality, first/last readings, computed_at; composite unique key.
- `locations`, `users`, `audit_logs`.

Indexes: `(counter_definition_id, collected_at DESC)`, `(device_id, started_at DESC)` on polling runs, unique asset_code when present, sensible endpoint uniqueness. Use UTC timestamps and explicit report timezone (`Asia/Ho_Chi_Minh` default). Retention cleanup removes only unreferenced old poll runs, old counter events and completed jobs; raw readings and daily usage remain available for audit.

### Counter algorithm

- Same counter epoch, monotonically increasing: `delta = current - previous` subject to plausible threshold checks.
- Same value: delta 0.
- Decrease: emit `reset_suspected` or `wrap_suspected`; start a new epoch only according to explicit policy; never produce negative usage or silently add current value.
- Missing/timeout: record failed poll, **do not insert zero reading**.
- Large spike: mark `suspicious`, exclude from trusted usage pending review.
- Daily usage spanning midnight: mark allocation as `estimated`; do not claim exact calendar-day usage from 15-minute snapshots.
- Preserve raw readings; aggregates must be reproducible and idempotently recalculated.

## 6. API v1

Prefix `/api/v1`, JSON, UUID IDs, UTC ISO-8601, pagination and structured errors (`code`, `message`, `details`, `request_id`). Authenticate all non-health routes, authorize write/admin operations.

| Method | Route | Purpose |
|---|---|---|
| GET/POST | `/printers` | List/create devices |
| GET/PATCH/DELETE | `/printers/{id}` | Read/update/soft-delete |
| POST | `/printers/{id}/poll` | Enqueue poll, return 202/job ID |
| GET | `/printers/{id}/counters` | Latest values with quality |
| GET | `/printers/{id}/readings` | Paginated raw history |
| GET | `/printers/{id}/polling-runs` | Poll diagnostics |
| POST | `/discovery/jobs` | Scan allowlisted subnet(s), return 202 |
| GET | `/discovery/jobs/{id}` | Job progress |
| GET | `/discovery/jobs/{id}/results` | Candidates |
| POST | `/discovery/import` | Import selected candidates |
| GET/POST | `/snmp/profiles` | List/create profiles |
| POST | `/snmp/profiles/{id}/validate` | Schema validation |
| POST | `/printers/{id}/profile-test` | Test profile without persisting counters |
| GET | `/reports/usage/daily` | Daily usage with quality |
| GET | `/reports/usage/monthly` | Monthly usage with quality |
| GET | `/reports/usage/export` | CSV export |
| GET | `/health/live` | Process liveness |
| GET | `/health/ready` | Readiness incl. DB |

Provide an OpenAPI 3.1 specification in `docs/openapi.yaml`. All async job endpoints must expose job status and idempotency safeguards.

The initial API slice is available through `cmd/api`: health endpoints and the
authenticated, paginated `GET /api/v1/printers` registry route. Remaining CRUD,
poll jobs, discovery, profiles and reports are still pending.

## 7. Repository layout

```text
npms/
  backend/
    cmd/{api,worker,snmp-debug}/
    internal/{auth,device,discovery,snmp,profile,counter,polling,monitoring,reporting,ingestion,database}/
    profiles/{generic,hp,brother,ricoh,canon}/
    internal/database/migrations/
    tests/fixtures/
    go.mod
  frontend/src/{features,shared}/
  docs/{architecture.md,database.md,snmp-profiles.md,openapi.yaml}/
  config.example.yaml
  data/ (runtime, ignored)
  AGENTS.md
  README.md
```

## 8. Build milestones and acceptance

1. **SNMP diagnostic CLI**: `snmp-debug probe --host ...`, `walk --host ... --oid ...`, `export --host ... --output ...`; redact credentials; JSON output with raw OID/instance/type/unit; fixture-based tests.
2. **Profile Engine**: generic YAML schema, matching, validation, profile tests, verified fixture for each supported model.
3. **Database + Counter Engine**: migrations, SQLite repositories, ingestion, deduplication, reset/spike tests, daily aggregation quality flags. Counter validation remains independent of SQLite.
4. **API + Worker**: CRUD, bounded discovery, durable polling, manual poll, authentication, OpenAPI. The worker loop, bounded scheduler and polling service are implemented independently of the SNMP transport; the SNMP reader lives in ingestion. The current worker executable only performs registry cycles until endpoint/credential resolution is connected.
5. **Frontend + operations**: devices, status, counter history, discovery review, profile diagnostics, reports, standalone SQLite backup instructions.

**Definition of Done:** `go test ./...`, frontend typecheck/build and lint, SQLite migration idempotency test, no plaintext secrets, integration tests with fake SNMP devices, and manual confirmation of counter units/values against real printer configuration pages before marking model profile `verified`.

## 9. Local development

Prerequisites: current supported Go and Node.js 20.19+ LTS. The runtime is designed for
Linux and Windows: SQLite uses a pure-Go driver, paths are handled with Go's
portable filesystem APIs, and no Docker/CGO service is required.

On Linux/macOS, the convenience commands are:

```bash
make init-config # Generates config.yaml with secure 32-byte encryption key
make migrate-up
make test
make lint
```

On Windows PowerShell, run the equivalent commands directly:

```powershell
Set-Location backend
go run ./cmd/db-migrate --config ..\config.yaml
go test ./...
go fmt ./...
go vet ./...
```

The database commands run locally against SQLite; no container is started. Keep
`config.yaml` local and never commit real community strings, SNMPv3 keys,
encryption keys or API tokens.
Error-level diagnostics are written as JSON lines to `logging.error_file`
(default `./data/npms-errors.log`) with restrictive file permissions. With
`logging.daily: true`, files are suffixed by UTC date and split into
`.part-001`, `.part-002` when `logging.max_size_mb` is exceeded. Secrets are
never included in this file.

## 9.1. Build and deploy on Linux

From a clean checkout with Go and Node.js 20.19+ LTS:

```bash
cp config.example.yaml config.yaml
mkdir -p bin
cd backend
go mod download
go build -trimpath -ldflags="-s -w" -o ../bin/npms-api ./cmd/api
go build -trimpath -ldflags="-s -w" -o ../bin/npms-worker ./cmd/worker
go build -trimpath -ldflags="-s -w" -o ../bin/npms-db-migrate ./cmd/db-migrate
cd ../frontend
npm ci
npm run build
cd ..
./bin/npms-db-migrate --config ./config.yaml
./bin/npms-api --config ./config.yaml
```

Run the worker as a second process: `./bin/npms-worker --config ./config.yaml`.
Serve `frontend/dist` through an approved local web server or reverse proxy.
Keep the API on localhost unless LAN access is explicitly required. Store
secrets in the permission-protected `config.yaml`. Stop the worker before
backing up `data/npms.db`.

## 9.2. Build and deploy on Windows

Use PowerShell from the repository root. Go produces native `.exe` files and
does not require a compiler or CGo runtime on the target machine:

```powershell
Copy-Item .\config.example.yaml .\config.yaml
New-Item -ItemType Directory -Force .\bin | Out-Null
Set-Location backend
go mod download
go build -trimpath -ldflags="-s -w" -o ..\bin\npms-api.exe .\cmd\api
go build -trimpath -ldflags="-s -w" -o ..\bin\npms-worker.exe .\cmd\worker
go build -trimpath -ldflags="-s -w" -o ..\bin\npms-db-migrate.exe .\cmd\db-migrate
Set-Location ..\frontend
npm ci
npm run build
Set-Location ..
.\bin\npms-db-migrate.exe --config ..\config.yaml
.\bin\npms-api.exe --config ..\config.yaml
```

Run the worker in a second PowerShell window or register both executables as
Windows services with the approved service manager. Allow outbound UDP/161
only to authorized printer IPs. Do not expose the API or SQLite file through a
shared folder. For either OS, set `VITE_API_BASE_URL` before `npm run build`
when the frontend is served from a different origin. Set that value in the
frontend build environment, not in the backend runtime configuration.

## 9.3. Single-binary deployment

For a small standalone installation, use the unified build scripts instead of
the three separate binaries:

```bash
./scripts/build-standalone.sh
cp config.example.yaml config.yaml
./npms --config ./config.yaml
```

On Windows PowerShell:

```powershell
.\scripts\build-standalone.ps1
Copy-Item .\config.example.yaml .\config.yaml
.\npms.exe --config .\config.yaml
```

The unified binary runs migrations, API, worker and the embedded React
frontend. Open `http://127.0.0.1:8080` after startup. Build on the target OS
(or use a matching Go cross-build); SQLite data and environment secrets are
not embedded in the binary. Unified mode is recommended for one-machine
deployments; separate API/worker services remain preferable when independent
restart and scaling control are required.

## 10. Security and operational requirements

- Prefer SNMPv3 authPriv; SNMPv2c read-only on trusted LAN when necessary.
- Encrypt credentials at rest; key is supplied by the permission-protected
  `security.encryption_key` field in `config.yaml`, never stored in the DB.
- Allowlist discovery targets and rate-limit scanning. No arbitrary Internet scanning.
- Bounded polling concurrency, context cancellation, timeout, retries with backoff.
- Restrict DB access, authenticated APIs, audit mutations, sanitize logs.
- Health/readiness endpoints, structured logs, backup/restore documentation.
- Device SNMP errors should not block polling other devices.

## 11. Open questions requiring real-device evidence

- Confirm exact Brother HL-L5100DN label and Ricoh IM model numbers.
- Capture SNMP WALK fixture from each model, including marker units and indexes.
- Verify whether Ricoh exposes separate print/copy counters and whether their semantics match device panel totals.
- Confirm subnet(s), SNMP version/credentials and deployment host network reachability.

**Implementation rule:** When evidence is missing, implement generic behavior with `unverified` status; never invent vendor OIDs or mark counters verified.
