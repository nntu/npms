# Polling scheduler

The worker scheduler is OS-independent and keeps the LAN scan bounded. It
accepts a target list and a transport-neutral polling function, limits active
polls to the configured concurrency, retries transient failures with capped
exponential backoff, and stops waiting when the context is cancelled.

The scheduler returns one result per target in input order. A failed target does
not stop unrelated devices; the caller persists its polling-run error and moves
on. Duplicate target IDs are rejected so a retry cannot accidentally schedule
the same device twice in one batch.

Counter polling creates every polling-run ledger entry for a cycle in one SQLite
transaction, initially with `attempt_count = 0`. It stores each terminal run
state and its immutable reading or failure event in another atomic transaction.
The final `attempt_count` is the actual number of transport attempts reported by
the scheduler; a target canceled before its first attempt remains zero. Repeating
completion after an uncertain commit is idempotent and does not duplicate
readings or failure events.
