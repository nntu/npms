# Local database

NPMS uses SQLite for the standalone v1 deployment. No PostgreSQL server or
Docker container is required.

```bash
cp config.example.yaml config.yaml
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
The initial schema stores UTC timestamps as ISO-8601 text and uses SQLite
`TEXT` IDs so the domain remains transport/database neutral.
