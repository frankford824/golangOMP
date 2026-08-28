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

type erpCostReadRepo struct{ db *DB }

func NewERPCostReadRepo(db *DB) repo.ERPCostReadRepo {
	return &erpCostReadRepo{db: db}
}

func (r *erpCostReadRepo) InventoryWatermark(ctx context.Context) (time.Time, error) {
	var watermark sql.NullTime
	err := r.db.db.QueryRowContext(ctx, `SELECT MAX(local_updated_at) FROM jst_inventory`).Scan(&watermark)
	if err != nil {
		return time.Time{}, fmt.Errorf("read jst inventory watermark: %w", err)
	}
	if !watermark.Valid {
		return time.Unix(0, 0).UTC(), nil
	}
	return watermark.Time, nil
}

func (r *erpCostReadRepo) ListInventoryCosts(ctx context.Context, query repo.ERPCostFeedPageQuery) ([]domain.ERPCostSKU, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT sku_id, NULLIF(TRIM(sku_type), ''), CAST(cost_price AS CHAR), CAST(sale_price AS CHAR), local_updated_at
		  FROM jst_inventory
		 WHERE local_updated_at > ?
		   AND local_updated_at <= ?
		   AND (local_updated_at > ? OR (local_updated_at = ? AND sku_id > ?))
		 ORDER BY local_updated_at ASC, sku_id ASC
		 LIMIT ?`,
		query.UpdatedSince,
		query.Watermark,
		query.LastModifiedAt,
		query.LastModifiedAt,
		strings.TrimSpace(query.LastSKUID),
		query.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list jst inventory costs: %w", err)
	}
	defer rows.Close()
	return scanERPCostSKUs(rows)
}

func (r *erpCostReadRepo) BatchInventoryCosts(ctx context.Context, skuIDs []string) ([]domain.ERPCostSKU, time.Time, error) {
	if len(skuIDs) == 0 {
		return []domain.ERPCostSKU{}, time.Unix(0, 0).UTC(), nil
	}
	tx, err := r.db.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("begin jst cost batch snapshot: %w", err)
	}
	defer rollback(tx)

	var watermark sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT MAX(local_updated_at) FROM jst_inventory`).Scan(&watermark); err != nil {
		return nil, time.Time{}, fmt.Errorf("read jst batch watermark: %w", err)
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(skuIDs)), ",")
	args := make([]interface{}, 0, len(skuIDs))
	for _, skuID := range skuIDs {
		args = append(args, skuID)
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT sku_id, NULLIF(TRIM(sku_type), ''), CAST(cost_price AS CHAR), CAST(sale_price AS CHAR), local_updated_at
		  FROM jst_inventory
		 WHERE sku_id IN (`+placeholders+`)
		 ORDER BY sku_id ASC`, args...)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("batch query jst inventory costs: %w", err)
	}
	items, err := scanERPCostSKUs(rows)
	rows.Close()
	if err != nil {
		return nil, time.Time{}, err
	}
	if err := tx.Commit(); err != nil {
		return nil, time.Time{}, fmt.Errorf("commit jst cost batch snapshot: %w", err)
	}
	if !watermark.Valid {
		return items, time.Unix(0, 0).UTC(), nil
	}
	return items, watermark.Time, nil
}

func (r *erpCostReadRepo) CostChangeWatermark(ctx context.Context) (int64, error) {
	var watermark sql.NullInt64
	err := r.db.db.QueryRowContext(ctx, `SELECT MAX(id) FROM jst_cost_changes`).Scan(&watermark)
	if err != nil {
		return 0, fmt.Errorf("read jst cost change watermark: %w", err)
	}
	if !watermark.Valid {
		return 0, nil
	}
	return watermark.Int64, nil
}

func (r *erpCostReadRepo) ListCostChanges(ctx context.Context, query repo.ERPCostChangePageQuery) ([]domain.ERPCostChange, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, sku_id, NULLIF(TRIM(sku_type), ''),
		       CAST(old_cost_price AS CHAR), CAST(new_cost_price AS CHAR),
		       source_modified_at, changed_at
		  FROM jst_cost_changes
		 WHERE changed_at > ?
		   AND id > ?
		   AND id <= ?
		 ORDER BY id ASC
		 LIMIT ?`, query.ChangedSince, query.LastID, query.WatermarkID, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("list jst cost changes: %w", err)
	}
	defer rows.Close()

	items := make([]domain.ERPCostChange, 0, query.Limit)
	for rows.Next() {
		var (
			item       domain.ERPCostChange
			skuType    sql.NullString
			oldCost    sql.NullString
			newCost    sql.NullString
			sourceTime sql.NullTime
		)
		if err := rows.Scan(&item.ID, &item.SKUID, &skuType, &oldCost, &newCost, &sourceTime, &item.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan jst cost change: %w", err)
		}
		item.SKUType = nullStringPointer(skuType)
		item.OldCostPrice = nullStringPointer(oldCost)
		item.NewCostPrice = nullStringPointer(newCost)
		if sourceTime.Valid {
			value := sourceTime.Time
			item.SourceModifiedAt = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jst cost changes: %w", err)
	}
	return items, nil
}

type erpCostRows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}

func scanERPCostSKUs(rows erpCostRows) ([]domain.ERPCostSKU, error) {
	items := make([]domain.ERPCostSKU, 0)
	for rows.Next() {
		var (
			item      domain.ERPCostSKU
			skuType   sql.NullString
			costPrice sql.NullString
			salePrice sql.NullString
			modified  sql.NullTime
		)
		if err := rows.Scan(&item.SKUID, &skuType, &costPrice, &salePrice, &modified); err != nil {
			return nil, fmt.Errorf("scan jst inventory cost: %w", err)
		}
		item.SKUType = nullStringPointer(skuType)
		item.CostPrice = nullStringPointer(costPrice)
		item.SalePrice = nullStringPointer(salePrice)
		if modified.Valid {
			item.ModifiedAt = modified.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jst inventory costs: %w", err)
	}
	return items, nil
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}
