CREATE TABLE subx_command_jobs (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    operation_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    request_hash BLOB NOT NULL,
    payload_token TEXT NOT NULL,
    result_token TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('queued', 'submitting', 'completed', 'failed', 'needs_attention')),
    attempts INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    retryable INTEGER NOT NULL DEFAULT 0 CHECK (retryable IN (0, 1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX idx_subx_command_jobs_runnable
    ON subx_command_jobs(state, created_at);
CREATE INDEX idx_subx_command_jobs_user_created
    ON subx_command_jobs(user_id, created_at DESC);

CREATE TABLE subx_command_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL REFERENCES subx_command_jobs(id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE INDEX idx_subx_command_events_job
    ON subx_command_events(job_id, id);
