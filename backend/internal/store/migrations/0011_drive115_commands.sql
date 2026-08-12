CREATE TABLE drive115_command_jobs (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    operation TEXT NOT NULL CHECK (operation IN ('create_folder', 'move', 'rename', 'delete')),
    idempotency_key TEXT NOT NULL,
    request_hash BLOB NOT NULL,
    payload_token TEXT NOT NULL,
    result_token TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('awaiting_confirmation', 'queued', 'submitting', 'completed', 'failed', 'needs_attention')),
    attempts INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(user_id, idempotency_key)
);

CREATE INDEX drive115_command_jobs_state_idx ON drive115_command_jobs(state, created_at);

CREATE TABLE drive115_command_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL REFERENCES drive115_command_jobs(id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX drive115_command_events_job_idx ON drive115_command_events(job_id, id);
