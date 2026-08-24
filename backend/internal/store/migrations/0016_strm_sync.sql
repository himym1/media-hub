CREATE TABLE strm_folder_states (
    folder_id TEXT NOT NULL,
    target_path TEXT NOT NULL,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (folder_id, target_path)
);

CREATE TABLE strm_sync_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL,
    full_sync INTEGER NOT NULL DEFAULT 0,
    scanned INTEGER NOT NULL DEFAULT 0,
    created_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0,
    skipped INTEGER NOT NULL DEFAULT 0,
    removed INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    started_at INTEGER NOT NULL,
    finished_at INTEGER NOT NULL
);
