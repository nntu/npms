# Polling scheduler

The worker scheduler is OS-independent and keeps the LAN scan bounded. It
accepts a target list and a transport-neutral polling function, limits active
polls to the configured concurrency, retries transient failures with capped
exponential backoff, and stops waiting when the context is cancelled.

The scheduler returns one result per target in input order. A failed target does
not stop unrelated devices; the caller persists its polling-run error and moves
on. Duplicate target IDs are rejected so a retry cannot accidentally schedule
the same device twice in one batch.
