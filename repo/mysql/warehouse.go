package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type warehouseRepo struct{ db *DB }

func NewWarehouseRepo(db *DB) repo.WarehouseRepo { return &warehouseRepo{db: db} }

func (r *warehouseRepo) Create(ctx context.Context, tx repo.Tx, receipt *domain.WarehouseReceipt) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO warehouse_receipts
		  (task_id, receipt_no, status, receiver_id, received_at, completed_at, reject_reason, remark)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		receipt.TaskID,
		receipt.ReceiptNo,
		string(receipt.Status),
		toNullInt64(receipt.ReceiverID),
		toNullTime(receipt.ReceivedAt),
		toNullTime(receipt.CompletedAt),
		receipt.RejectReason,
		receipt.Remark,
	)
	if err != nil {
		return 0, fmt.Errorf("insert warehouse_receipt: %w", err)
	}
	return res.LastInsertId()
}

func (r *warehouseRepo) GetByID(ctx context.Context, id int64) (*domain.WarehouseReceipt, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_id, receipt_no, status, receiver_id, received_at, completed_at, reject_reason, remark, created_at, updated_at
		FROM warehouse_receipts WHERE id = ?`, id)
	return scanWarehouseReceipt(row)
}

func (r *warehouseRepo) GetByTaskID(ctx context.Context, taskID int64) (*domain.WarehouseReceipt, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_id, receipt_no, status, receiver_id, received_at, completed_at, reject_reason, remark, created_at, updated_at
		FROM warehouse_receipts WHERE task_id = ?`, taskID)
	return scanWarehouseReceipt(row)
}

func (r *warehouseRepo) List(ctx context.Context, filter repo.WarehouseListFilter) ([]*domain.WarehouseReceipt, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	fromSQL := "warehouse_receipts wr"

	if filter.TaskID != nil {
		where = append(where, "task_id = ?")
		args = append(args, *filter.TaskID)
	}
	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.ReceiverID != nil {
		where = append(where, "receiver_id = ?")
		args = append(args, *filter.ReceiverID)
	}
	if filter.WorkflowLane != nil {
		fromSQL = "warehouse_receipts wr INNER JOIN tasks t ON t.id = wr.task_id"
		switch *filter.WorkflowLane {
		case domain.WorkflowLaneCustomization:
			where = append(where, "t.customization_required = 1")
		default:
			where = append(where, "t.customization_required = 0")
		}
	}

	whereSQL := strings.Join(where, " AND ")
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, fromSQL, whereSQL)
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count warehouse_receipts: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT wr.id, wr.task_id, wr.receipt_no, wr.status, wr.receiver_id, wr.received_at, wr.completed_at, wr.reject_reason, wr.remark, wr.created_at, wr.updated_at
		FROM %s
		WHERE %s
		ORDER BY wr.id DESC LIMIT ? OFFSET ?`, fromSQL, whereSQL)
	args = append(args, pageSize, offset)

	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list warehouse_receipts: %w", err)
	}
	defer rows.Close()

	var receipts []*domain.WarehouseReceipt
	for rows.Next() {
		receipt, err := scanWarehouseReceiptRows(rows)
		if err != nil {
			return nil, 0, err
		}
		receipts = append(receipts, receipt)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return receipts, total, nil
}

func (r *warehouseRepo) Update(ctx context.Context, tx repo.Tx, receipt *domain.WarehouseReceipt) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE warehouse_receipts
		SET status = ?, receiver_id = ?, received_at = ?, completed_at = ?, reject_reason = ?, remark = ?
		WHERE id = ?`,
		string(receipt.Status),
		toNullInt64(receipt.ReceiverID),
		toNullTime(receipt.ReceivedAt),
		toNullTime(receipt.CompletedAt),
		receipt.RejectReason,
		receipt.Remark,
		receipt.ID,
	)
	if err != nil {
		return fmt.Errorf("update warehouse_receipt: %w", err)
	}
	return nil
}

func scanWarehouseReceipt(row *sql.Row) (*domain.WarehouseReceipt, error) {
	var receipt domain.WarehouseReceipt
	var receiverID sql.NullInt64
	var receivedAt sql.NullTime
	var completedAt sql.NullTime
	err := row.Scan(
		&receipt.ID, &receipt.TaskID, &receipt.ReceiptNo, &receipt.Status, &receiverID,
		&receivedAt, &completedAt, &receipt.RejectReason, &receipt.Remark, &receipt.CreatedAt, &receipt.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan warehouse_receipt: %w", err)
	}
	receipt.ReceiverID = fromNullInt64(receiverID)
	receipt.ReceivedAt = fromNullTime(receivedAt)
	receipt.CompletedAt = fromNullTime(completedAt)
	return &receipt, nil
}

func scanWarehouseReceiptRows(rows *sql.Rows) (*domain.WarehouseReceipt, error) {
	var receipt domain.WarehouseReceipt
	var receiverID sql.NullInt64
	var receivedAt sql.NullTime
	var completedAt sql.NullTime
	if err := rows.Scan(
		&receipt.ID, &receipt.TaskID, &receipt.ReceiptNo, &receipt.Status, &receiverID,
		&receivedAt, &completedAt, &receipt.RejectReason, &receipt.Remark, &receipt.CreatedAt, &receipt.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan warehouse_receipt row: %w", err)
	}
	receipt.ReceiverID = fromNullInt64(receiverID)
	receipt.ReceivedAt = fromNullTime(receivedAt)
	receipt.CompletedAt = fromNullTime(completedAt)
	return &receipt, nil
}
