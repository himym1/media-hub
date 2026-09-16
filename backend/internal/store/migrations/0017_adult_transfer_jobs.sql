CREATE TABLE transfer_jobs_v17 (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    request_hash BLOB NOT NULL,
    selection_token TEXT NOT NULL,
    source_id TEXT NOT NULL,
    candidate_id TEXT NOT NULL,
    title TEXT NOT NULL,
    year INTEGER NOT NULL DEFAULT 0,
    season INTEGER NOT NULL DEFAULT 0 CHECK (season BETWEEN 0 AND 100),
    episode_start INTEGER NOT NULL DEFAULT 0,
    episode_end INTEGER NOT NULL DEFAULT 0,
    media_type TEXT NOT NULL CHECK (media_type IN ('movie', 'series', 'adult')),
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
    archived_at INTEGER NOT NULL DEFAULT 0,
    UNIQUE (user_id, idempotency_key)
);

INSERT INTO transfer_jobs_v17 (
    id, user_id, idempotency_key, request_hash, selection_token,
    source_id, candidate_id, title, year, season, episode_start, episode_end,
    media_type, tmdb_id, state, resume_state, provider_token, emby_item_id,
    attempts, error_code, error_message, retryable, next_attempt_at,
    created_at, updated_at, archived_at
)
SELECT
    id, user_id, idempotency_key, request_hash, selection_token,
    source_id, candidate_id, title, year, season, episode_start, episode_end,
    media_type, tmdb_id, state, resume_state, provider_token, emby_item_id,
    attempts, error_code, error_message, retryable, next_attempt_at,
    created_at, updated_at, archived_at
FROM transfer_jobs;

DROP TABLE transfer_jobs;
ALTER TABLE transfer_jobs_v17 RENAME TO transfer_jobs;

CREATE INDEX idx_transfer_jobs_runnable
    ON transfer_jobs(state, next_attempt_at, created_at);
CREATE INDEX idx_transfer_jobs_user_created
    ON transfer_jobs(user_id, created_at DESC);
CREATE INDEX idx_transfer_jobs_identity
    ON transfer_jobs(user_id, media_type, tmdb_id, season, created_at DESC);
CREATE INDEX idx_transfer_jobs_user_archived_created
    ON transfer_jobs(user_id, archived_at, created_at DESC);
