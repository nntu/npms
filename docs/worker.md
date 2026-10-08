# Worker

The worker executable is portable across Linux and Windows and uses SQLite as
its local registry. It supports separate status and counter intervals and a
`--once` mode for startup/registry checks:

```text
cp config.example.yaml config.yaml
go run ./cmd/worker --config ./config.yaml --once
go run ./cmd/worker --config ./config.yaml
```

On PowerShell, copy `config.example.yaml` to `config.yaml`, set
`security.encryption_key`, then start the worker with `--config .\config.yaml`.

The status cycle resolves encrypted endpoint credentials, connects through SNMP,
reads `sysName`, and updates endpoint `last_success_at`. A failed device does
not stop other devices. The counter cycle reads only counter definitions already
stored in SQLite, persists immutable raw readings, and marks definitions as
`valid` or `unverified`. Delta/reset/spike evaluation remains in Counter Engine.
