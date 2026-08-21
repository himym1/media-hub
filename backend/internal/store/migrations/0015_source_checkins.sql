CREATE TABLE source_checkins (
    source_id TEXT PRIMARY KEY,
    label TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    retryable INTEGER NOT NULL DEFAULT 0,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at INTEGER NOT NULL DEFAULT 0,
    last_success_at INTEGER NOT NULL DEFAULT 0,
    last_day TEXT NOT NULL DEFAULT '',
    notified_day TEXT NOT NULL DEFAULT '',
    notified_state TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL
);
