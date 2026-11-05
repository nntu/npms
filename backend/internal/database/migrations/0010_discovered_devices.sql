CREATE TABLE IF NOT EXISTS discovered_devices (
    address TEXT PRIMARY KEY NOT NULL,
    port INTEGER NOT NULL DEFAULT 161,
    name TEXT,
    description TEXT,
    sys_object_id TEXT,
    serial TEXT,
    mac TEXT,
    location TEXT,
    manufacturer TEXT,
    model TEXT,
    device_type TEXT NOT NULL DEFAULT 'printer' CHECK (device_type IN ('printer', 'scanner')),
    matched_profile_id TEXT,
    markers_json TEXT,
    already_registered INTEGER NOT NULL DEFAULT 0 CHECK (already_registered IN (0, 1)),
    status TEXT NOT NULL DEFAULT 'discovered' CHECK (status IN ('discovered', 'registered', 'ignored')),
    discovered_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_discovered_devices_type ON discovered_devices(device_type);
CREATE INDEX IF NOT EXISTS idx_discovered_devices_status ON discovered_devices(status);

ALTER TABLE devices ADD COLUMN device_type TEXT NOT NULL DEFAULT 'printer' CHECK (device_type IN ('printer', 'scanner'));
