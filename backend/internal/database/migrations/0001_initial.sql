CREATE TABLE devices (
    id TEXT PRIMARY KEY NOT NULL,
    asset_code TEXT UNIQUE,
    display_name TEXT NOT NULL,
    manufacturer TEXT,
    model TEXT,
    serial TEXT,
    sys_object_id TEXT,
    status TEXT NOT NULL DEFAULT 'unknown',
    first_seen_at TEXT,
    last_seen_at TEXT,
    deleted_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE device_identifiers (
    device_id TEXT NOT NULL REFERENCES devices(id),
    kind TEXT NOT NULL,
    value TEXT NOT NULL,
    source TEXT NOT NULL,
    confidence TEXT NOT NULL,
    UNIQUE(device_id, kind, value)
);

CREATE TABLE device_endpoints (
    id TEXT PRIMARY KEY NOT NULL,
    device_id TEXT NOT NULL REFERENCES devices(id),
    address TEXT NOT NULL,
    protocol TEXT NOT NULL,
    port INTEGER NOT NULL,
    credential_id TEXT,
    is_primary INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1)),
    last_success_at TEXT
);

CREATE TABLE snmp_credentials (
    id TEXT PRIMARY KEY NOT NULL,
    version TEXT NOT NULL CHECK (version IN ('2c', '3')),
    encrypted_secret_material BLOB NOT NULL,
    security_metadata TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE snmp_profiles (
    id TEXT PRIMARY KEY NOT NULL,
    profile_key TEXT NOT NULL,
    version INTEGER NOT NULL,
    schema_version INTEGER NOT NULL,
    content TEXT NOT NULL,
    checksum TEXT NOT NULL,
    verification_status TEXT NOT NULL,
    UNIQUE(profile_key, version)
);

CREATE TABLE device_profile_assignments (
    device_id TEXT PRIMARY KEY NOT NULL REFERENCES devices(id),
    profile_id TEXT NOT NULL REFERENCES snmp_profiles(id),
    assigned_at TEXT NOT NULL,
    explicit INTEGER NOT NULL DEFAULT 0 CHECK (explicit IN (0, 1))
);

CREATE TABLE polling_runs (
    id TEXT PRIMARY KEY NOT NULL,
    device_id TEXT NOT NULL REFERENCES devices(id),
    job_kind TEXT NOT NULL,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    result TEXT NOT NULL,
    error_code TEXT,
    profile_version INTEGER,
    attempt_count INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE counter_definitions (
    id TEXT PRIMARY KEY NOT NULL,
    device_id TEXT NOT NULL REFERENCES devices(id),
    key TEXT NOT NULL,
    source_protocol TEXT NOT NULL,
    oid TEXT NOT NULL,
    instance TEXT,
    unit TEXT NOT NULL,
    semantic_type TEXT NOT NULL,
    scope TEXT NOT NULL,
    verified INTEGER NOT NULL DEFAULT 0 CHECK (verified IN (0, 1)),
    UNIQUE(device_id, key, source_protocol, oid, instance)
);

CREATE TABLE counter_readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    poll_run_id TEXT NOT NULL REFERENCES polling_runs(id),
    counter_definition_id TEXT NOT NULL REFERENCES counter_definitions(id),
    raw_value INTEGER NOT NULL,
    collected_at TEXT NOT NULL,
    quality TEXT NOT NULL,
    UNIQUE(poll_run_id, counter_definition_id)
);

CREATE TABLE counter_epochs (
    id TEXT PRIMARY KEY NOT NULL,
    counter_definition_id TEXT NOT NULL REFERENCES counter_definitions(id),
    started_at TEXT NOT NULL,
    ended_at TEXT,
    reason TEXT NOT NULL,
    verified INTEGER NOT NULL DEFAULT 0 CHECK (verified IN (0, 1))
);

CREATE TABLE counter_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL REFERENCES devices(id),
    counter_definition_id TEXT REFERENCES counter_definitions(id),
    event_type TEXT NOT NULL,
    details TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE daily_counter_usage (
    device_id TEXT NOT NULL REFERENCES devices(id),
    counter_definition_id TEXT NOT NULL REFERENCES counter_definitions(id),
    local_date TEXT NOT NULL,
    delta INTEGER NOT NULL,
    quality TEXT NOT NULL,
    first_reading_id INTEGER REFERENCES counter_readings(id),
    last_reading_id INTEGER REFERENCES counter_readings(id),
    computed_at TEXT NOT NULL,
    PRIMARY KEY(device_id, counter_definition_id, local_date)
);

CREATE INDEX idx_polling_runs_device_started ON polling_runs(device_id, started_at DESC);
CREATE INDEX idx_counter_readings_definition_collected ON counter_readings(counter_definition_id, collected_at DESC);
