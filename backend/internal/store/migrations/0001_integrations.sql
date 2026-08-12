CREATE TABLE integrations (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    label TEXT NOT NULL CHECK (length(label) > 0),
    base_url TEXT NOT NULL DEFAULT '',
    secret_ref TEXT,
    health_status TEXT NOT NULL DEFAULT 'unconfigured'
        CHECK (health_status IN ('healthy', 'degraded', 'unavailable', 'unconfigured')),
    health_detail TEXT NOT NULL DEFAULT '',
    last_checked_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
