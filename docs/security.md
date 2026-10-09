# Security Model & Key Management in NPMS

Documenting secret handling, encryption key management, API authentication, and security invariants in the Network Printer Management System (NPMS).

---

## 1. Cryptographic Key Management

NPMS uses **AES-256 GCM** (Galois/Counter Mode) authenticated encryption for storing SNMP credentials at rest in the SQLite database (`snmp_credentials` table).

### 1.1 Key Requirement
- Key Size: **32 bytes** (256 bits).
- Encoding: Base64 (standard, raw unpadded, or URL-safe) or 64-character Hexadecimal string.
- Runtime storage: Loaded from `security.encryption_key` in `config.yaml`.
- File Permissions: `config.yaml` should be created with `0600` (read/write for owner only) permissions.

### 1.2 Key Generation Tools
Generate a new 32-byte key automatically:
```bash
# 1. Using Makefile
make init-config

# 2. Using npms unified executable
./npms init

# 3. Using npms-init standalone CLI
./bin/npms-init --generate-api-token
```

---

## 2. Secrets Encryption at Rest

- **Encrypted Fields**: SNMP v2c community strings, SNMPv3 authentication passphrases, and privacy passphrases.
- **Nonce Handling**: Each encryption operation generates a fresh 12-byte random cryptographic nonce (`crypto/rand`).
- **Tamper Protection**: AES-GCM provides authenticated tag checking; tampered secrets fail decryption deterministically.

---

## 3. API Authentication & Authorization

- **Bearer Token**: Configured via `server.api_token` in `config.yaml`.
- **Enforcement**: When `server.api_token` is set, all management endpoints under `/api/v1` require an `Authorization: Bearer <TOKEN>` header.
- **Localhost Single-Machine Mode**: If `server.api_token` is empty, API requests are intended for single-machine localhost usage.

---

## 4. Credential Redaction & Log Safety

- **Logs & Responses**: Credentials, raw Community strings, and SNMPv3 passphrases are **never** logged to disk, printed to console, or returned in API responses.
- **Diagnostics**: `snmp-debug` redacts secrets from debug outputs and JSON exports.
- **Diagnostics Isolation**: Diagnostic probes (e.g. `POST /api/v1/discovery/probe`) perform read-only identity discovery without storing credentials.

---

## 5. Defense in Depth Invariants

- **No Remote Code Execution**: Printer profile YAML files are purely declarative. Profiles do not execute commands, shell scripts, or dynamically load `.so` plugins.
- **Bounded Scanning**: Polling and discovery are constrained to explicitly configured target IPs or approved subnets with bounded concurrency (default: 5 concurrent workers).
- **Soft Deletion**: Removing a printer does not hard-delete immutable historical counter readings, preserving audit trails.
