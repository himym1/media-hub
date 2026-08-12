CREATE TABLE library_mappings (
    id INTEGER PRIMARY KEY,
    media_type TEXT NOT NULL UNIQUE CHECK (media_type IN ('movie', 'series')),
    cloud_directory_id TEXT NOT NULL,
    emby_library_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
