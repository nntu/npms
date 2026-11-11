ALTER TABLE counter_definitions
ADD COLUMN capability_state TEXT NOT NULL DEFAULT 'unverified'
CHECK (capability_state IN ('valid', 'unverified', 'unsupported', 'unavailable', 'suspicious'));

UPDATE counter_definitions
SET capability_state = 'valid'
WHERE verified = 1;
