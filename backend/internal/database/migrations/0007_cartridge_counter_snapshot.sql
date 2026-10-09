ALTER TABLE cartridge_logs ADD COLUMN counter_quality TEXT NOT NULL DEFAULT 'unavailable';
ALTER TABLE cartridge_logs ADD COLUMN counter_collected_at TEXT;

