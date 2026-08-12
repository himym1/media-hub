CREATE TABLE archive_plans (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payload_token TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('awaiting_confirmation', 'queued', 'running', 'completed', 'failed', 'needs_attention')),
    step_index INTEGER NOT NULL DEFAULT 0,
    step_total INTEGER NOT NULL,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX archive_plans_state_idx ON archive_plans(state, created_at);

CREATE TABLE archive_plan_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id TEXT NOT NULL REFERENCES archive_plans(id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX archive_plan_events_plan_idx ON archive_plan_events(plan_id, id);
