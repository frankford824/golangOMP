package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type procurementRepo struct{ db *DB }

func NewProcurementRepo(db *DB) repo.ProcurementRepo { return &procurementRepo{db: db} }

func (r *procurementRepo) GetByTaskID(ctx context.Context, taskID int64) (*domain.ProcurementRecord, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_id, status, procurement_price, quantity, supplier_name, purchase_remark, expected_delivery_at, created_at, updated_at
		FROM procurement_records
		WHERE task_id = ?`, taskID)
	return scanProcurementRecord(row)
}

func (r *procurementRepo) ListItemsByTaskID(ctx context.Context, taskID int64) ([]*domain.ProcurementRecordItem, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, procurement_record_id, task_id, task_sku_item_id, sku_code, status,
		       quantity, cost_price, base_sale_price, created_at, updated_at
		FROM procurement_record_items
		WHERE task_id = ?
		ORDER BY id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list procurement_record_items: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.ProcurementRecordItem, 0)
	for rows.Next() {
		item, err := scanProcurementRecordItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate procurement_record_items: %w", err)
	}
	return items, nil
}

func (r *procurementRepo) Upsert(ctx context.Context, tx repo.Tx, record *domain.ProcurementRecord) error {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO procurement_records
		  (task_id, status, procurement_price, quantity, supplier_name, purchase_remark, expected_delivery_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  id = LAST_INSERT_ID(id),
		  status = VALUES(status),
		  procurement_price = VALUES(procurement_price),
		  quantity = VALUES(quantity),
		  supplier_name = VALUES(supplier_name),
		  purchase_remark = VALUES(purchase_remark),
		  expected_delivery_at = VALUES(expected_delivery_at),
		  updated_at = CURRENT_TIMESTAMP`,
		record.TaskID,
		string(record.Status),
		toNullFloat64(record.ProcurementPrice),
		toNullInt64(record.Quantity),
		record.SupplierName,
		record.PurchaseRemark,
		toNullTime(record.ExpectedDeliveryAt),
	)
	if err != nil {
		return fmt.Errorf("upsert procurement_record: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil {
		record.ID = id
	}
	return nil
}

func (r *procurementRepo) CreateItems(ctx context.Context, tx repo.Tx, items []*domain.ProcurementRecordItem) error {
	if len(items) == 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	stmt, err := sqlTx.PrepareContext(ctx, `
		INSERT INTO procurement_record_items
		  (procurement_record_id, task_id, task_sku_item_id, sku_code, status, quantity, cost_price, base_sale_price)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert procurement_record_items: %w", err)
	}
	defer stmt.Close()

	for _, item := range items {
		if item == nil {
			continue
		}
		res, err := stmt.ExecContext(
			ctx,
			item.ProcurementRecordID,
			item.TaskID,
			item.TaskSKUItemID,
			item.SKUCode,
			string(item.Status),
			toNullInt64(item.Quantity),
			toNullFloat64(item.CostPrice),
			toNullFloat64(item.BaseSalePrice),
		)
		if err != nil {
			return fmt.Errorf("insert procurement_record_item: %w", err)
		}
		if id, err := res.LastInsertId(); err == nil {
			item.ID = id
		}
	}
	return nil
}

func scanProcurementRecord(row *sql.Row) (*domain.ProcurementRecord, error) {
	var record domain.ProcurementRecord
	var procurementPrice sql.NullFloat64
	var quantity sql.NullInt64
	var expectedDeliveryAt sql.NullTime
	err := row.Scan(
		&record.ID,
		&record.TaskID,
		&record.Status,
		&procurementPrice,
		&quantity,
		&record.SupplierName,
		&record.PurchaseRemark,
		&expectedDeliveryAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan procurement_record: %w", err)
	}
	record.ProcurementPrice = fromNullFloat64(procurementPrice)
	record.Quantity = fromNullInt64(quantity)
	record.ExpectedDeliveryAt = fromNullTime(expectedDeliveryAt)
	return &record, nil
}

func scanProcurementRecordItem(scanner interface{ Scan(...interface{}) error }) (*domain.ProcurementRecordItem, error) {
	var item domain.ProcurementRecordItem
	var quantity sql.NullInt64
	var costPrice, baseSalePrice sql.NullFloat64
	if err := scanner.Scan(
		&item.ID,
		&item.ProcurementRecordID,
		&item.TaskID,
		&item.TaskSKUItemID,
		&item.SKUCode,
		&item.Status,
		&quantity,
		&costPrice,
		&baseSalePrice,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan procurement_record_item: %w", err)
	}
	item.Quantity = fromNullInt64(quantity)
	item.CostPrice = fromNullFloat64(costPrice)
	item.BaseSalePrice = fromNullFloat64(baseSalePrice)
	return &item, nil
}
