# Supported local platforms

NPMS v1 is designed to run as a local Go process on Linux or Windows.

## Portability rules

- SQLite uses the pure-Go `modernc.org/sqlite` driver, so the database does not
  require a PostgreSQL server, Docker or a separately installed SQLite library.
- Database directories are created with `os.MkdirAll`; paths are joined with
  `filepath`, not hard-coded with `/` or `\\`.
- SNMP uses Go UDP networking and supports the same v2c/v3 behavior on both OSes.
- Runtime database files belong under `data/` and are excluded from source
  control. Back up the `.db` file while the worker is stopped.
- Make is a Linux/macOS convenience only. Windows users can run the equivalent
  `go run`, `go test`, `go fmt` and `go vet` commands from PowerShell.

## Windows operation

```powershell
Copy-Item ..\config.example.yaml ..\config.yaml
Set-Location backend
go run ./cmd/db-migrate --config ..\config.yaml
go run ./cmd/snmp-debug probe --host 192.0.2.10 --version 2c --community $env:SNMP_COMMUNITY
go test ./...
```

Use Windows Firewall rules to allow outbound UDP/161 to the explicitly
authorized printer subnet. Do not expose the diagnostic or future management
API directly to an untrusted network.

## Production build layout

Build `cmd/api`, `cmd/worker` and `cmd/db-migrate` as native binaries with
`go build -trimpath -ldflags="-s -w"`. Build the React application with
`npm ci && npm run build`; deploy only `frontend/dist` to the local web server.
The API and worker run as separate processes using the same `database.path` from
`config.yaml`.
Run the migration binary before first start and keep SQLite on local disk.
Linux should use systemd or the site's service manager; Windows should use the
approved Windows service manager. Store secrets in the permission-protected
`config.yaml`, never in the repository.

For one-machine deployments, `scripts/build-standalone.sh` or
`scripts/build-standalone.ps1` builds a single `npms`/`npms.exe`. It embeds the
React assets, runs SQLite migrations at startup, starts the API and worker, and
serves the UI from the same HTTP listener. Profiles remain external YAML files:
ship the configured `profiles.path` directory beside the deployment and update
that path when moving outside the source tree. Build on the target OS, keep the
SQLite file outside the binary, and run the binary as the platform service.
