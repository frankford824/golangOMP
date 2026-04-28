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

type searchRepo struct{ db *DB }

func NewSearchRepo(db *DB) repo.SearchRepo { return &searchRepo{db: db} }

func (r *searchRepo) SearchTasks(ctx context.Context, q string, limit int) ([]domain.SearchTask, error) {
	if r.tableExists(ctx, "task_search_documents") {
		return r.searchTasksFromDocuments(ctx, q, limit)
	}
	return r.searchTasksLegacy(ctx, q, limit)
}

func (r *searchRepo) searchTasksFromDocuments(ctx context.Context, q string, limit int) ([]domain.SearchTask, error) {
	limit = normalizeSearchLimit(limit)
	like := "%" + strings.TrimSpace(q) + "%"
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT task_id, task_no, product_name_snapshot, task_status, priority,
		       task_type, sku_code, primary_sku_code, product_i_id,
		       owner_department, owner_team, owner_org_team,
		       creator_id, creator_name, designer_id, designer_name,
		       created_at, deadline_at
		  FROM task_search_documents
		 WHERE search_text LIKE ?
		 ORDER BY updated_at DESC, task_id DESC
		 LIMIT ?`, like, limit)
	if err != nil {
		return nil, fmt.Errorf("search task documents: %w", err)
	}
	return scanSearchTasks(rows)
}

func (r *searchRepo) searchTasksLegacy(ctx context.Context, q string, limit int) ([]domain.SearchTask, error) {
	limit = normalizeSearchLimit(limit)
	like := "%" + strings.TrimSpace(q) + "%"
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT t.id, t.task_no, t.product_name_snapshot, t.task_status, t.priority,
		       t.task_type, t.sku_code, t.primary_sku_code,
		       COALESCE(
		         NULLIF(td.category, ''),
		         NULLIF(td.category_name, ''),
		         NULLIF(CASE WHEN JSON_VALID(td.product_selection_snapshot_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) ELSE '' END, ''),
		         NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) ELSE '' END, ''),
		         NULLIF(CASE WHEN JSON_VALID(td.last_filing_payload_json) THEN JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) ELSE '' END, '')
		       ) AS product_i_id,
		       t.owner_department, t.owner_team, t.owner_org_team,
		       t.creator_id, COALESCE(NULLIF(creator.display_name, ''), creator.username, '') AS creator_name,
		       t.designer_id, COALESCE(NULLIF(designer.display_name, ''), designer.username, '') AS designer_name,
		       t.created_at, t.deadline_at
		  FROM tasks t
		  LEFT JOIN task_details td ON td.task_id = t.id
		  LEFT JOIN users creator ON creator.id = t.creator_id
		  LEFT JOIN users designer ON designer.id = t.designer_id
		 WHERE t.task_no LIKE ?
		    OR t.product_name_snapshot LIKE ?
		    OR t.sku_code LIKE ?
		    OR t.primary_sku_code LIKE ?
		    OR CAST(t.id AS CHAR) LIKE ?
		    OR t.task_type LIKE ?
		    OR t.task_status LIKE ?
		    OR t.priority LIKE ?
		    OR COALESCE(t.owner_team, '') LIKE ?
		    OR COALESCE(t.owner_department, '') LIKE ?
		    OR COALESCE(t.owner_org_team, '') LIKE ?
		    OR COALESCE(creator.username, '') LIKE ?
		    OR COALESCE(creator.display_name, '') LIKE ?
		    OR COALESCE(designer.username, '') LIKE ?
		    OR COALESCE(designer.display_name, '') LIKE ?
		    OR DATE_FORMAT(t.created_at, '%Y-%m-%d') LIKE ?
		    OR DATE_FORMAT(t.created_at, '%Y%m%d') LIKE ?
		    OR DATE_FORMAT(t.deadline_at, '%Y-%m-%d') LIKE ?
		    OR COALESCE(td.category, '') LIKE ?
		    OR COALESCE(td.category_name, '') LIKE ?
		    OR COALESCE(td.category_code, '') LIKE ?
		    OR COALESCE(td.product_short_name, '') LIKE ?
		    OR COALESCE(td.demand_text, '') LIKE ?
		    OR COALESCE(td.copy_text, '') LIKE ?
		    OR COALESCE(td.remark, '') LIKE ?
		    OR COALESCE(td.change_request, '') LIKE ?
		    OR COALESCE(td.design_requirement, '') LIKE ?
		    OR COALESCE(td.material, '') LIKE ?
		    OR COALESCE(td.spec_text, '') LIKE ?
		    OR COALESCE(td.size_text, '') LIKE ?
		    OR COALESCE(td.craft_text, '') LIKE ?
		    OR COALESCE(td.process, '') LIKE ?
		    OR COALESCE(td.reference_link, '') LIKE ?
		    OR (JSON_VALID(td.product_selection_snapshot_json) AND JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.i_id')) LIKE ?)
		    OR (JSON_VALID(td.product_selection_snapshot_json) AND JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.name')) LIKE ?)
		    OR (JSON_VALID(td.product_selection_snapshot_json) AND JSON_UNQUOTE(JSON_EXTRACT(td.product_selection_snapshot_json, '$.erp_product.product_name')) LIKE ?)
		    OR (JSON_VALID(td.last_filing_payload_json) AND JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.product.i_id')) LIKE ?)
		    OR (JSON_VALID(td.last_filing_payload_json) AND JSON_UNQUOTE(JSON_EXTRACT(td.last_filing_payload_json, '$.i_id')) LIKE ?)
		    OR EXISTS (
		        SELECT 1
		          FROM task_assets ta
		         WHERE ta.task_id = t.id
		           AND COALESCE(ta.deleted_at, ta.cleaned_at) IS NULL
		           AND (ta.file_name LIKE ? OR COALESCE(ta.original_filename, '') LIKE ? OR COALESCE(ta.storage_key, '') LIKE ? OR COALESCE(ta.source_module_key, '') LIKE ?)
		    )
		 ORDER BY t.id DESC
		 LIMIT ?`, repeatArgs(like, 42, limit)...)
	if err != nil {
		return nil, fmt.Errorf("search tasks: %w", err)
	}
	return scanSearchTasks(rows)
}

func scanSearchTasks(rows *sql.Rows) ([]domain.SearchTask, error) {
	defer rows.Close()

	var out []domain.SearchTask
	for rows.Next() {
		var item domain.SearchTask
		var title, status, priority, taskType, skuCode, primarySKUCode, productIID sql.NullString
		var ownerDepartment, ownerTeam, ownerOrgTeam, creatorName, designerName sql.NullString
		var creatorID, designerID sql.NullInt64
		var createdAt, deadlineAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.TaskNo, &title, &status, &priority,
			&taskType, &skuCode, &primarySKUCode, &productIID,
			&ownerDepartment, &ownerTeam, &ownerOrgTeam,
			&creatorID, &creatorName, &designerID, &designerName,
			&createdAt, &deadlineAt,
		); err != nil {
			return nil, fmt.Errorf("scan search task: %w", err)
		}
		item.Title = nullStringPtr(title)
		item.TaskStatus = nullStringPtr(status)
		item.Priority = nullStringPtr(priority)
		item.TaskType = nullStringPtr(taskType)
		item.SKUCode = nullStringPtr(skuCode)
		item.PrimarySKUCode = nullStringPtr(primarySKUCode)
		item.ProductIID = nullStringPtr(productIID)
		item.OwnerDepartment = nullStringPtr(ownerDepartment)
		item.OwnerTeam = nullStringPtr(ownerTeam)
		item.OwnerOrgTeam = nullStringPtr(ownerOrgTeam)
		item.CreatorID = nullInt64Ptr(creatorID)
		item.CreatorName = nullStringPtr(creatorName)
		item.DesignerID = nullInt64Ptr(designerID)
		item.DesignerName = nullStringPtr(designerName)
		item.CreatedAt = nullTimePtr(createdAt)
		item.DeadlineAt = nullTimePtr(deadlineAt)
		item.Highlight = nil
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *searchRepo) SearchAssets(ctx context.Context, q string, limit int) ([]domain.SearchAsset, error) {
	limit = normalizeSearchLimit(limit)
	like := "%" + strings.TrimSpace(q) + "%"
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT COALESCE(ta.asset_id, ta.id) AS asset_id, ta.file_name, ta.source_module_key, ta.task_id
		  FROM task_assets ta
		  LEFT JOIN tasks t ON t.id = ta.task_id
		  LEFT JOIN users creator ON creator.id = t.creator_id
		  LEFT JOIN users designer ON designer.id = t.designer_id
		 WHERE COALESCE(ta.deleted_at, ta.cleaned_at) IS NULL
		   AND (ta.file_name LIKE ?
		    OR COALESCE(ta.original_filename, '') LIKE ?
		    OR COALESCE(ta.storage_key, '') LIKE ?
		    OR COALESCE(ta.source_module_key, '') LIKE ?
		    OR COALESCE(t.task_no, '') LIKE ?
		    OR COALESCE(t.product_name_snapshot, '') LIKE ?
		    OR COALESCE(t.sku_code, '') LIKE ?
		    OR COALESCE(t.primary_sku_code, '') LIKE ?
		    OR COALESCE(t.task_type, '') LIKE ?
		    OR COALESCE(t.owner_team, '') LIKE ?
		    OR COALESCE(t.owner_department, '') LIKE ?
		    OR COALESCE(t.owner_org_team, '') LIKE ?
		    OR COALESCE(creator.username, '') LIKE ?
		    OR COALESCE(creator.display_name, '') LIKE ?
		    OR COALESCE(designer.username, '') LIKE ?
		    OR COALESCE(designer.display_name, '') LIKE ?)
		 ORDER BY COALESCE(ta.asset_id, ta.id) DESC
		 LIMIT ?`, repeatArgs(like, 16, limit)...)
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
	like := "%" + strings.TrimSpace(q) + "%"
	if r.tableExists(ctx, "products") {
		rows, err := r.db.db.QueryContext(ctx, `
			SELECT sku_code AS erp_code,
			       product_name,
			       COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.i_id')), ''), '') AS i_id,
			       COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.category_name')), ''), NULLIF(category, '')) AS category
			  FROM products
			 WHERE sku_code LIKE ?
			    OR product_name LIKE ?
			    OR category LIKE ?
			    OR (JSON_VALID(spec_json) AND JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.i_id')) LIKE ?)
			    OR (JSON_VALID(spec_json) AND JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.category_name')) LIKE ?)
			 ORDER BY id DESC
			 LIMIT ?`, like, like, like, like, like, limit)
		if err == nil {
			return scanSearchProducts(rows)
		}
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT sku_code AS erp_code, MAX(product_name_snapshot) AS product_name, NULL AS i_id, NULL AS category
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
		var iid, category sql.NullString
		if err := rows.Scan(&item.ERPCode, &item.ProductName, &iid, &category); err != nil {
			return nil, fmt.Errorf("scan search product: %w", err)
		}
		item.IID = nullStringPtr(iid)
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

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}

func repeatArgs(value string, count int, tail ...interface{}) []interface{} {
	args := make([]interface{}, 0, count+len(tail))
	for i := 0; i < count; i++ {
		args = append(args, value)
	}
	args = append(args, tail...)
	return args
}
