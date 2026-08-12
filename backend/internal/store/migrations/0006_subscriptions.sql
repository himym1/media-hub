ALTER TABLE transfer_jobs
    ADD COLUMN season INTEGER NOT NULL DEFAULT 0 CHECK (season BETWEEN 0 AND 100);

CREATE INDEX idx_transfer_jobs_identity
    ON transfer_jobs(user_id, media_type, tmdb_id, season, created_at DESC);

CREATE TABLE subscriptions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tmdb_id TEXT NOT NULL,
    title TEXT NOT NULL,
    original_title TEXT NOT NULL DEFAULT '',
    year INTEGER NOT NULL DEFAULT 0 CHECK (year BETWEEN 0 AND 2100),
    media_type TEXT NOT NULL CHECK (media_type IN ('movie', 'series')),
    season INTEGER NOT NULL DEFAULT 0 CHECK (season BETWEEN 0 AND 100),
    policy TEXT NOT NULL CHECK (policy IN ('once', 'upgrade')),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    interval_minutes INTEGER NOT NULL CHECK (interval_minutes BETWEEN 15 AND 10080),
    source_ids_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(source_ids_json)),
    preferences_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(preferences_json)),
    next_run_at INTEGER NOT NULL,
    last_run_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (user_id, tmdb_id, media_type, season)
);

CREATE INDEX idx_subscriptions_user_created
    ON subscriptions(user_id, created_at DESC);
CREATE INDEX idx_subscriptions_due
    ON subscriptions(enabled, next_run_at, created_at);

CREATE TABLE subscription_runs (
    id TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    trigger_type TEXT NOT NULL CHECK (trigger_type IN ('scheduled', 'manual')),
    state TEXT NOT NULL CHECK (state IN (
        'queued', 'searching', 'retry_wait', 'no_match', 'duplicate',
        'enqueued', 'completed', 'failed', 'needs_attention'
    )),
    resume_state TEXT NOT NULL DEFAULT '',
    source_id TEXT NOT NULL DEFAULT '',
    candidate_fingerprint TEXT NOT NULL DEFAULT '',
    transfer_job_id TEXT REFERENCES transfer_jobs(id) ON DELETE SET NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    retryable INTEGER NOT NULL DEFAULT 0 CHECK (retryable IN (0, 1)),
    next_attempt_at INTEGER NOT NULL DEFAULT 0,
    started_at INTEGER NOT NULL,
    finished_at INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL
);

CREATE INDEX idx_subscription_runs_runnable
    ON subscription_runs(state, next_attempt_at, started_at);
CREATE INDEX idx_subscription_runs_subscription
    ON subscription_runs(subscription_id, started_at DESC);
CREATE INDEX idx_subscription_runs_fingerprint
    ON subscription_runs(subscription_id, candidate_fingerprint, state);
