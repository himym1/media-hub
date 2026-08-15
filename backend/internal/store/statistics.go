package store

import (
	"context"
	"fmt"
)

type OperationalStatistics struct {
	TransfersTotal          int `json:"transfersTotal"`
	TransfersActive         int `json:"transfersActive"`
	TransfersCompleted      int `json:"transfersCompleted"`
	TransfersFailed         int `json:"transfersFailed"`
	TransfersNeedsAttention int `json:"transfersNeedsAttention"`
	SubscriptionsTotal      int `json:"subscriptionsTotal"`
	SubscriptionsEnabled    int `json:"subscriptionsEnabled"`
	RunsTotal               int `json:"runsTotal"`
	RunsFailed              int `json:"runsFailed"`
	CommandsPending         int `json:"commandsPending"`
	CommandsNeedsAttention  int `json:"commandsNeedsAttention"`
	NotificationsAttention  int `json:"notificationsNeedsAttention"`
}

func (s *Store) OperationalStatistics(ctx context.Context, userID int64) (OperationalStatistics, error) {
	var value OperationalStatistics
	err := s.database.QueryRowContext(ctx, `
		SELECT
			(SELECT count(*) FROM transfer_jobs WHERE user_id = ? AND archived_at = 0),
			(SELECT count(*) FROM transfer_jobs WHERE user_id = ? AND archived_at = 0 AND state IN ('queued','transferring','retry_wait','transferred','submitting_sync','syncing','refreshing_emby','indexing_emby','verifying_playback')),
			(SELECT count(*) FROM transfer_jobs WHERE user_id = ? AND archived_at = 0 AND state = 'completed'),
			(SELECT count(*) FROM transfer_jobs WHERE user_id = ? AND archived_at = 0 AND state = 'failed'),
			(SELECT count(*) FROM transfer_jobs WHERE user_id = ? AND archived_at = 0 AND state = 'needs_attention'),
			(SELECT count(*) FROM subscriptions WHERE user_id = ?),
			(SELECT count(*) FROM subscriptions WHERE user_id = ? AND enabled = 1),
			(SELECT count(*) FROM subscription_runs r JOIN subscriptions s ON s.id = r.subscription_id WHERE s.user_id = ?),
			(SELECT count(*) FROM subscription_runs r JOIN subscriptions s ON s.id = r.subscription_id WHERE s.user_id = ? AND r.state IN ('failed','needs_attention')),
			((SELECT count(*) FROM subx_command_jobs WHERE user_id = ? AND state IN ('queued','submitting')) +
			 (SELECT count(*) FROM drive115_command_jobs WHERE user_id = ? AND state IN ('awaiting_confirmation','queued','submitting')) +
			 (SELECT count(*) FROM local_upload_jobs WHERE user_id = ? AND state IN ('queued','hashing','submitting_init','uploading')) +
			 (SELECT count(*) FROM archive_plans WHERE user_id = ? AND state IN ('awaiting_confirmation','queued','running'))),
			((SELECT count(*) FROM subx_command_jobs WHERE user_id = ? AND state = 'needs_attention') +
			 (SELECT count(*) FROM drive115_command_jobs WHERE user_id = ? AND state = 'needs_attention') +
			 (SELECT count(*) FROM local_upload_jobs WHERE user_id = ? AND state = 'needs_attention') +
			 (SELECT count(*) FROM archive_plans WHERE user_id = ? AND state = 'needs_attention')),
			(SELECT count(*) FROM transfer_notifications n JOIN transfer_jobs j ON j.id = n.job_id WHERE j.user_id = ? AND j.archived_at = 0 AND n.state = 'needs_attention')
	`, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID, userID).Scan(
		&value.TransfersTotal,
		&value.TransfersActive,
		&value.TransfersCompleted,
		&value.TransfersFailed,
		&value.TransfersNeedsAttention,
		&value.SubscriptionsTotal,
		&value.SubscriptionsEnabled,
		&value.RunsTotal,
		&value.RunsFailed,
		&value.CommandsPending,
		&value.CommandsNeedsAttention,
		&value.NotificationsAttention,
	)
	if err != nil {
		return OperationalStatistics{}, fmt.Errorf("read operational statistics: %w", err)
	}
	return value, nil
}
