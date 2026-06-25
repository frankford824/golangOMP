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
	kw := normalizeSearchKeyword(q)
	items, seen, err := r.searchTasksFromDocumentsPrimary(ctx, kw, limit)
	if err != nil {
		return nil, err
	}
	if len(items) >= limit {
		return items, nil
	}
	fallback, err := r.searchTasksFromDocumentsLike(ctx, kw, limit-len(items), seen)
	if err != nil {
		return nil, err
	}
	items = append(items, fallback...)
	return items, nil
}

func (r *searchRepo) searchTasksFromDocumentsPrimary(ctx context.Context, kw normalizedSearchKeyword, limit int) ([]domain.SearchTask, map[int64]struct{}, error) {
	clauses := make([]string, 0, 4)
	args := make([]interface{}, 0, 16)
	if kw.Raw == "" {
		return []domain.SearchTask{}, map[int64]struct{}{}, nil
	}
	if kw.HasInt64 {
		clauses = append(clauses, "task_id = ?")
		args = append(args, kw.Int64)
	}
	if kw.IsCode {
		clauses = appendAnyClause(clauses,
			"task_no = ?",
			"sku_code = ?",
			"primary_sku_code = ?",
			"product_i_id = ?",
			"task_no LIKE ?",
			"sku_code LIKE ?",
			"primary_sku_code LIKE ?",
			"product_i_id LIKE ?",
		)
		args = append(args, kw.Upper, kw.Upper, kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", kw.Upper+"%", kw.Upper+"%")
	}
	if !kw.IsCode && len([]rune(kw.Raw)) >= 2 {
		clauses = append(clauses, "MATCH(search_text) AGAINST (? IN NATURAL LANGUAGE MODE)")
		args = append(args, kw.Raw)
	}
	if len(clauses) == 0 {
		return []domain.SearchTask{}, map[int64]struct{}{}, nil
	}
	args = append(args, limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT task_id, task_no, product_name_snapshot, task_status, priority,
		       task_type, sku_code, primary_sku_code, product_i_id,
		       owner_department, owner_team, owner_org_team,
		       creator_id, creator_name, designer_id, designer_name,
		       created_at, deadline_at
		  FROM task_search_documents
		 WHERE `+strings.Join(clauses, " OR ")+`
		 ORDER BY updated_at DESC, task_id DESC
		 LIMIT ?`, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("search task documents primary: %w", err)
	}
	items, err := scanSearchTasks(rows)
	if err != nil {
		return nil, nil, err
	}
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		seen[item.ID] = struct{}{}
	}
	return items, seen, nil
}

func (r *searchRepo) searchTasksFromDocumentsLike(ctx context.Context, kw normalizedSearchKeyword, limit int, seen map[int64]struct{}) ([]domain.SearchTask, error) {
	if limit <= 0 || kw.Raw == "" {
		return []domain.SearchTask{}, nil
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT task_id, task_no, product_name_snapshot, task_status, priority,
		       task_type, sku_code, primary_sku_code, product_i_id,
		       owner_department, owner_team, owner_org_team,
		       creator_id, creator_name, designer_id, designer_name,
		       created_at, deadline_at
		  FROM task_search_documents
		 WHERE search_text LIKE ?
		 ORDER BY updated_at DESC, task_id DESC
		 LIMIT ?`, kw.Like, limit+len(seen))
	if err != nil {
		return nil, fmt.Errorf("search task documents fallback: %w", err)
	}
	items, err := scanSearchTasks(rows)
	if err != nil {
		return nil, err
	}
	out := make([]domain.SearchTask, 0, limit)
	for _, item := range items {
		if _, ok := seen[item.ID]; ok {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *searchRepo) searchTasksLegacy(ctx context.Context, q string, limit int) ([]domain.SearchTask, error) {
	limit = normalizeSearchLimit(limit)
	like := "%" + strings.TrimSpace(q) + "%"
	activeAssetWhere := taskAssetsActiveSQL(ctx, r.db.db, "ta")
	query := strings.Replace(`
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
			           AND {{active_asset_where}}
			           AND (ta.file_name LIKE ? OR COALESCE(ta.original_filename, '') LIKE ? OR COALESCE(ta.storage_key, '') LIKE ? OR COALESCE(ta.source_module_key, '') LIKE ?)
			    )
			 ORDER BY t.id DESC
			 LIMIT ?`, "{{active_asset_where}}", activeAssetWhere, 1)
	rows, err := r.db.db.QueryContext(ctx, query, repeatArgs(like, 42, limit)...)
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
	kw := normalizeSearchKeyword(q)
	activeAssetWhere := taskAssetsActiveSQL(ctx, r.db.db, "ta")
	searchClauses := make([]string, 0, 4)
	searchArgs := make([]interface{}, 0, 20)
	if kw.HasInt64 {
		searchClauses = appendAnyClause(searchClauses, "ta.asset_id = ?", "ta.id = ?", "ta.task_id = ?")
		searchArgs = append(searchArgs, kw.Int64, kw.Int64, kw.Int64)
	}
	if kw.IsCode {
		searchClauses = appendAnyClause(searchClauses,
			"t.task_no = ?",
			"t.sku_code = ?",
			"t.primary_sku_code = ?",
			"ta.scope_sku_code = ?",
			"t.task_no LIKE ?",
			"t.sku_code LIKE ?",
			"t.primary_sku_code LIKE ?",
			"ta.scope_sku_code LIKE ?",
		)
		searchArgs = append(searchArgs, kw.Upper, kw.Upper, kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", kw.Upper+"%", kw.Upper+"%")
	}
	searchClauses = appendAnyClause(searchClauses,
		"ta.file_name LIKE ?",
		"COALESCE(ta.original_filename, '') LIKE ?",
		"COALESCE(ta.storage_key, '') LIKE ?",
		"COALESCE(ta.source_module_key, '') LIKE ?",
		"COALESCE(t.product_name_snapshot, '') LIKE ?",
		"COALESCE(t.owner_team, '') LIKE ?",
		"COALESCE(t.owner_department, '') LIKE ?",
		"COALESCE(t.owner_org_team, '') LIKE ?",
		"COALESCE(creator.username, '') LIKE ?",
		"COALESCE(creator.display_name, '') LIKE ?",
		"COALESCE(designer.username, '') LIKE ?",
		"COALESCE(designer.display_name, '') LIKE ?",
	)
	searchArgs = append(searchArgs, kw.Like, kw.Like, kw.Like, kw.Like, kw.Like, kw.Like, kw.Like, kw.Like, kw.Like, kw.Like, kw.Like, kw.Like)
	query := strings.Replace(`
			SELECT COALESCE(ta.asset_id, ta.id) AS asset_id, ta.file_name, ta.source_module_key, ta.task_id,
			       ta.asset_type, ta.flow_review_status
			  FROM task_assets ta
			  LEFT JOIN design_assets da ON da.id = ta.asset_id
			  LEFT JOIN tasks t ON t.id = ta.task_id
			  LEFT JOIN users creator ON creator.id = t.creator_id
			  LEFT JOIN users designer ON designer.id = t.designer_id
			 WHERE {{active_asset_where}}
			   AND (
			       ta.asset_id IS NULL
			       OR ta.id = COALESCE(da.current_version_id, (
			           SELECT ta2.id FROM task_assets ta2 WHERE ta2.asset_id = da.id ORDER BY ta2.asset_version_no DESC, ta2.id DESC LIMIT 1
			       ))
			   )
			   AND (`+strings.Join(searchClauses, " OR ")+`)
			 ORDER BY COALESCE(ta.asset_id, ta.id) DESC
			 LIMIT ?`, "{{active_asset_where}}", activeAssetWhere, 1)
	searchArgs = append(searchArgs, limit)
	rows, err := r.db.db.QueryContext(ctx, query, searchArgs...)
	if err != nil {
		return nil, fmt.Errorf("search assets: %w", err)
	}
	defer rows.Close()

	var out []domain.SearchAsset
	for rows.Next() {
		var item domain.SearchAsset
		var module sql.NullString
		var taskID sql.NullInt64
		var assetType, flowReviewStatus sql.NullString
		if err := rows.Scan(&item.AssetID, &item.FileName, &module, &taskID, &assetType, &flowReviewStatus); err != nil {
			return nil, fmt.Errorf("scan search asset: %w", err)
		}
		item.SourceModuleKey = nullStringPtr(module)
		item.TaskID = nullInt64Ptr(taskID)
		item.ResourceID = fmt.Sprintf("%d", item.AssetID)
		item.SourceType = string(domain.AssetResourceSourceSystem)
		item.SourceLabel = "系统资源"
		status := domain.NormalizeTaskAssetFlowReviewStatus(domain.TaskAssetFlowReviewStatus(nullStringValue(flowReviewStatus)), domain.TaskAssetType(nullStringValue(assetType)))
		item.FlowReviewStatus = string(status)
		item.UsableState = string(usableStateFromFlowStatus(status))
		item.UsableLabel = usableLabelFromState(usableStateFromFlowStatus(status))
		out = append(out, item)
	}
	return out, rows.Err()
}

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func usableStateFromFlowStatus(status domain.TaskAssetFlowReviewStatus) domain.TaskAssetUsableState {
	switch status {
	case domain.TaskAssetFlowReviewStatusApproved:
		return domain.TaskAssetUsableStateReadyForUse
	case domain.TaskAssetFlowReviewStatusRejected:
		return domain.TaskAssetUsableStateRejected
	case domain.TaskAssetFlowReviewStatusSuperseded:
		return domain.TaskAssetUsableStateHistory
	case domain.TaskAssetFlowReviewStatusCleaned:
		return domain.TaskAssetUsableStateCleaned
	case domain.TaskAssetFlowReviewStatusPendingReview:
		return domain.TaskAssetUsableStatePendingReview
	default:
		return domain.TaskAssetUsableStateNotApplicable
	}
}

func usableLabelFromState(state domain.TaskAssetUsableState) string {
	switch state {
	case domain.TaskAssetUsableStateReadyForUse:
		return "可直接使用"
	case domain.TaskAssetUsableStatePendingReview:
		return "待审核"
	case domain.TaskAssetUsableStateRejected:
		return "审核未通过"
	case domain.TaskAssetUsableStateHistory:
		return "历史版本"
	case domain.TaskAssetUsableStateCleaned:
		return "文件已清理"
	default:
		return "不进入审核流"
	}
}

func (r *searchRepo) SearchProducts(ctx context.Context, q string, limit int) ([]domain.SearchProduct, error) {
	limit = normalizeSearchLimit(limit)
	kw := normalizeSearchKeyword(q)
	if r.tableExists(ctx, "products") {
		rows, err := r.db.db.QueryContext(ctx, `
			SELECT sku_code AS erp_code,
			       product_name,
			       COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.i_id')), ''), '') AS i_id,
			       COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.category_name')), ''), NULLIF(category, '')) AS category
			  FROM products
			 WHERE sku_code = ?
			    OR COALESCE(JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.i_id')), '') = ?
			    OR sku_code LIKE ?
			    OR COALESCE(JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.i_id')), '') LIKE ?
			    OR sku_code LIKE ?
			    OR product_name LIKE ?
			    OR category LIKE ?
			    OR (JSON_VALID(spec_json) AND JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.i_id')) LIKE ?)
			    OR (JSON_VALID(spec_json) AND JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.category_name')) LIKE ?)
			 ORDER BY CASE
			            WHEN sku_code = ? THEN 0
			            WHEN COALESCE(JSON_UNQUOTE(JSON_EXTRACT(spec_json, '$.i_id')), '') = ? THEN 1
			            WHEN sku_code LIKE ? THEN 2
			            ELSE 3
			          END,
			          id DESC
			 LIMIT ?`,
			kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", kw.Like, kw.Like, kw.Like, kw.Like, kw.Like,
			kw.Upper, kw.Upper, kw.Upper+"%", limit)
		if err == nil {
			return scanSearchProducts(rows)
		}
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT sku_code AS erp_code, MAX(product_name_snapshot) AS product_name, NULL AS i_id, NULL AS category
		  FROM tasks
		 WHERE sku_code = ?
		    OR primary_sku_code = ?
		    OR sku_code LIKE ?
		    OR primary_sku_code LIKE ?
		    OR sku_code LIKE ?
		    OR primary_sku_code LIKE ?
		    OR product_name_snapshot LIKE ?
		 GROUP BY sku_code
		 ORDER BY CASE
		            WHEN sku_code = ? THEN 0
		            WHEN primary_sku_code = ? THEN 1
		            WHEN sku_code LIKE ? THEN 2
		            ELSE 3
		          END,
		          MAX(id) DESC
		 LIMIT ?`, kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", kw.Like, kw.Like, kw.Like, kw.Upper, kw.Upper, kw.Upper+"%", limit)
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
