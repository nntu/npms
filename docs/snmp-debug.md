# `snmp-debug`

The first NPMS vertical slice is a diagnostic CLI. It supports SNMP v2c and
v3 GET/WALK operations and does not persist credentials or readings.

Linux/macOS examples:

```bash
cd backend
go run ./cmd/snmp-debug probe --host 192.0.2.10 --version 2c --community "$SNMP_COMMUNITY"
go run ./cmd/snmp-debug walk --host 192.0.2.10 --version 2c --community "$SNMP_COMMUNITY" --oid 1.3.6.1.2.1.43.10.2.1.4
go run ./cmd/snmp-debug export --host 192.0.2.10 --version 3 --username "$SNMP_USER" --auth-protocol SHA --auth-passphrase "$SNMP_AUTH" --priv-protocol AES --priv-passphrase "$SNMP_PRIV" --output sample.json
```

For a human-readable console check showing the target IP, printer identity and
current validated marker values:

```bash
go run ./cmd/snmp-debug check --host 192.168.1.20 --version 2c --community public
```

The `check` command does not write to SQLite. It reports only values returned
by the printer; marker-life values are not automatically interpreted as pages
or physical sheets.

Windows PowerShell example:

```powershell
cd backend
go run ./cmd/snmp-debug probe --host 192.0.2.10 --version 2c --community $env:SNMP_COMMUNITY
```

`probe` reads standard system identifiers and walks the generic Printer-MIB
marker life/unit columns. Marker rows are joined only when their complete table
instance is identical. No marker is interpreted as pages or sheets by this
slice. Real-device fixtures and panel verification are still required before a
profile can be marked `verified`.

The CLI prints raw values but never prints the supplied community string,
username passphrases, or private keys. Do not place secrets directly in shell
history or committed files.

The authenticated API and frontend also provide a single-IP discovery probe at
`POST /api/v1/discovery/probe`. It reads standard system identity OIDs and does
not scan a CIDR, persist credentials, or create a device automatically.
