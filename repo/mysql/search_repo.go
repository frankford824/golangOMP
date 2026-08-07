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

// SearchTasksScoped is the only task-search path used by the v8 global search.
// It joins the live task row so the same stable-ID scope predicate used by
// resource groups is applied before LIMIT. Historical unscoped search helpers
// remain only for non-v8 diagnostics and tests.
func (r *searchRepo) SearchTasksScoped(ctx context.Context, q string, limit int, access domain.ResourceGroupAccessFilter) ([]domain.SearchTask, error) {
	if !r.tableExists(ctx, "task_search_documents") {
		return []domain.SearchTask{}, nil
	}
	limit = normalizeSearchLimit(limit)
	keyword := strings.TrimSpace(q)
	if keyword == "" {
		return []domain.SearchTask{}, nil
	}
	like := "%" + keyword + "%"
	where := []string{`(
		d.task_no LIKE ? OR COALESCE(d.sku_code,'') LIKE ? OR COALESCE(d.primary_sku_code,'') LIKE ?
		OR COALESCE(d.product_i_id,'') LIKE ? OR d.search_text LIKE ?
	)`}
	args := []interface{}{like, like, like, like, like}
	where, args = appendResourceGroupAccessScope(where, args, access)
	args = append(args, limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT `+taskSearchDocumentSelectCols+`
		FROM task_search_documents d
		JOIN tasks t ON t.id = d.task_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY d.updated_at DESC, d.task_id DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("search scoped task documents: %w", err)
	}
	return scanSearchTasks(rows)
}

func (r *searchRepo) searchTasksFromDocuments(ctx context.Context, q string, limit int) ([]domain.SearchTask, error) {
	limit = normalizeSearchLimit(limit)
	kw := normalizeSearchKeyword(q)
	return r.searchTasksFromDocumentsPrimary(ctx, kw, limit)
}

func (r *searchRepo) searchTasksFromDocumentsPrimary(ctx context.Context, kw normalizedSearchKeyword, limit int) ([]domain.SearchTask, error) {
	if kw.Raw == "" {
		return []domain.SearchTask{}, nil
	}
	if kw.HasInt64 || kw.IsCode {
		return r.searchTaskDocumentsByCode(ctx, kw, limit)
	}
	if len([]rune(kw.Raw)) >= 2 {
		return r.searchTaskDocumentsByText(ctx, kw, limit)
	}
	return []domain.SearchTask{}, nil
}

const taskSearchDocumentSelectCols = `d.task_id, d.task_no, d.product_name_snapshot, d.task_status, d.priority,
		       d.task_type, d.sku_code, d.primary_sku_code, d.product_i_id,
		       d.owner_department, d.owner_team, d.owner_org_team,
		       d.creator_id, d.creator_name, d.designer_id, d.designer_name,
		       d.created_at, d.deadline_at`

const searchNaturalFallbackThreshold = 1

// searchTaskDocumentsByCode recalls task ids through a UNION ALL of per-column
// exact and prefix predicates so each branch uses a BTREE index, then hydrates
// full document columns for the recalled ids. This avoids the previous OR
// predicate that reverse-scanned idx_task_search_updated across the whole table.
func (r *searchRepo) searchTaskDocumentsByCode(ctx context.Context, kw normalizedSearchKeyword, limit int) ([]domain.SearchTask, error) {
	branches := make([]string, 0, 9)
	args := make([]interface{}, 0, 9)
	if kw.HasInt64 {
		branches = append(branches, "SELECT task_id, 0 AS match_rank FROM task_search_documents WHERE task_id = ?")
		args = append(args, kw.Int64)
	}
	if kw.IsCode {
		exact := []string{"task_no", "sku_code", "primary_sku_code", "product_i_id"}
		for i, col := range exact {
			branches = append(branches, fmt.Sprintf("SELECT task_id, %d AS match_rank FROM task_search_documents WHERE %s = ?", i+1, col))
			args = append(args, kw.Upper)
		}
		for i, col := range exact {
			branches = append(branches, fmt.Sprintf("SELECT task_id, %d AS match_rank FROM task_search_documents WHERE %s LIKE ?", i+5, col))
			args = append(args, kw.Upper+"%")
		}
	}
	if len(branches) == 0 {
		return []domain.SearchTask{}, nil
	}
	args = append(args, limit)
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, `
		SELECT `+taskSearchDocumentSelectCols+`
		  FROM task_search_documents d
		  JOIN (
		    SELECT task_id, MIN(match_rank) AS match_rank
		      FROM (
		        `+strings.Join(branches, "\n		        UNION ALL\n		        ")+`
		      ) u
		     GROUP BY task_id
		  ) m ON m.task_id = d.task_id
		 ORDER BY m.match_rank ASC, d.updated_at DESC, d.task_id DESC
		 LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("search task documents by code: %w", err)
	}
	return scanSearchTasks(rows)
}

func (r *searchRepo) searchTaskDocumentsByText(ctx context.Context, kw normalizedSearchKeyword, limit int) ([]domain.SearchTask, error) {
	items, err := r.searchTaskDocumentsByTextMode(ctx, kw, limit, true, nil)
	if err != nil {
		return nil, err
	}
	if len(items) >= limit || len(items) >= searchNaturalFallbackThreshold {
		return items, nil
	}
	fallback, err := r.searchTaskDocumentsByTextMode(ctx, kw, limit-len(items), false, searchTaskResultIDs(items))
	if err != nil {
		return nil, err
	}
	return appendSearchTasksUnique(items, fallback, limit), nil
}

func (r *searchRepo) searchTaskDocumentsByTextMode(ctx context.Context, kw normalizedSearchKeyword, limit int, booleanMode bool, excludeTaskIDs []int64) ([]domain.SearchTask, error) {
	if limit <= 0 {
		return []domain.SearchTask{}, nil
	}
	matchQuery := kw.Raw
	matchMode := "NATURAL LANGUAGE"
	matchRank := 20
	if booleanMode {
		matchQuery = booleanPhraseSearchQuery(kw.Raw)
		matchMode = "BOOLEAN"
		matchRank = 10
	}
	excludeSQL := ""
	args := []interface{}{matchQuery, matchQuery}
	if len(excludeTaskIDs) > 0 {
		excludeSQL = " AND task_id NOT IN (" + searchPlaceholders(len(excludeTaskIDs)) + ")"
		for _, id := range excludeTaskIDs {
			args = append(args, id)
		}
	}
	args = append(args, limit)
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, `
		SELECT `+taskSearchDocumentSelectCols+`
		  FROM task_search_documents d
		  JOIN (
		    SELECT task_id, `+fmt.Sprintf("%d", matchRank)+` AS match_rank,
		           MATCH(search_text) AGAINST (? IN `+matchMode+` MODE) AS match_score,
		           updated_at
		      FROM task_search_documents
		     WHERE MATCH(search_text) AGAINST (? IN `+matchMode+` MODE)`+excludeSQL+`
		     ORDER BY match_score DESC, updated_at DESC, task_id DESC
		     LIMIT ?
		  ) m ON m.task_id = d.task_id
		 ORDER BY m.match_rank ASC, m.match_score DESC, d.updated_at DESC, d.task_id DESC
		`, args...)
	if err != nil {
		return nil, fmt.Errorf("search task documents by text: %w", err)
	}
	return scanSearchTasks(rows)
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
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, query, repeatArgs(like, 42, limit)...)
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
	if assetSearchDocumentsTableExists(ctx, r.db.db) {
		items, err := r.searchAssetsFromDocuments(ctx, q, limit)
		if err == nil {
			return items, nil
		}
		if !isMySQLFullTextIndexMissing(err) {
			return nil, err
		}
	}
	return r.searchAssetsLegacy(ctx, q, limit)
}

func (r *searchRepo) SearchResourceGroups(ctx context.Context, q string, limit int, publishedOnly bool, access domain.ResourceGroupAccessFilter) ([]domain.SearchAsset, error) {
	if !r.tableExists(ctx, "task_asset_group_search_documents") {
		return []domain.SearchAsset{}, nil
	}
	limit = normalizeSearchLimit(limit)
	keyword := strings.TrimSpace(q)
	if keyword == "" {
		return []domain.SearchAsset{}, nil
	}
	textColumn := "d.internal_text"
	where := make([]string, 0, 3)
	if publishedOnly {
		textColumn = "d.final_text"
		where = append(where, `EXISTS (
			SELECT 1 FROM asset_workbench_client_materials cm
			WHERE cm.resource_group_id = g.id
			  AND cm.finalized_revision_id = g.finalized_revision_id
			  AND cm.enabled = 1
		)`)
	}
	matchClause := textColumn + " LIKE ?"
	whereArgs := []interface{}{"%" + keyword + "%"}
	orderClause := "g.updated_at DESC, g.id DESC"
	var orderArgs []interface{}
	if len([]rune(keyword)) >= 2 {
		matchClause = "MATCH(" + textColumn + ") AGAINST (? IN BOOLEAN MODE)"
		whereArgs[0] = booleanPhraseSearchQuery(keyword)
		orderClause = "MATCH(" + textColumn + ") AGAINST (? IN BOOLEAN MODE) DESC, g.updated_at DESC, g.id DESC"
		orderArgs = append(orderArgs, booleanPhraseSearchQuery(keyword))
	}
	where = append([]string{matchClause}, where...)
	if !publishedOnly {
		where, whereArgs = appendResourceGroupAccessScope(where, whereArgs, access)
	}
	args := append(append(whereArgs, orderArgs...), limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT g.id, g.task_id, t.task_no, COALESCE(tsi.sku_code, ''),
		       revision.id, revision.mode,
		       (SELECT COUNT(*) FROM task_asset_group_revision_items ri WHERE ri.revision_id = revision.id),
		       COALESCE((
		         SELECT ta.file_name
		         FROM task_asset_group_revision_items ri
		         JOIN task_assets ta ON ta.id = ri.task_asset_id
		         WHERE ri.revision_id = revision.id
		         ORDER BY ri.sort_order, ri.id LIMIT 1
		       ), '')
		FROM task_asset_group_search_documents d
		JOIN task_asset_groups g ON g.id = d.group_id AND g.finalized_revision_id = d.finalized_revision_id
		JOIN task_asset_group_revisions revision ON revision.id = g.finalized_revision_id
		JOIN tasks t ON t.id = g.task_id
		LEFT JOIN task_sku_items tsi ON tsi.id = g.task_sku_item_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY `+orderClause+` LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("search task resource groups: %w", err)
	}
	defer rows.Close()
	items := make([]domain.SearchAsset, 0, limit)
	for rows.Next() {
		var item domain.SearchAsset
		var taskID int64
		if err := rows.Scan(&item.ResourceGroupID, &taskID, &item.TaskNo, &item.SKUCode,
			&item.FinalizedRevisionID, &item.Mode, &item.FinalItemCount, &item.FileName); err != nil {
			return nil, fmt.Errorf("scan task resource group search result: %w", err)
		}
		item.AssetID = item.ResourceGroupID
		item.TaskID = &taskID
		item.ResourceID = fmt.Sprintf("group:%d", item.ResourceGroupID)
		item.SourceType = "task_resource_group"
		item.SourceLabel = "任务资源组"
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *searchRepo) searchAssetsFromDocuments(ctx context.Context, q string, limit int) ([]domain.SearchAsset, error) {
	limit = normalizeSearchLimit(limit)
	kw := normalizeSearchKeyword(q)
	items, err := r.searchAssetDocumentsByMode(ctx, kw, limit, true, nil, true)
	if err != nil {
		return nil, err
	}
	if len(items) >= limit || len(items) >= searchNaturalFallbackThreshold {
		return items, nil
	}
	fallback, err := r.searchAssetDocumentsByMode(ctx, kw, limit-len(items), false, searchAssetResultIDs(items), false)
	if err != nil {
		return nil, err
	}
	return appendSearchAssetsUnique(items, fallback, limit), nil
}

func (r *searchRepo) searchAssetDocumentsByMode(ctx context.Context, kw normalizedSearchKeyword, limit int, booleanMode bool, excludeAssetIDs []int64, includeNumeric bool) ([]domain.SearchAsset, error) {
	if limit <= 0 {
		return []domain.SearchAsset{}, nil
	}
	hasSemanticText := assetSearchDocumentsSemanticTextExists(ctx, r.db.db)
	branches := make([]string, 0, 8)
	args := make([]interface{}, 0, 16)
	excludePredicate := ""
	excludeArgs := make([]interface{}, 0, len(excludeAssetIDs))
	if len(excludeAssetIDs) > 0 {
		excludePredicate = " AND asset_id NOT IN (" + searchPlaceholders(len(excludeAssetIDs)) + ")"
		for _, id := range excludeAssetIDs {
			excludeArgs = append(excludeArgs, id)
		}
	}
	if includeNumeric && kw.HasInt64 {
		branches = append(branches,
			"SELECT asset_id, 0 AS match_rank, 1.0 AS match_score, sort_time FROM asset_search_documents WHERE asset_id = ?"+excludePredicate,
			"SELECT asset_id, 1 AS match_rank, 1.0 AS match_score, sort_time FROM asset_search_documents WHERE task_asset_id = ?"+excludePredicate,
			"SELECT asset_id, 2 AS match_rank, 1.0 AS match_score, sort_time FROM asset_search_documents WHERE task_id = ?"+excludePredicate,
		)
		for i := 0; i < 3; i++ {
			args = append(args, kw.Int64)
			args = append(args, excludeArgs...)
		}
	}
	if len([]rune(kw.Raw)) >= 2 {
		matchQuery := kw.Raw
		matchMode := "NATURAL LANGUAGE"
		searchRank := 20
		semanticRank := 40
		if booleanMode {
			matchQuery = booleanPhraseSearchQuery(kw.Raw)
			matchMode = "BOOLEAN"
			searchRank = 10
			semanticRank = 30
		}
		branches = append(branches, fmt.Sprintf(`SELECT asset_id, %d AS match_rank,
		               MATCH(search_text) AGAINST (? IN %s MODE) AS match_score,
		               sort_time
		          FROM asset_search_documents
		         WHERE MATCH(search_text) AGAINST (? IN %s MODE)%s`, searchRank, matchMode, matchMode, excludePredicate))
		args = append(args, matchQuery, matchQuery)
		args = append(args, excludeArgs...)
		if hasSemanticText {
			branches = append(branches, fmt.Sprintf(`SELECT asset_id, %d AS match_rank,
			               MATCH(semantic_text) AGAINST (? IN %s MODE) AS match_score,
			               sort_time
			          FROM asset_search_documents
			         WHERE MATCH(semantic_text) AGAINST (? IN %s MODE)%s`, semanticRank, matchMode, matchMode, excludePredicate))
			args = append(args, matchQuery, matchQuery)
			args = append(args, excludeArgs...)
		}
	}
	if len(branches) == 0 {
		return []domain.SearchAsset{}, nil
	}
	args = append(args, limit)
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, `
		SELECT d.asset_id, ta.file_name, ta.source_module_key, d.task_id,
		       d.asset_type, d.flow_review_status
		  FROM asset_search_documents d
		  JOIN task_assets ta ON ta.id = d.task_asset_id
		  JOIN (
		    SELECT asset_id, MIN(match_rank) AS match_rank, MAX(match_score) AS match_score,
		           MAX(sort_time) AS sort_time
		      FROM (
		        `+strings.Join(branches, "\n		        UNION ALL\n		        ")+`
		      ) u
		     GROUP BY asset_id
		     ORDER BY match_rank ASC, match_score DESC, sort_time DESC, asset_id DESC
		     LIMIT ?
		  ) m ON m.asset_id = d.asset_id
		 ORDER BY m.match_rank ASC, m.match_score DESC, d.sort_time DESC, d.asset_id DESC
		`, args...)
	if err != nil {
		return nil, fmt.Errorf("search asset documents: %w", err)
	}
	defer rows.Close()

	var out []domain.SearchAsset
	for rows.Next() {
		var item domain.SearchAsset
		var module sql.NullString
		var taskID sql.NullInt64
		var assetType, flowReviewStatus sql.NullString
		if err := rows.Scan(&item.AssetID, &item.FileName, &module, &taskID, &assetType, &flowReviewStatus); err != nil {
			return nil, fmt.Errorf("scan search asset document: %w", err)
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

func (r *searchRepo) searchAssetsLegacy(ctx context.Context, q string, limit int) ([]domain.SearchAsset, error) {
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
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, query, searchArgs...)
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
	if productSearchDocumentsTableExists(ctx, r.db.db) {
		items, err := r.searchProductsFromDocuments(ctx, q, limit)
		if err == nil {
			return items, nil
		}
		if !isMySQLFullTextIndexMissing(err) {
			return nil, err
		}
	}
	return r.searchProductsLegacy(ctx, q, limit)
}

func (r *searchRepo) searchProductsFromDocuments(ctx context.Context, q string, limit int) ([]domain.SearchProduct, error) {
	limit = normalizeSearchLimit(limit)
	kw := normalizeSearchKeyword(q)
	if kw.IsCode {
		return r.searchProductDocumentsByCode(ctx, kw, limit)
	}
	if len([]rune(kw.Raw)) >= 2 {
		return r.searchProductDocumentsByText(ctx, kw, limit)
	}
	return []domain.SearchProduct{}, nil
}

// searchProductDocumentsByCode recalls exact and prefix matches on the indexed
// sku_code / i_id columns via UNION ALL so each branch uses its own BTREE index
// instead of an OR table scan. Rows matched by multiple branches (e.g. exact +
// prefix) are de-duplicated by the outer GROUP BY, keeping the strongest match
// rank. FULLTEXT/semantic recall is intentionally excluded for code keywords to
// avoid full-table natural-language noise.
func (r *searchRepo) searchProductDocumentsByCode(ctx context.Context, kw normalizedSearchKeyword, limit int) ([]domain.SearchProduct, error) {
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, `
		SELECT erp_code, product_name, i_id, category
		  FROM (
		    SELECT sku_code AS erp_code, product_name, i_id, category,
		           MIN(match_rank) AS match_rank,
		           MAX(source_updated_at) AS source_updated_at
		      FROM (
		        SELECT sku_code, product_name, i_id, category, source_updated_at, 0 AS match_rank
		          FROM product_search_documents WHERE sku_code = ?
		        UNION ALL
		        SELECT sku_code, product_name, i_id, category, source_updated_at, 1 AS match_rank
		          FROM product_search_documents WHERE i_id = ?
		        UNION ALL
		        SELECT sku_code, product_name, i_id, category, source_updated_at, 2 AS match_rank
		          FROM product_search_documents WHERE sku_code LIKE ?
		        UNION ALL
		        SELECT sku_code, product_name, i_id, category, source_updated_at, 3 AS match_rank
		          FROM product_search_documents WHERE i_id LIKE ?
		      ) u
		     GROUP BY sku_code, product_name, i_id, category
		  ) g
		 ORDER BY match_rank ASC, source_updated_at DESC, erp_code DESC
		 LIMIT ?`,
		kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("search product documents by code: %w", err)
	}
	return scanSearchProducts(rows)
}

func (r *searchRepo) searchProductDocumentsByText(ctx context.Context, kw normalizedSearchKeyword, limit int) ([]domain.SearchProduct, error) {
	limit = normalizeSearchLimit(limit)
	terms := productSearchQueryNgrams(kw.Raw)
	if len(terms) > 0 && productSearchNgramIndexReady(ctx, r.db.db) {
		return r.searchProductDocumentsByNgrams(ctx, kw, terms, limit)
	}
	return r.searchProductDocumentsByTextFallback(ctx, kw, limit)
}

func (r *searchRepo) searchProductDocumentsByNgrams(ctx context.Context, kw normalizedSearchKeyword, terms []string, limit int) ([]domain.SearchProduct, error) {
	limit = normalizeSearchLimit(limit)
	if len(terms) == 0 {
		return []domain.SearchProduct{}, nil
	}
	likePattern := "%" + strings.Join(strings.Fields(kw.Raw), "%") + "%"
	hasSemanticText := productSearchDocumentsSemanticTextExists(ctx, r.db.db)
	where := "search_text LIKE ?"
	order := "CASE WHEN product_name LIKE ? THEN 0 ELSE 1 END"
	args := make([]interface{}, 0, len(terms)+6)
	for _, term := range terms {
		args = append(args, term)
	}
	candidateLimit := limit * 50
	if candidateLimit < 1000 {
		candidateLimit = 1000
	}
	if candidateLimit > 5000 {
		candidateLimit = 5000
	}
	args = append(args, len(terms), candidateLimit, likePattern, likePattern)
	if hasSemanticText {
		where = "(search_text LIKE ? OR semantic_text LIKE ?)"
		order = "CASE WHEN product_name LIKE ? THEN 0 WHEN search_text LIKE ? THEN 1 ELSE 2 END"
		args = append(args[:len(terms)+2], likePattern, likePattern, likePattern, likePattern)
	}
	args = append(args, limit)
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, `
		SELECT d.sku_code AS erp_code, d.product_name, d.i_id, d.category
		  FROM product_search_documents d
		  JOIN (
		    SELECT n.sku_code
		      FROM product_search_ngrams n
		      JOIN product_search_documents ranked ON ranked.sku_code = n.sku_code
		     WHERE n.term IN (`+searchPlaceholders(len(terms))+`)
		     GROUP BY n.sku_code
		    HAVING COUNT(DISTINCT n.term) = ?
		     ORDER BY MAX(ranked.source_updated_at) DESC, n.sku_code DESC
		     LIMIT ?
		  ) hits ON hits.sku_code = d.sku_code
		 WHERE `+strings.ReplaceAll(strings.ReplaceAll(where, "search_text", "d.search_text"), "semantic_text", "d.semantic_text")+`
		 ORDER BY `+strings.ReplaceAll(strings.ReplaceAll(order, "product_name", "d.product_name"), "search_text", "d.search_text")+`, d.source_updated_at DESC, d.sku_code DESC
		 LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("search product documents by ngram: %w", err)
	}
	return scanSearchProducts(rows)
}

func (r *searchRepo) searchProductDocumentsByTextFallback(ctx context.Context, kw normalizedSearchKeyword, limit int) ([]domain.SearchProduct, error) {
	limit = normalizeSearchLimit(limit)
	likePattern := "%" + strings.Join(strings.Fields(kw.Raw), "%") + "%"
	hasSemanticText := productSearchDocumentsSemanticTextExists(ctx, r.db.db)
	where := "search_text LIKE ?"
	order := "CASE WHEN product_name LIKE ? THEN 0 ELSE 1 END"
	args := []interface{}{likePattern, likePattern}
	if hasSemanticText {
		where = "(search_text LIKE ? OR semantic_text LIKE ?)"
		order = "CASE WHEN product_name LIKE ? THEN 0 WHEN search_text LIKE ? THEN 1 ELSE 2 END"
		args = []interface{}{likePattern, likePattern, likePattern, likePattern}
	}
	args = append(args, limit)
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, `
		SELECT sku_code AS erp_code, product_name, i_id, category
		  FROM product_search_documents
		 WHERE `+where+`
		 ORDER BY `+order+`, source_updated_at DESC, sku_code DESC
		 LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("search product documents by text fallback: %w", err)
	}
	return scanSearchProducts(rows)
}

func (r *searchRepo) searchProductsLegacy(ctx context.Context, q string, limit int) ([]domain.SearchProduct, error) {
	limit = normalizeSearchLimit(limit)
	kw := normalizeSearchKeyword(q)
	if r.tableExists(ctx, "products") {
		iidExpr := productIIDSearchExpr(ctx, r.db.db, "")
		categoryNameExpr := productCategoryNameSearchExpr("")
		query := fmt.Sprintf(`
			SELECT sku_code AS erp_code,
			       product_name,
			       COALESCE(NULLIF(%[1]s, ''), '') AS i_id,
			       COALESCE(NULLIF(%[2]s, ''), NULLIF(category, '')) AS category
			  FROM products
			 WHERE sku_code = ?
			    OR %[1]s = ?
			    OR sku_code LIKE ?
			    OR %[1]s LIKE ?
			    OR sku_code LIKE ?
			    OR product_name LIKE ?
			    OR category LIKE ?
			    OR %[1]s LIKE ?
			    OR %[2]s LIKE ?
			 ORDER BY CASE
			            WHEN sku_code = ? THEN 0
			            WHEN %[1]s = ? THEN 1
			            WHEN sku_code LIKE ? THEN 2
			            ELSE 3
			          END,
			          id DESC
			 LIMIT ?`, iidExpr, categoryNameExpr)
		qctx, cancel := mysqlReadQueryContext(ctx)
		rows, err := r.db.db.QueryContext(qctx, query,
			kw.Upper, kw.Upper, kw.Upper+"%", kw.Upper+"%", kw.Like, kw.Like, kw.Like, kw.Like, kw.Like,
			kw.Upper, kw.Upper, kw.Upper+"%", limit)
		if err == nil {
			items, scanErr := scanSearchProducts(rows)
			cancel()
			return items, scanErr
		}
		cancel()
	}
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, `
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
	qctx, cancel := mysqlReadQueryContext(ctx)
	defer cancel()
	rows, err := r.db.db.QueryContext(qctx, `
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
	return mysqlTableExists(ctx, r.db.db, table)
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

func searchPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func searchTaskResultIDs(items []domain.SearchTask) []int64 {
	out := make([]int64, 0, len(items))
	for _, item := range items {
		if item.ID > 0 {
			out = append(out, item.ID)
		}
	}
	return out
}

func searchAssetResultIDs(items []domain.SearchAsset) []int64 {
	out := make([]int64, 0, len(items))
	for _, item := range items {
		if item.AssetID > 0 {
			out = append(out, item.AssetID)
		}
	}
	return out
}

func searchProductResultCodes(items []domain.SearchProduct) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ERPCode) != "" {
			out = append(out, item.ERPCode)
		}
	}
	return out
}

func appendSearchTasksUnique(dst []domain.SearchTask, src []domain.SearchTask, limit int) []domain.SearchTask {
	seen := make(map[int64]struct{}, len(dst)+len(src))
	for _, item := range dst {
		seen[item.ID] = struct{}{}
	}
	for _, item := range src {
		if len(dst) >= limit {
			break
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		dst = append(dst, item)
	}
	return dst
}

func appendSearchAssetsUnique(dst []domain.SearchAsset, src []domain.SearchAsset, limit int) []domain.SearchAsset {
	seen := make(map[int64]struct{}, len(dst)+len(src))
	for _, item := range dst {
		seen[item.AssetID] = struct{}{}
	}
	for _, item := range src {
		if len(dst) >= limit {
			break
		}
		if _, ok := seen[item.AssetID]; ok {
			continue
		}
		seen[item.AssetID] = struct{}{}
		dst = append(dst, item)
	}
	return dst
}

func appendSearchProductsUnique(dst []domain.SearchProduct, src []domain.SearchProduct, limit int) []domain.SearchProduct {
	seen := make(map[string]struct{}, len(dst)+len(src))
	for _, item := range dst {
		seen[item.ERPCode] = struct{}{}
	}
	for _, item := range src {
		if len(dst) >= limit {
			break
		}
		if _, ok := seen[item.ERPCode]; ok {
			continue
		}
		seen[item.ERPCode] = struct{}{}
		dst = append(dst, item)
	}
	return dst
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
