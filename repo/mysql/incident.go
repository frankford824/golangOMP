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

// IncidentRepoImpl implements repo.IncidentRepo.
type IncidentRepoImpl struct{ db *sql.DB }

func NewIncidentRepo(db *DB) repo.IncidentRepo { return &IncidentRepoImpl{db: db.db} }

const incidentSelectCols = `
	id, sku_id, job_id, status, reason, assignee_id,
	resolved_by, resolved_at, closed_by, closed_at, close_reason,
	created_at, updated_at`

func scanIncident(s interface {
	Scan(...interface{}) error
}) (*domain.Incident, error) {
	inc := &domain.Incident{}
	var (
		jobID       sql.NullInt64
		assigneeID  sql.NullInt64
		resolvedBy  sql.NullInt64
		resolvedAt  sql.NullTime
		closedBy    sql.NullInt64
		closedAt    sql.NullTime
		closeReason sql.NullString
	)
	if err := s.Scan(
		&inc.ID,
		&inc.SKUID,
		&jobID,
		&inc.Status,
		&inc.Reason,
		&assigneeID,
		&resolvedBy,
		&resolvedAt,
		&closedBy,
		&closedAt,
		&closeReason,
		&inc.CreatedAt,
		&inc.UpdatedAt,
	); err != nil {
		return nil, err
	}
	inc.JobID = fromNullInt64(jobID)
	inc.AssigneeID = fromNullInt64(assigneeID)
	inc.ResolvedBy = fromNullInt64(resolvedBy)
	inc.ResolvedAt = fromNullTime(resolvedAt)
	inc.ClosedBy = fromNullInt64(closedBy)
	inc.ClosedAt = fromNullTime(closedAt)
	inc.CloseReason = fromNullString(closeReason)
	return inc, nil
}

// Create inserts a new incident inside an active transaction.
// EventRepo.Append MUST be called in the same TX immediately after (spec §8.2).
func (r *IncidentRepoImpl) Create(ctx context.Context, tx repo.Tx, incident *domain.Incident) (int64, error) {
	sqlTx := Unwrap(tx)
	now := time.Now()
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO incidents (sku_id, job_id, status, reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		incident.SKUID,
		toNullInt64(incident.JobID),
		domain.IncidentStatusOpen,
		incident.Reason,
		now,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("create incident: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("incident last insert id: %w", err)
	}
	return id, nil
}

func (r *IncidentRepoImpl) GetByID(ctx context.Context, id int64) (*domain.Incident, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+incidentSelectCols+` FROM incidents WHERE id = ?`, id)
	inc, err := scanIncident(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return inc, err
}

func (r *IncidentRepoImpl) List(ctx context.Context, filter repo.IncidentListFilter) ([]*domain.Incident, error) {
	q := `SELECT ` + incidentSelectCols + ` FROM incidents`
	args := make([]interface{}, 0, 4)
	conds := make([]string, 0, 2)

	if filter.Status != nil {
		conds = append(conds, "status = ?")
		args = append(args, *filter.Status)
	}
	if filter.SKUID != nil {
		conds = append(conds, "sku_id = ?")
		args = append(args, *filter.SKUID)
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY created_at DESC"

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	q += " LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var incidents []*domain.Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, inc)
	}
	return incidents, rows.Err()
}

// UpdateStatus sets incident.status inside an active transaction.
// EventRepo.Append MUST be called in the same TX (spec §8.2).
func (r *IncidentRepoImpl) UpdateStatus(ctx context.Context, tx repo.Tx, id int64, status domain.IncidentStatus) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE incidents SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return wrapErr(err, "update incident status")
}

// Assign sets the assignee and transitions status to InProgress.
func (r *IncidentRepoImpl) Assign(ctx context.Context, id, assigneeID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE incidents
		SET assignee_id = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		assigneeID, domain.IncidentStatusInProgress, time.Now(), id,
	)
	return wrapErr(err, "assign incident")
}

// Resolve transitions the incident to Resolved and records who resolved it.
func (r *IncidentRepoImpl) Resolve(ctx context.Context, id, resolverID int64) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE incidents
		SET status = ?, resolved_by = ?, resolved_at = ?, updated_at = ?
		WHERE id = ?`,
		domain.IncidentStatusResolved, resolverID, now, now, id,
	)
	return wrapErr(err, "resolve incident")
}

// Close transitions the incident to Closed (Admin only).
// reason MUST be non-empty (enforced by the service layer; documented here for clarity).
func (r *IncidentRepoImpl) Close(ctx context.Context, id, closerID int64, reason string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE incidents
		SET status = ?, closed_by = ?, closed_at = ?, close_reason = ?, updated_at = ?
		WHERE id = ?`,
		domain.IncidentStatusClosed, closerID, now, reason, now, id,
	)
	return wrapErr(err, "close incident")
}
