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

type orgMoveRequestRepo struct{ db *DB }

func NewOrgMoveRequestRepo(db *DB) repo.OrgMoveRequestRepo {
	return &orgMoveRequestRepo{db: db}
}

func (r *orgMoveRequestRepo) Create(ctx context.Context, tx repo.Tx, request *domain.OrgMoveRequest) (int64, error) {
	result, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO org_move_requests (source_department, target_department, user_id, state, requested_by, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		request.SourceDepartment,
		nullString(request.TargetDepartment),
		request.UserID,
		request.State,
		request.RequestedByUserID,
		request.Reason,
		request.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert org_move_request: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("org_move_request last insert id: %w", err)
	}
	return id, nil
}

func (r *orgMoveRequestRepo) Get(ctx context.Context, id int64) (*domain.OrgMoveRequest, error) {
	row := r.db.db.QueryRowContext(ctx, orgMoveRequestSelectSQL()+` WHERE omr.id = ?`, id)
	return scanOrgMoveRequest(row)
}

func (r *orgMoveRequestRepo) List(ctx context.Context, filter repo.OrgMoveRequestFilter) ([]*domain.OrgMoveRequest, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	where := []string{"1=1"}
	args := make([]interface{}, 0, 8)
	if filter.State != nil && *filter.State != "" {
		where = append(where, "omr.state = ?")
		args = append(args, *filter.State)
	}
	if filter.UserID != nil && *filter.UserID > 0 {
		where = append(where, "omr.user_id = ?")
		args = append(args, *filter.UserID)
	}
	if filter.SourceDepartmentID != nil && *filter.SourceDepartmentID > 0 {
		where = append(where, "src.id = ?")
		args = append(args, *filter.SourceDepartmentID)
	}
	if len(filter.SourceDepartments) > 0 {
		placeholders := make([]string, 0, len(filter.SourceDepartments))
		for _, department := range filter.SourceDepartments {
			department = strings.TrimSpace(department)
			if department == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, department)
		}
		if len(placeholders) > 0 {
			where = append(where, "omr.source_department IN ("+strings.Join(placeholders, ",")+")")
		}
	}

	countQuery := `
		SELECT COUNT(*)
		FROM org_move_requests omr
		LEFT JOIN org_departments src ON src.name = omr.source_department COLLATE utf8mb4_0900_ai_ci
		WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count org_move_requests: %w", err)
	}

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, orgMoveRequestSelectSQL()+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY omr.id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list org_move_requests: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.OrgMoveRequest, 0)
	for rows.Next() {
		item, err := scanOrgMoveRequest(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func (r *orgMoveRequestRepo) UpdateState(ctx context.Context, tx repo.Tx, id int64, from, to domain.OrgMoveRequestState, decidedBy int64, reason string, decidedAt time.Time) (bool, error) {
	result, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE org_move_requests
		SET state = ?, resolved_by = ?, resolved_at = ?, reason = ?
		WHERE id = ? AND state = ?`,
		to, decidedBy, decidedAt, reason, id, from,
	)
	if err != nil {
		return false, fmt.Errorf("update org_move_request state: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("org_move_request rows affected: %w", err)
	}
	return affected == 1, nil
}

func orgMoveRequestSelectSQL() string {
	// SA-B.2(2026-04-24):org_departments.name 继承 MySQL 8 默认 utf8mb4_0900_ai_ci,
	// org_move_requests.{source_department,target_department} 继承 R2 mig 065 的 utf8mb4_unicode_ci。
	// JOIN 时必须显式 COLLATE 对齐父表(老表)以避免 Error 1267 Illegal mix of collations。
	return `
		SELECT omr.id, omr.source_department, COALESCE(src.id, 0) AS source_department_id,
		       COALESCE(omr.target_department, '') AS target_department, dst.id AS target_department_id,
		       omr.user_id, omr.state, omr.requested_by, omr.resolved_by, omr.reason, omr.resolved_at, omr.created_at
		FROM org_move_requests omr
		LEFT JOIN org_departments src ON src.name = omr.source_department COLLATE utf8mb4_0900_ai_ci
		LEFT JOIN org_departments dst ON dst.name = omr.target_department COLLATE utf8mb4_0900_ai_ci`
}

func scanOrgMoveRequest(scanner interface {
	Scan(dest ...interface{}) error
}) (*domain.OrgMoveRequest, error) {
	var item domain.OrgMoveRequest
	var targetDepartmentID sql.NullInt64
	var resolvedBy sql.NullInt64
	var resolvedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.SourceDepartment,
		&item.SourceDepartmentID,
		&item.TargetDepartment,
		&targetDepartmentID,
		&item.UserID,
		&item.State,
		&item.RequestedByUserID,
		&resolvedBy,
		&item.Reason,
		&resolvedAt,
		&item.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan org_move_request: %w", err)
	}
	item.TargetDepartmentID = fromNullInt64(targetDepartmentID)
	item.DecidedByUserID = fromNullInt64(resolvedBy)
	item.ResolvedAt = fromNullTime(resolvedAt)
	item.UpdatedAt = item.CreatedAt
	if item.ResolvedAt != nil {
		item.UpdatedAt = *item.ResolvedAt
	}
	if item.State == domain.OrgMoveRequestStateRejected {
		item.RejectReason = item.Reason
	}
	return &item, nil
}
