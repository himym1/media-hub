ALTER TABLE transfer_jobs ADD COLUMN archived_at INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_transfer_jobs_user_archived_created
ON transfer_jobs(user_id, archived_at, created_at DESC);
