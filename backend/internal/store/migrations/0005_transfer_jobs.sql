CREATE TABLE transfer_jobs (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    request_hash BLOB NOT NULL,
    selection_token TEXT NOT NULL,
    source_id TEXT NOT NULL,
    candidate_id TEXT NOT NULL,
    title TEXT NOT NULL,
    year INTEGER NOT NULL DEFAULT 0,
    media_type TEXT NOT NULL CHECK (media_type IN ('movie', 'series')),
    tmdb_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN (
        'queued', 'transferring', 'retry_wait', 'transferred',
        'submitting_sync', 'syncing', 'refreshing_emby', 'indexing_emby',
        'verifying_playback', 'completed', 'failed', 'needs_attention'
    )),
    resume_state TEXT NOT NULL DEFAULT '',
    provider_token TEXT NOT NULL DEFAULT '',
    emby_item_id TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    retryable INTEGER NOT NULL DEFAULT 0 CHECK (retryable IN (0, 1)),
    next_attempt_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX idx_transfer_jobs_runnable
    ON transfer_jobs(state, next_attempt_at, created_at);
CREATE INDEX idx_transfer_jobs_user_created
    ON transfer_jobs(user_id, created_at DESC);

CREATE TABLE transfer_job_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL REFERENCES transfer_jobs(id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE INDEX idx_transfer_job_events_job
    ON transfer_job_events(job_id, id);

CREATE TABLE transfer_notifications (
    job_id TEXT NOT NULL REFERENCES transfer_jobs(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('completed', 'failed', 'needs_attention')),
    state TEXT NOT NULL CHECK (state IN ('pending', 'submitting', 'sent', 'needs_attention')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (job_id, event_type)
);

CREATE INDEX idx_transfer_notifications_runnable
    ON transfer_notifications(state, next_attempt_at, created_at);
