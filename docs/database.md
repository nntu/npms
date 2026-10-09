# Local database

NPMS uses SQLite for the standalone v1 deployment. No PostgreSQL server or
Docker container is required.

```bash
go run ./backend/cmd/init --config ./config.yaml --template ./config.example.yaml
make migrate-up
```

On Windows PowerShell:

```powershell
Set-Location backend
go run ./cmd/db-migrate --config ..\config.yaml
```

The default database is `./data/npms.db`; set `database.path` in `config.yaml`
to use another location. The application enables
foreign keys, WAL mode and a bounded busy timeout. Credentials remain encrypted
application data and must never be stored as plaintext in the SQLite file.

Set `security.encryption_key` in the permission-protected `config.yaml` to a
32-byte base64 or 64-character hex key. SNMP community strings and v3 passphrases are encrypted with AES-GCM
before insertion into `snmp_credentials`; the key is never stored in SQLite.

Migrations are embedded in the Go binary and tracked in `schema_migrations`.
The `jobs` table persists asynchronous manual-poll status so job lookup remains
available after an API restart.
The `poll_leases` table prevents API and worker processes from polling the same
device and job kind concurrently. Expired leases may be taken over safely;
owners must release only their own lease.
The initial schema stores UTC timestamps as ISO-8601 text and uses SQLite
`TEXT` IDs so the domain remains transport/database neutral.
When a printer is registered with a loaded profile, the profile snapshot,
checksum, device assignment and counter definitions are committed in the same
transaction as the device, credential and endpoint. A profile with the same
key/version but a different checksum is rejected until its version is bumped.

Cartridge replacement logs also store counter quality and capture time. The API
reads the configured marker counter over SNMP before a replacement when
possible; a manual page count is retained as `unverified`, and an SNMP failure
is recorded as `unavailable` rather than as zero.
The cartridge catalog keeps refill-bottle stock separately from new, refilled,
and empty cartridge stock. `refill-printer` consumes refill bottles and logs
the target device; the existing `refill` endpoint remains the workflow for
turning empty cartridges into refilled cartridges in inventory.
