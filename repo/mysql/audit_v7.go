package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type auditV7Repo struct{ db *DB }

func NewAuditV7Repo(db *DB) repo.AuditV7Repo { return &auditV7Repo{db: db} }

func (r *auditV7Repo) CreateRecord(ctx context.Context, tx repo.Tx, record *domain.AuditRecord) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO audit_records
		  (task_id, stage, action, auditor_id, issue_types_json, comment, affects_launch, need_outsource)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.TaskID,
		string(record.Stage),
		string(record.Action),
		record.AuditorID,
		record.IssueTypesJSON,
		record.Comment,
		record.AffectsLaunch,
		record.NeedOutsource,
	)
	if err != nil {
		return 0, fmt.Errorf("insert audit_record: %w", err)
	}
	return res.LastInsertId()
}

func (r *auditV7Repo) ListRecords(ctx context.Context, filter repo.AuditRecordListFilter) ([]*domain.AuditRecord, error) {
	query := `
		SELECT ar.id, ar.task_id, ar.stage, ar.action, ar.auditor_id, ar.issue_types_json,
		       ar.comment, ar.affects_launch, ar.need_outsource, ar.created_at
		FROM audit_records ar
		JOIN tasks t ON t.id = ar.task_id
		LEFT JOIN users u ON u.id = ar.auditor_id
		WHERE 1=1`
	args := []interface{}{}
	if filter.TaskNo != "" {
		query += ` AND (t.task_no LIKE ? OR CAST(ar.task_id AS CHAR) LIKE ?)`
		kw := "%" + filter.TaskNo + "%"
		args = append(args, kw, kw)
	}
	if filter.Auditor != "" {
		query += ` AND (u.display_name LIKE ? OR u.username LIKE ?)`
		kw := "%" + filter.Auditor + "%"
		args = append(args, kw, kw)
	}
	if filter.Action != "" {
		query += ` AND ar.action = ?`
		args = append(args, filter.Action)
	}
	if filter.StartAt != "" {
		query += ` AND ar.created_at >= ?`
		args = append(args, filter.StartAt+" 00:00:00")
	}
	if filter.EndAt != "" {
		query += ` AND ar.created_at <= ?`
		args = append(args, filter.EndAt+" 23:59:59")
	}
	query += ` ORDER BY ar.created_at DESC, ar.id DESC`
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	query += ` LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit_records: %w", err)
	}
	defer rows.Close()

	var records []*domain.AuditRecord
	for rows.Next() {
		var ar domain.AuditRecord
		if err := rows.Scan(
			&ar.ID, &ar.TaskID, &ar.Stage, &ar.Action, &ar.AuditorID,
			&ar.IssueTypesJSON, &ar.Comment, &ar.AffectsLaunch, &ar.NeedOutsource, &ar.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit_record: %w", err)
		}
		records = append(records, &ar)
	}
	return records, rows.Err()
}

func (r *auditV7Repo) ListRecordsByTaskID(ctx context.Context, taskID int64) ([]*domain.AuditRecord, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_id, stage, action, auditor_id, issue_types_json,
		       comment, affects_launch, need_outsource, created_at
		FROM audit_records WHERE task_id = ? ORDER BY id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list audit_records: %w", err)
	}
	defer rows.Close()

	var records []*domain.AuditRecord
	for rows.Next() {
		var ar domain.AuditRecord
		if err := rows.Scan(
			&ar.ID, &ar.TaskID, &ar.Stage, &ar.Action, &ar.AuditorID,
			&ar.IssueTypesJSON, &ar.Comment, &ar.AffectsLaunch, &ar.NeedOutsource, &ar.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit_record: %w", err)
		}
		records = append(records, &ar)
	}
	return records, rows.Err()
}

func (r *auditV7Repo) CreateHandover(ctx context.Context, tx repo.Tx, handover *domain.AuditHandover) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO audit_handovers
		  (handover_no, task_id, from_auditor_id, to_auditor_id, reason, current_judgement, risk_remark, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		handover.HandoverNo,
		handover.TaskID,
		handover.FromAuditorID,
		handover.ToAuditorID,
		handover.Reason,
		handover.CurrentJudgement,
		handover.RiskRemark,
		string(handover.Status),
	)
	if err != nil {
		return 0, fmt.Errorf("insert audit_handover: %w", err)
	}
	return res.LastInsertId()
}

func (r *auditV7Repo) GetHandoverByID(ctx context.Context, id int64) (*domain.AuditHandover, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, handover_no, task_id, from_auditor_id, to_auditor_id,
		       reason, current_judgement, risk_remark, status, created_at, updated_at
		FROM audit_handovers WHERE id = ?`, id)
	var h domain.AuditHandover
	err := row.Scan(
		&h.ID, &h.HandoverNo, &h.TaskID, &h.FromAuditorID, &h.ToAuditorID,
		&h.Reason, &h.CurrentJudgement, &h.RiskRemark, &h.Status, &h.CreatedAt, &h.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan audit_handover: %w", err)
	}
	return &h, nil
}

func (r *auditV7Repo) ListHandoversByTaskID(ctx context.Context, taskID int64) ([]*domain.AuditHandover, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, handover_no, task_id, from_auditor_id, to_auditor_id,
		       reason, current_judgement, risk_remark, status, created_at, updated_at
		FROM audit_handovers
		WHERE task_id = ?
		ORDER BY created_at DESC, id DESC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list audit_handovers: %w", err)
	}
	defer rows.Close()

	var handovers []*domain.AuditHandover
	for rows.Next() {
		var h domain.AuditHandover
		if err := rows.Scan(
			&h.ID, &h.HandoverNo, &h.TaskID, &h.FromAuditorID, &h.ToAuditorID,
			&h.Reason, &h.CurrentJudgement, &h.RiskRemark, &h.Status, &h.CreatedAt, &h.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit_handover: %w", err)
		}
		handovers = append(handovers, &h)
	}
	return handovers, rows.Err()
}

func (r *auditV7Repo) UpdateHandoverStatus(ctx context.Context, tx repo.Tx, id int64, status domain.HandoverStatus) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE audit_handovers SET status = ? WHERE id = ?`,
		string(status), id,
	)
	if err != nil {
		return fmt.Errorf("update handover status: %w", err)
	}
	return nil
}
