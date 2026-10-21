-- 0009_departments_table.sql: Create departments table and link devices to departments

CREATE TABLE IF NOT EXISTS departments (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE COLLATE NOCASE,
    name TEXT NOT NULL,
    description TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_departments_code ON departments(code);

-- Add department_id to devices
ALTER TABLE devices ADD COLUMN department_id TEXT REFERENCES departments(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_devices_department_id ON devices(department_id);

-- Auto-seed departments from existing distinct non-empty device department values
INSERT OR IGNORE INTO departments (id, code, name, description, created_at, updated_at)
SELECT
    lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-a' || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))),
    UPPER(REPLACE(REPLACE(REPLACE(TRIM(department), 'Phòng ', 'P_'), 'phòng ', 'P_'), ' ', '_')),
    TRIM(department),
    'Được tự động đồng bộ từ thiết bị hiện có',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM devices
WHERE department IS NOT NULL AND TRIM(department) != ''
GROUP BY TRIM(department);

-- Link existing devices to newly seeded departments
UPDATE devices
SET department_id = (
    SELECT d.id FROM departments d 
    WHERE LOWER(d.name) = LOWER(TRIM(devices.department))
    LIMIT 1
)
WHERE department IS NOT NULL AND TRIM(department) != '' AND department_id IS NULL;
