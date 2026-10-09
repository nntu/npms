CREATE TABLE cartridges (
    id TEXT PRIMARY KEY NOT NULL,
    sku_code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    compatible_models TEXT,
    stock_new INTEGER NOT NULL DEFAULT 0,
    stock_refilled INTEGER NOT NULL DEFAULT 0,
    stock_empty INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE cartridge_logs (
    id TEXT PRIMARY KEY NOT NULL,
    cartridge_id TEXT NOT NULL REFERENCES cartridges(id),
    device_id TEXT REFERENCES devices(id),
    action_type TEXT NOT NULL CHECK (action_type IN ('import', 'replace', 'refill', 'discard')),
    source_type TEXT CHECK (source_type IN ('new', 'refilled')),
    quantity INTEGER NOT NULL DEFAULT 1,
    page_count INTEGER DEFAULT 0,
    notes TEXT,
    performed_at TEXT NOT NULL
);

CREATE INDEX idx_cartridge_logs_device ON cartridge_logs(device_id, performed_at DESC);
CREATE INDEX idx_cartridge_logs_cartridge ON cartridge_logs(cartridge_id, performed_at DESC);
