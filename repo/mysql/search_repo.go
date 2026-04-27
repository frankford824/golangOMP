package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type searchRepo struct{ db *DB }

func NewSearchRepo(db *DB) repo.SearchRepo { return &searchRepo{db: db} }

func (r *searchRepo) SearchTasks(ctx context.Context, q string, limit int) ([]domain.SearchTask, error) {
	limit = normalizeSearchLimit(limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_no, product_name_snapshot, task_status, priority
		  FROM tasks
		 WHERE task_no LIKE CONCAT('%', ?, '%')
		    OR product_name_snapshot LIKE CONCAT('%', ?, '%')
		    OR sku_code LIKE CONCAT('%', ?, '%')
		    OR primary_sku_code LIKE CONCAT('%', ?, '%')
		    OR CAST(id AS CHAR) LIKE CONCAT('%', ?, '%')
		 ORDER BY id DESC
		 LIMIT ?`, q, q, q, q, q, limit)
	if err != nil {
		return nil, fmt.Errorf("search tasks: %w", err)
	}
	defer rows.Close()

	var out []domain.SearchTask
	for rows.Next() {
		var item domain.SearchTask
		var title, status, priority sql.NullString
		if err := rows.Scan(&item.ID, &item.TaskNo, &title, &status, &priority); err != nil {
			return nil, fmt.Errorf("scan search task: %w", err)
		}
		item.Title = nullStringPtr(title)
		item.TaskStatus = nullStringPtr(status)
		item.Priority = nullStringPtr(priority)
		item.Highlight = nil
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *searchRepo) SearchAssets(ctx context.Context, q string, limit int) ([]domain.SearchAsset, error) {
	limit = normalizeSearchLimit(limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT COALESCE(asset_id, id) AS asset_id, file_name, source_module_key, task_id
		  FROM task_assets
		 WHERE file_name LIKE CONCAT('%', ?, '%')
		   AND COALESCE(deleted_at, cleaned_at) IS NULL
		 ORDER BY COALESCE(asset_id, id) DESC
		 LIMIT ?`, q, limit)
	if err != nil {
		return nil, fmt.Errorf("search assets: %w", err)
	}
	defer rows.Close()

	var out []domain.SearchAsset
	for rows.Next() {
		var item domain.SearchAsset
		var module sql.NullString
		var taskID sql.NullInt64
		if err := rows.Scan(&item.AssetID, &item.FileName, &module, &taskID); err != nil {
			return nil, fmt.Errorf("scan search asset: %w", err)
		}
		item.SourceModuleKey = nullStringPtr(module)
		item.TaskID = nullInt64Ptr(taskID)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *searchRepo) SearchProducts(ctx context.Context, q string, limit int) ([]domain.SearchProduct, error) {
	limit = normalizeSearchLimit(limit)
	if r.tableExists(ctx, "erp_product_snapshots") {
		rows, err := r.db.db.QueryContext(ctx, `
			SELECT erp_code, product_name, category
			  FROM erp_product_snapshots
			 WHERE erp_code LIKE CONCAT('%', ?, '%')
			    OR product_name LIKE CONCAT('%', ?, '%')
			 ORDER BY erp_code DESC
			 LIMIT ?`, q, q, limit)
		if err == nil {
			return scanSearchProducts(rows)
		}
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT sku_code AS erp_code, MAX(product_name_snapshot) AS product_name, NULL AS category
		  FROM tasks
		 WHERE sku_code LIKE CONCAT('%', ?, '%')
		    OR primary_sku_code LIKE CONCAT('%', ?, '%')
		    OR product_name_snapshot LIKE CONCAT('%', ?, '%')
		 GROUP BY sku_code
		 ORDER BY MAX(id) DESC
		 LIMIT ?`, q, q, q, limit)
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	return scanSearchProducts(rows)
}

func (r *searchRepo) SearchUsers(ctx context.Context, q string, limit int) ([]domain.SearchUser, error) {
	limit = normalizeSearchLimit(limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, username, department
		  FROM users
		 WHERE status = 'active'
		   AND (username LIKE CONCAT('%', ?, '%')
		    OR display_name LIKE CONCAT('%', ?, '%')
		    OR email LIKE CONCAT('%', ?, '%'))
		 ORDER BY id DESC
		 LIMIT ?`, q, q, q, limit)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	var out []domain.SearchUser
	for rows.Next() {
		var item domain.SearchUser
		var department sql.NullString
		if err := rows.Scan(&item.UserID, &item.Username, &department); err != nil {
			return nil, fmt.Errorf("scan search user: %w", err)
		}
		item.DepartmentName = nullStringPtr(department)
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanSearchProducts(rows *sql.Rows) ([]domain.SearchProduct, error) {
	defer rows.Close()
	var out []domain.SearchProduct
	for rows.Next() {
		var item domain.SearchProduct
		var category sql.NullString
		if err := rows.Scan(&item.ERPCode, &item.ProductName, &category); err != nil {
			return nil, fmt.Errorf("scan search product: %w", err)
		}
		item.Category = nullStringPtr(category)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *searchRepo) tableExists(ctx context.Context, table string) bool {
	table = strings.TrimSpace(table)
	if table == "" {
		return false
	}
	var found string
	err := r.db.db.QueryRowContext(ctx, `
		SELECT table_name
		  FROM information_schema.tables
		 WHERE table_schema = DATABASE()
		   AND table_name = ?
		 LIMIT 1`, table).Scan(&found)
	return err == nil
}

func normalizeSearchLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}
