ALTER TABLE counter_definitions ADD COLUMN mode TEXT NOT NULL DEFAULT 'get' CHECK (mode IN ('get', 'walk'));
ALTER TABLE counter_definitions ADD COLUMN unit_oid TEXT;
ALTER TABLE counter_definitions ADD COLUMN selection TEXT;
ALTER TABLE counter_definitions ADD COLUMN aggregation TEXT;
ALTER TABLE counter_definitions ADD COLUMN require_unit_validation INTEGER NOT NULL DEFAULT 0 CHECK (require_unit_validation IN (0, 1));

CREATE INDEX idx_counter_definitions_device_key ON counter_definitions(device_id, key);
