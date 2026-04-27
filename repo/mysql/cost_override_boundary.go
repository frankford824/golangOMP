package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type taskCostOverrideReviewRepo struct{ db *DB }
type taskCostFinanceFlagRepo struct{ db *DB }

func NewTaskCostOverrideReviewRepo(db *DB) repo.TaskCostOverrideReviewRepo {
	return &taskCostOverrideReviewRepo{db: db}
}

func NewTaskCostFinanceFlagRepo(db *DB) repo.TaskCostFinanceFlagRepo {
	return &taskCostFinanceFlagRepo{db: db}
}

func (r *taskCostOverrideReviewRepo) Upsert(ctx context.Context, tx repo.Tx, record *domain.TaskCostOverrideReviewRecord) (*domain.TaskCostOverrideReviewRecord, error) {
	if record == nil {
		return nil, fmt.Errorf("upsert cost override review: record is nil")
	}
	execDB := execerFromTx(r.db, tx)
	reviewedAt := toNullTime(record.ReviewedAt)
	now := time.Now().UTC()

	if _, err := execDB.ExecContext(ctx, `
		INSERT INTO cost_override_reviews (
			override_event_id, task_id, review_required, review_status, review_note, review_actor, reviewed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			task_id = VALUES(task_id),
			review_required = VALUES(review_required),
			review_status = VALUES(review_status),
			review_note = VALUES(review_note),
			review_actor = VALUES(review_actor),
			reviewed_at = VALUES(reviewed_at),
			updated_at = VALUES(updated_at)`,
		strings.TrimSpace(record.OverrideEventID),
		record.TaskID,
		record.ReviewRequired,
		string(record.ReviewStatus),
		strings.TrimSpace(record.ReviewNote),
		strings.TrimSpace(record.ReviewActor),
		reviewedAt,
		now,
		now,
	); err != nil {
		return nil, fmt.Errorf("upsert cost override review: %w", err)
	}

	return r.GetByEventID(ctx, record.OverrideEventID)
}

func (r *taskCostOverrideReviewRepo) GetByEventID(ctx context.Context, eventID string) (*domain.TaskCostOverrideReviewRecord, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT record_id, override_event_id, task_id, review_required, review_status, review_note, review_actor, reviewed_at, created_at, updated_at
		FROM cost_override_reviews
		WHERE override_event_id = ?`, strings.TrimSpace(eventID))
	record, err := scanTaskCostOverrideReviewRecord(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cost override review by event_id: %w", err)
	}
	return record, nil
}

func (r *taskCostOverrideReviewRepo) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskCostOverrideReviewRecord, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT record_id, override_event_id, task_id, review_required, review_status, review_note, review_actor, reviewed_at, created_at, updated_at
		FROM cost_override_reviews
		WHERE task_id = ?
		ORDER BY record_id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list cost override reviews: %w", err)
	}
	defer rows.Close()

	records := make([]*domain.TaskCostOverrideReviewRecord, 0)
	for rows.Next() {
		record, err := scanTaskCostOverrideReviewRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan cost override review: %w", err)
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r *taskCostFinanceFlagRepo) Upsert(ctx context.Context, tx repo.Tx, flag *domain.TaskCostFinanceFlag) (*domain.TaskCostFinanceFlag, error) {
	if flag == nil {
		return nil, fmt.Errorf("upsert cost finance flag: flag is nil")
	}
	execDB := execerFromTx(r.db, tx)
	financeMarkedAt := toNullTime(flag.FinanceMarkedAt)
	now := time.Now().UTC()

	if _, err := execDB.ExecContext(ctx, `
		INSERT INTO cost_override_finance_flags (
			override_event_id, task_id, finance_required, finance_status, finance_note, finance_marked_by, finance_marked_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			task_id = VALUES(task_id),
			finance_required = VALUES(finance_required),
			finance_status = VALUES(finance_status),
			finance_note = VALUES(finance_note),
			finance_marked_by = VALUES(finance_marked_by),
			finance_marked_at = VALUES(finance_marked_at),
			updated_at = VALUES(updated_at)`,
		strings.TrimSpace(flag.OverrideEventID),
		flag.TaskID,
		flag.FinanceRequired,
		string(flag.FinanceStatus),
		strings.TrimSpace(flag.FinanceNote),
		strings.TrimSpace(flag.FinanceMarkedBy),
		financeMarkedAt,
		now,
		now,
	); err != nil {
		return nil, fmt.Errorf("upsert cost finance flag: %w", err)
	}

	return r.GetByEventID(ctx, flag.OverrideEventID)
}

func (r *taskCostFinanceFlagRepo) GetByEventID(ctx context.Context, eventID string) (*domain.TaskCostFinanceFlag, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT record_id, override_event_id, task_id, finance_required, finance_status, finance_note, finance_marked_by, finance_marked_at, created_at, updated_at
		FROM cost_override_finance_flags
		WHERE override_event_id = ?`, strings.TrimSpace(eventID))
	flag, err := scanTaskCostFinanceFlag(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cost finance flag by event_id: %w", err)
	}
	return flag, nil
}

func (r *taskCostFinanceFlagRepo) ListByTaskID(ctx context.Context, taskID int64) ([]*domain.TaskCostFinanceFlag, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT record_id, override_event_id, task_id, finance_required, finance_status, finance_note, finance_marked_by, finance_marked_at, created_at, updated_at
		FROM cost_override_finance_flags
		WHERE task_id = ?
		ORDER BY record_id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list cost finance flags: %w", err)
	}
	defer rows.Close()

	flags := make([]*domain.TaskCostFinanceFlag, 0)
	for rows.Next() {
		flag, err := scanTaskCostFinanceFlag(rows)
		if err != nil {
			return nil, fmt.Errorf("scan cost finance flag: %w", err)
		}
		flags = append(flags, flag)
	}
	return flags, rows.Err()
}

func scanTaskCostOverrideReviewRecord(scanner interface {
	Scan(...interface{}) error
}) (*domain.TaskCostOverrideReviewRecord, error) {
	record := &domain.TaskCostOverrideReviewRecord{}
	var reviewStatus string
	var reviewedAt sql.NullTime
	if err := scanner.Scan(
		&record.RecordID,
		&record.OverrideEventID,
		&record.TaskID,
		&record.ReviewRequired,
		&reviewStatus,
		&record.ReviewNote,
		&record.ReviewActor,
		&reviewedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.ReviewStatus = domain.TaskCostOverrideReviewStatus(reviewStatus)
	record.ReviewedAt = fromNullTime(reviewedAt)
	return record, nil
}

func scanTaskCostFinanceFlag(scanner interface {
	Scan(...interface{}) error
}) (*domain.TaskCostFinanceFlag, error) {
	flag := &domain.TaskCostFinanceFlag{}
	var financeStatus string
	var financeMarkedAt sql.NullTime
	if err := scanner.Scan(
		&flag.RecordID,
		&flag.OverrideEventID,
		&flag.TaskID,
		&flag.FinanceRequired,
		&financeStatus,
		&flag.FinanceNote,
		&flag.FinanceMarkedBy,
		&financeMarkedAt,
		&flag.CreatedAt,
		&flag.UpdatedAt,
	); err != nil {
		return nil, err
	}
	flag.FinanceStatus = domain.TaskCostOverrideFinanceStatus(financeStatus)
	flag.FinanceMarkedAt = fromNullTime(financeMarkedAt)
	return flag, nil
}

func execerFromTx(db *DB, tx repo.Tx) interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
} {
	if tx != nil {
		return Unwrap(tx)
	}
	return db.db
}
