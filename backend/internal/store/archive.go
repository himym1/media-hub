package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrArchivePlanNotFound = errors.New("archive plan not found")

type ArchivePlan struct {
	ID, PayloadToken, State, ErrorCode, ErrorMessage string
	UserID                                           int64
	StepIndex, StepTotal                             int
	CreatedAt, UpdatedAt                             int64
}
type ArchivePlanEvent struct {
	ID        int64  `json:"id"`
	State     string `json:"state"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"createdAt"`
}

const archivePlanColumns = `id,user_id,payload_token,state,step_index,step_total,error_code,error_message,created_at,updated_at`

func (s *Store) CreateArchivePlan(ctx context.Context, value ArchivePlan) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO archive_plans(id,user_id,payload_token,state,step_total,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, value.ID, value.UserID, value.PayloadToken, value.State, value.StepTotal, value.CreatedAt, value.UpdatedAt); err != nil {
		return err
	}
	if err := insertArchivePlanEvent(ctx, tx, value.ID, value.State, "归档计划已创建，等待确认", value.CreatedAt); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) ArchivePlan(ctx context.Context, userID int64, id string) (ArchivePlan, error) {
	value, err := scanArchivePlan(s.database.QueryRowContext(ctx, `SELECT `+archivePlanColumns+` FROM archive_plans WHERE id=? AND user_id=?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return ArchivePlan{}, ErrArchivePlanNotFound
	}
	return value, err
}
func (s *Store) ListArchivePlans(ctx context.Context, userID int64, limit int) ([]ArchivePlan, error) {
	rows, err := s.database.QueryContext(ctx, `SELECT `+archivePlanColumns+` FROM archive_plans WHERE user_id=? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ArchivePlan{}
	for rows.Next() {
		value, err := scanArchivePlan(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Store) ArchivePlanEvents(ctx context.Context, userID int64, id string) ([]ArchivePlanEvent, error) {
	if _, err := s.ArchivePlan(ctx, userID, id); err != nil {
		return nil, err
	}
	rows, err := s.database.QueryContext(ctx, `SELECT id,state,message,created_at FROM archive_plan_events WHERE plan_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ArchivePlanEvent{}
	for rows.Next() {
		var value ArchivePlanEvent
		if err := rows.Scan(&value.ID, &value.State, &value.Message, &value.CreatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Store) ConfirmArchivePlan(ctx context.Context, userID int64, id, confirmation string, now time.Time) (ArchivePlan, error) {
	if id != confirmation {
		return ArchivePlan{}, ErrArchivePlanNotFound
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return ArchivePlan{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE archive_plans SET state='queued',updated_at=? WHERE id=? AND user_id=? AND state='awaiting_confirmation'`, now.UTC().Unix(), id, userID)
	if err != nil {
		return ArchivePlan{}, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return ArchivePlan{}, ErrArchivePlanNotFound
	}
	if err := insertArchivePlanEvent(ctx, tx, id, "queued", "用户已确认归档计划", now.UTC().Unix()); err != nil {
		return ArchivePlan{}, err
	}
	value, err := scanArchivePlan(tx.QueryRowContext(ctx, `SELECT `+archivePlanColumns+` FROM archive_plans WHERE id=?`, id))
	if err != nil {
		return ArchivePlan{}, err
	}
	if err := tx.Commit(); err != nil {
		return ArchivePlan{}, err
	}
	return value, nil
}
func (s *Store) NextArchivePlan(ctx context.Context) (ArchivePlan, bool, error) {
	value, err := scanArchivePlan(s.database.QueryRowContext(ctx, `SELECT `+archivePlanColumns+` FROM archive_plans WHERE state='queued' ORDER BY created_at,id LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return ArchivePlan{}, false, nil
	}
	return value, err == nil, err
}
func (s *Store) BeginArchivePlan(ctx context.Context, id string, now time.Time) (ArchivePlan, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return ArchivePlan{}, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE archive_plans SET state='running',updated_at=? WHERE id=? AND state='queued'`, now.UTC().Unix(), id)
	if err != nil {
		return ArchivePlan{}, false, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return ArchivePlan{}, false, err
	}
	if err := insertArchivePlanEvent(ctx, tx, id, "running", "归档计划开始执行", now.UTC().Unix()); err != nil {
		return ArchivePlan{}, false, err
	}
	value, err := scanArchivePlan(tx.QueryRowContext(ctx, `SELECT `+archivePlanColumns+` FROM archive_plans WHERE id=?`, id))
	if err != nil {
		return ArchivePlan{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ArchivePlan{}, false, err
	}
	return value, true, nil
}
func (s *Store) AdvanceArchivePlan(ctx context.Context, id string, index int, now time.Time) error {
	_, err := s.database.ExecContext(ctx, `UPDATE archive_plans SET step_index=?,updated_at=? WHERE id=? AND state='running'`, index, now.UTC().Unix(), id)
	return err
}
func (s *Store) FinishArchivePlan(ctx context.Context, value ArchivePlan, message string) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE archive_plans SET state=?,step_index=?,error_code=?,error_message=?,updated_at=? WHERE id=? AND state='running'`, value.State, value.StepIndex, value.ErrorCode, value.ErrorMessage, value.UpdatedAt, value.ID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("archive plan state changed")
	}
	if err := insertArchivePlanEvent(ctx, tx, value.ID, value.State, message, value.UpdatedAt); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) RetryArchivePlan(ctx context.Context, userID int64, id, confirmation string, now time.Time) (ArchivePlan, error) {
	if id != confirmation {
		return ArchivePlan{}, ErrArchivePlanNotFound
	}
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return ArchivePlan{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE archive_plans SET state='queued',error_code='',error_message='',updated_at=? WHERE id=? AND user_id=? AND state IN ('failed','needs_attention')`, now.UTC().Unix(), id, userID)
	if err != nil {
		return ArchivePlan{}, err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return ArchivePlan{}, ErrArchivePlanNotFound
	}
	if err := insertArchivePlanEvent(ctx, tx, id, "queued", "用户确认后继续归档计划", now.UTC().Unix()); err != nil {
		return ArchivePlan{}, err
	}
	value, err := scanArchivePlan(tx.QueryRowContext(ctx, `SELECT `+archivePlanColumns+` FROM archive_plans WHERE id=?`, id))
	if err != nil {
		return ArchivePlan{}, err
	}
	if err := tx.Commit(); err != nil {
		return ArchivePlan{}, err
	}
	return value, nil
}
func (s *Store) MarkInterruptedArchivePlans(ctx context.Context, now time.Time) error {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM archive_plans WHERE state='running'`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE archive_plans SET state='needs_attention',error_code='interrupted',error_message='归档步骤结果需要人工核对',updated_at=? WHERE id=?`, now.UTC().Unix(), id); err != nil {
			return err
		}
		if err := insertArchivePlanEvent(ctx, tx, id, "needs_attention", "执行中断，未自动继续", now.UTC().Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func insertArchivePlanEvent(ctx context.Context, tx *sql.Tx, id, state, message string, created int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO archive_plan_events(plan_id,state,message,created_at) VALUES(?,?,?,?)`, id, state, message, created)
	return err
}
func scanArchivePlan(scanner subscriptionScanner) (ArchivePlan, error) {
	var value ArchivePlan
	err := scanner.Scan(&value.ID, &value.UserID, &value.PayloadToken, &value.State, &value.StepIndex, &value.StepTotal, &value.ErrorCode, &value.ErrorMessage, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}
