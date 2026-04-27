package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type outsourceRepo struct{ db *DB }

func NewOutsourceRepo(db *DB) repo.OutsourceRepo { return &outsourceRepo{db: db} }

func (r *outsourceRepo) Create(ctx context.Context, tx repo.Tx, order *domain.OutsourceOrder) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO outsource_orders
		  (outsource_no, task_id, vendor_name, outsource_type, delivery_requirement, settlement_note, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		order.OutsourceNo,
		order.TaskID,
		order.VendorName,
		order.OutsourceType,
		order.DeliveryRequirement,
		order.SettlementNote,
		string(order.Status),
	)
	if err != nil {
		return 0, fmt.Errorf("insert outsource_order: %w", err)
	}
	return res.LastInsertId()
}

func (r *outsourceRepo) GetByID(ctx context.Context, id int64) (*domain.OutsourceOrder, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, outsource_no, task_id, vendor_name, outsource_type,
		       delivery_requirement, settlement_note, status, returned_at, created_at, updated_at
		FROM outsource_orders WHERE id = ?`, id)
	return scanOutsourceOrder(row)
}

func (r *outsourceRepo) List(ctx context.Context, filter repo.OutsourceListFilter) ([]*domain.OutsourceOrder, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}

	if filter.TaskID != nil {
		where = append(where, "task_id = ?")
		args = append(args, *filter.TaskID)
	}
	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.Vendor != "" {
		where = append(where, "vendor_name LIKE ?")
		args = append(args, "%"+filter.Vendor+"%")
	}

	whereSQL := strings.Join(where, " AND ")
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM outsource_orders WHERE %s`, whereSQL)
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count outsource_orders: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT id, outsource_no, task_id, vendor_name, outsource_type,
		       delivery_requirement, settlement_note, status, returned_at, created_at, updated_at
		FROM outsource_orders WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereSQL)
	args = append(args, pageSize, offset)

	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list outsource_orders: %w", err)
	}
	defer rows.Close()

	var orders []*domain.OutsourceOrder
	for rows.Next() {
		var o domain.OutsourceOrder
		var returnedAt sql.NullTime
		if err := rows.Scan(
			&o.ID, &o.OutsourceNo, &o.TaskID, &o.VendorName, &o.OutsourceType,
			&o.DeliveryRequirement, &o.SettlementNote, &o.Status, &returnedAt, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan outsource_order: %w", err)
		}
		o.ReturnedAt = fromNullTime(returnedAt)
		orders = append(orders, &o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *outsourceRepo) Update(ctx context.Context, tx repo.Tx, order *domain.OutsourceOrder) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE outsource_orders
		SET vendor_name = ?, outsource_type = ?, delivery_requirement = ?, settlement_note = ?,
		    status = ?, returned_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		order.VendorName,
		order.OutsourceType,
		order.DeliveryRequirement,
		order.SettlementNote,
		string(order.Status),
		toNullTime(order.ReturnedAt),
		order.ID,
	)
	if err != nil {
		return fmt.Errorf("update outsource_order: %w", err)
	}
	return nil
}

func scanOutsourceOrder(row *sql.Row) (*domain.OutsourceOrder, error) {
	var o domain.OutsourceOrder
	var returnedAt sql.NullTime
	err := row.Scan(
		&o.ID, &o.OutsourceNo, &o.TaskID, &o.VendorName, &o.OutsourceType,
		&o.DeliveryRequirement, &o.SettlementNote, &o.Status, &returnedAt, &o.CreatedAt, &o.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan outsource_order: %w", err)
	}
	o.ReturnedAt = fromNullTime(returnedAt)
	return &o, nil
}
