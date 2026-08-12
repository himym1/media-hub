CREATE TABLE local_upload_jobs (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    request_hash BLOB NOT NULL,
    payload_token TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued', 'hashing', 'submitting_init', 'uploading', 'completed', 'failed', 'needs_attention')),
    bytes_done INTEGER NOT NULL DEFAULT 0,
    bytes_total INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE(user_id, idempotency_key)
);

CREATE INDEX local_upload_jobs_state_idx ON local_upload_jobs(state, created_at);

CREATE TABLE local_upload_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL REFERENCES local_upload_jobs(id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX local_upload_events_job_idx ON local_upload_events(job_id, id);
