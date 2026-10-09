CREATE TABLE poll_leases (
    device_id TEXT NOT NULL REFERENCES devices(id),
    job_kind TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    acquired_at TEXT NOT NULL,
    lease_until TEXT NOT NULL,
    PRIMARY KEY (device_id, job_kind)
);

CREATE INDEX idx_poll_leases_until ON poll_leases(lease_until);
