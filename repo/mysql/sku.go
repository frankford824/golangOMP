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

// SKURepoImpl implements repo.SKURepo.
type SKURepoImpl struct{ db *sql.DB }

func NewSKURepo(db *DB) repo.SKURepo { return &SKURepoImpl{db: db.db} }

const skuSelectCols = `id, sku_code, name, current_ver_id, workflow_status, created_at, updated_at`

func scanSKU(s interface {
	Scan(...interface{}) error
}) (*domain.SKU, error) {
	sku := &domain.SKU{}
	var currentVerID sql.NullInt64
	if err := s.Scan(
		&sku.ID,
		&sku.SKUCode,
		&sku.Name,
		&currentVerID,
		&sku.WorkflowStatus,
		&sku.CreatedAt,
		&sku.UpdatedAt,
	); err != nil {
		return nil, err
	}
	sku.CurrentVerID = fromNullInt64(currentVerID)
	return sku, nil
}

func (r *SKURepoImpl) GetByID(ctx context.Context, id int64) (*domain.SKU, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+skuSelectCols+` FROM skus WHERE id = ?`, id)
	sku, err := scanSKU(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sku, err
}

func (r *SKURepoImpl) GetBySKUCode(ctx context.Context, skuCode string) (*domain.SKU, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+skuSelectCols+` FROM skus WHERE sku_code = ?`, skuCode)
	sku, err := scanSKU(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sku, err
}

func (r *SKURepoImpl) List(ctx context.Context, filter repo.SKUListFilter) ([]*domain.SKU, error) {
	q := `SELECT ` + skuSelectCols + ` FROM skus`
	args := make([]interface{}, 0, 3)
	conds := make([]string, 0, 1)

	if filter.WorkflowStatus != nil {
		conds = append(conds, "workflow_status = ?")
		args = append(args, *filter.WorkflowStatus)
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY id DESC"

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize
	q += " LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list skus: %w", err)
	}
	defer rows.Close()

	var skus []*domain.SKU
	for rows.Next() {
		sku, err := scanSKU(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sku: %w", err)
		}
		skus = append(skus, sku)
	}
	return skus, rows.Err()
}

// Create inserts a new SKU row inside an active transaction.
// The caller is responsible for also calling EventRepo.Append in the same TX.
func (r *SKURepoImpl) Create(ctx context.Context, tx repo.Tx, sku *domain.SKU) (int64, error) {
	sqlTx := Unwrap(tx)
	now := time.Now()
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO skus (sku_code, name, workflow_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		sku.SKUCode, sku.Name, domain.WorkflowDraft, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert sku: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("sku last insert id: %w", err)
	}
	return id, nil
}

// UpdateWorkflowStatus sets workflow_status inside an active transaction.
// EventRepo.Append MUST be called in the same TX (spec §8.2).
func (r *SKURepoImpl) UpdateWorkflowStatus(
	ctx context.Context, tx repo.Tx, id int64, status domain.WorkflowStatus,
) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE skus SET workflow_status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update workflow_status: %w", err)
	}
	return nil
}

// SetCurrentVersion updates current_ver_id inside an active transaction.
// EventRepo.Append MUST be called in the same TX (spec §8.2).
func (r *SKURepoImpl) SetCurrentVersion(
	ctx context.Context, tx repo.Tx, skuID, verID int64,
) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx,
		`UPDATE skus SET current_ver_id = ?, updated_at = ? WHERE id = ?`,
		verID, time.Now(), skuID,
	)
	if err != nil {
		return fmt.Errorf("set current_ver_id: %w", err)
	}
	return nil
}

// CASWorkflowStatus performs an atomic conditional status update (optimistic locking).
//
// SQL: UPDATE skus SET workflow_status=next, updated_at=now WHERE id=? AND workflow_status=expected
//
// Returns updated=true if the row was changed; updated=false (no error) means a concurrent
// request already moved the status away from expected — the service layer MUST treat this as
// a 409 conflict (spec §8.2 CAS gate).
func (r *SKURepoImpl) CASWorkflowStatus(
	ctx context.Context,
	tx repo.Tx,
	id int64,
	expected, next domain.WorkflowStatus,
) (bool, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx,
		`UPDATE skus SET workflow_status = ?, updated_at = ? WHERE id = ? AND workflow_status = ?`,
		next, time.Now(), id, expected,
	)
	if err != nil {
		return false, fmt.Errorf("CAS workflow_status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("CAS rows affected: %w", err)
	}
	return n == 1, nil
}
