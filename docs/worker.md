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
not stop other devices. Printer registration resolves the selected profile and
stores its counter definitions before polling begins. The counter cycle reads
those definitions, persists immutable raw readings, and marks definitions as
`valid` or `unverified`. GET definitions use one OID; WALK definitions join the
value and unit columns by identical instance and reject ambiguous multi-row
results until an explicit selection rule exists. Delta/reset/spike evaluation
remains in Counter Engine.

Status and counter cycles acquire SQLite poll leases per device. If another API
request or worker already owns the same lease, the cycle skips that device and
continues with the remaining devices.
